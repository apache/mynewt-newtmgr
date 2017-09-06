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

type DiscovererParams struct {
	Bx          *BleXport
	OwnAddrType BleAddrType
	Passive     bool
	Duration    time.Duration
}

// Listens for advertisements; reports the ones that match the specified
// predicate.  This type is not thread-safe.
type Discoverer struct {
	params    DiscovererParams
	abortChan chan struct{}
	blocker   nmxutil.Blocker
	mtx       sync.Mutex
}

func NewDiscoverer(params DiscovererParams) *Discoverer {
	return &Discoverer{
		params: params,
	}
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

func (d *Discoverer) Start(advRptCb BleAdvRptFn) error {
	// Sets up the abort channel to allow discovery to be cancelled.  Ensures
	// only one discovery procedure is active at a time.
	initiate := func() error {
		d.mtx.Lock()
		defer d.mtx.Unlock()

		if d.abortChan != nil {
			return nmxutil.NewAlreadyError(
				"Attempt to start BLE discoverer twice")
		}

		d.abortChan = make(chan struct{})

		return nil
	}

	finalize := func() {
		d.mtx.Lock()
		defer d.mtx.Unlock()

		d.abortChan = nil
	}

	if err := initiate(); err != nil {
		return err
	}
	defer finalize()

	// Scanning requires dedicated master privileges.
	if err := AcquireMaster(d.params.Bx, MASTER_PRIO_SCAN, d); err != nil {
		return err
	}
	defer d.params.Bx.ReleaseMaster()

	r := NewBleScanReq()
	r.OwnAddrType = d.params.OwnAddrType
	r.DurationMs = int(d.params.Duration / time.Millisecond)
	r.FilterPolicy = BLE_SCAN_FILT_NO_WL
	r.Limited = false
	r.Passive = d.params.Passive
	r.FilterDuplicates = true

	bl, err := d.params.Bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer d.params.Bx.RemoveListener(bl)

	// Ensure subsequent calls to Stop() block until discovery is fully
	// stopped.
	d.blocker.Start()
	defer d.blocker.Unblock(nil)

	// Report devices in a separate Goroutine.  This is done to prevent
	// deadlock in case the callback tries to stop the discoverer.
	cb := func(r BleAdvReport) { go advRptCb(r) }

	err = actScan(d.params.Bx, bl, r, d.abortChan, cb)
	if !nmxutil.IsXport(err) {
		// The transport did not restart; always attempt to cancel the scan
		// operation.  In some cases, the host has already stopped scanning
		// and will respond with an "ealready" error that can be ignored.
		if err := d.scanCancel(); err != nil {
			log.Errorf("Failed to cancel scan in progress: %s",
				err.Error())
		}
	}

	return err
}

// Ensures the discoverer is stopped.  Errors can typically be ignored.
func (d *Discoverer) Stop() error {
	initiate := func() error {
		d.mtx.Lock()
		defer d.mtx.Unlock()

		if d.abortChan == nil {
			return nmxutil.NewAlreadyError(
				"Attempt to stop inactive discoverer")
		}

		close(d.abortChan)
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}

	// Don't return until discovery is fully stopped.
	d.blocker.Wait(nmxutil.DURATION_FOREVER, nil)

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

	var dev *BleDev
	advRptCb := func(adv BleAdvReport) {
		if advPred(adv) {
			dev = &adv.Sender
			d.Stop()
		}
	}

	if err := d.Start(advRptCb); err != nil {
		if !nmxutil.IsScanTmo(err) {
			return nil, err
		}
	}

	return dev, nil
}
