/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package nmble

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type discovererState int

const (
	DISCOVERER_STATE_IDLE discovererState = iota
	DISCOVERER_STATE_STARTED
	DISCOVERER_STATE_STOPPING
	DISCOVERER_STATE_STOPPED
)

type DiscovererParams struct {
	Bx          *BleXport
	OwnAddrType BleAddrType
	Passive     bool
	Duration    time.Duration
}

// Listens for advertisements; reports the ones that match the specified
// predicate.  This type is not thread-safe.
type Discoverer struct {
	params   DiscovererParams
	bl       *Listener
	state    discovererState
	stopChan chan struct{}
	wg       sync.WaitGroup
	mtx      sync.Mutex
}

func NewDiscoverer(params DiscovererParams) *Discoverer {
	return &Discoverer{
		params: params,
	}
}

func (d *Discoverer) setState(state discovererState) {
	d.state = state
}

func (d *Discoverer) scanCancel() error {
	r := NewBleScanCancelReq()

	bl, err := d.params.Bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer d.params.Bx.RemoveListener(bl)

	if err := scanCancel(d.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (d *Discoverer) Start() (<-chan BleAdvReport, <-chan error, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.state != DISCOVERER_STATE_IDLE {
		return nil, nil, nmxutil.NewAlreadyError(
			"Attempt to start BLE discoverer twice")
	}

	d.stopChan = make(chan struct{})

	r := NewBleScanReq()
	r.OwnAddrType = d.params.OwnAddrType
	r.DurationMs = int(d.params.Duration / time.Millisecond)
	r.FilterPolicy = BLE_SCAN_FILT_NO_WL
	r.Limited = false
	r.Passive = d.params.Passive
	r.FilterDuplicates = true

	bl, err := d.params.Bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return nil, nil, err
	}

	ach, ech, err := actScan(d.params.Bx, bl, r)
	if err != nil {
		d.params.Bx.RemoveListener(bl)
		return nil, nil, err
	}

	// A buffer size of 1 is used so that the receiving Goroutine can stop the
	// discoverer without triggering a deadlock.
	parentEch := make(chan error, 1)

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		stopChan := d.stopChan

		// Block until scan completes.
		var done bool
		var err error
		for !done {
			select {
			case err = <-ech:
				done = true

			case <-stopChan:
				// Discovery aborted; remove the BLE listener.  This will cause
				// the scan to fail, triggering a send on the ech channel.
				if bl != nil {
					d.params.Bx.RemoveListener(bl)
					bl = nil
					stopChan = nil
				}
			}
		}

		d.mtx.Lock()
		defer d.mtx.Unlock()

		// Always attempt to cancel scanning.  No harm done if scanning is
		// already stopped.
		if !nmxutil.IsXport(err) {
			if err := d.scanCancel(); err != nil {
				log.Debugf("Failed to cancel scan in progress: %s",
					err.Error())
			}
		}

		if bl != nil {
			d.params.Bx.RemoveListener(bl)
		}

		d.setState(DISCOVERER_STATE_IDLE)

		parentEch <- err
		close(parentEch)
	}()

	d.setState(DISCOVERER_STATE_STARTED)

	return ach, parentEch, nil
}

// Ensures the discoverer is stopped.  Errors can typically be ignored.
func (d *Discoverer) Stop() error {
	initiate := func() error {
		d.mtx.Lock()
		defer d.mtx.Unlock()

		if d.state != DISCOVERER_STATE_STARTED {
			return nmxutil.NewAlreadyError(
				"Attempt to stop inactive discoverer")
		}

		d.setState(DISCOVERER_STATE_STOPPING)
		close(d.stopChan)
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}

	// Don't return until discovery is fully stopped.
	d.wg.Wait()

	d.mtx.Lock()
	d.setState(DISCOVERER_STATE_IDLE)
	d.mtx.Unlock()

	return nil
}

// Discovers a single device.  After a device is successfully discovered,
// discovery is stopped.
func DiscoverDevice(
	bx *BleXport,
	ownAddrType BleAddrType,
	duration time.Duration,
	advPred BleAdvPredicate) (*BleDev, error) {

	d := NewDiscoverer(DiscovererParams{
		Bx:          bx,
		OwnAddrType: ownAddrType,
		Passive:     false,
		Duration:    duration,
	})

	ach, ech, err := d.Start()
	if err != nil {
		if nmxutil.IsScanTmo(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	for {
		select {
		case adv, ok := <-ach:
			if ok && advPred(adv) {
				d.Stop()
				return &adv.Sender, nil
			}

		case err := <-ech:
			return nil, err
		}
	}
}
