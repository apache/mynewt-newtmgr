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
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

const scanRetryRate = time.Second

// Implements scan.Scanner.
type BleScanner struct {
	cfg scan.Cfg

	bx             *BleXport
	discoverer     *Discoverer
	reportedDevs   map[BleDev]string
	bos            *BleSesn
	enabled        bool
	suspendBlocker nmxutil.Blocker

	// Protects accesses to the reported devices map.
	mtx sync.Mutex
}

func NewBleScanner(bx *BleXport) *BleScanner {
	return &BleScanner{
		bx:           bx,
		reportedDevs: map[BleDev]string{},
	}
}

func (s *BleScanner) discover() (*BleDev, error) {
	s.mtx.Lock()
	s.discoverer = NewDiscoverer(DiscovererParams{
		Bx:          s.bx,
		OwnAddrType: s.cfg.SesnCfg.Ble.OwnAddrType,
		Passive:     false,
		Duration:    15 * time.Second,
	})
	s.mtx.Unlock()

	defer func() { s.discoverer = nil }()

	var dev *BleDev
	advRptCb := func(r BleAdvReport) {
		if s.cfg.Ble.ScanPred(r) {
			s.mtx.Lock()

			dev = &r.Sender
			s.discoverer.Stop()

			s.mtx.Unlock()
		}
	}
	if err := s.discoverer.Start(advRptCb); err != nil {
		return nil, err
	}

	return dev, nil
}

func (s *BleScanner) connect(dev BleDev) error {
	s.cfg.SesnCfg.PeerSpec.Ble = dev
	bs, err := NewBleSesn(s.bx, s.cfg.SesnCfg, MASTER_PRIO_SCAN)
	if err != nil {
		return err
	}

	s.mtx.Lock()
	s.bos = bs
	s.mtx.Unlock()

	if err := s.bos.Open(); err != nil {
		return err
	}

	return nil
}

func (s *BleScanner) readHwId() (string, error) {
	c := xact.NewConfigReadCmd()
	c.Name = "id/hwid"

	res, err := c.Run(s.bos)
	if err != nil {
		return "", err
	}
	if res.Status() != 0 {
		return "",
			fmt.Errorf("failed to read hardware ID; NMP status=%d",
				res.Status())
	}
	cres := res.(*xact.ConfigReadResult)
	return cres.Rsp.Val, nil
}

func (s *BleScanner) scan() (*scan.ScanPeer, error) {
	// Ensure subsequent calls to suspend() block.
	s.suspendBlocker.Block()

	// If the scanner is being suspended, unblock the suspend() call.
	defer s.suspendBlocker.Unblock(nil)

	// Discover the first device which matches the specified predicate.
	dev, err := s.discover()
	if err != nil {
		return nil, err
	}
	if dev == nil {
		return nil, nil
	}

	if err := s.connect(*dev); err != nil {
		return nil, err
	}
	defer s.bos.Close()

	// Now we are connected (and paired if required).  Read the peer's hardware
	// ID and report it upstream.
	hwId, err := s.readHwId()
	if err != nil {
		return nil, err
	}

	desc, err := s.bos.ConnInfo()
	if err != nil {
		return nil, err
	}

	peer := scan.ScanPeer{
		HwId: hwId,
		PeerSpec: sesn.PeerSpec{
			Ble: BleDev{
				AddrType: desc.PeerIdAddrType,
				Addr:     desc.PeerIdAddr,
			},
		},
	}

	return &peer, nil
}

func (s *BleScanner) Start(cfg scan.Cfg) error {
	if s.enabled {
		return nmxutil.NewAlreadyError("Attempt to start BLE scanner twice")
	}

	// Wrap predicate with logic that discards duplicates.
	innerPred := cfg.Ble.ScanPred
	cfg.Ble.ScanPred = func(adv BleAdvReport) bool {
		// Filter devices that have already been reported.
		s.mtx.Lock()
		seen := s.reportedDevs[adv.Sender] != ""
		s.mtx.Unlock()

		if seen {
			return false
		} else {
			return innerPred(adv)
		}
	}

	s.enabled = true
	s.cfg = cfg

	// Start background scanning.
	go func() {
		for s.enabled {
			p, err := s.scan()
			if err != nil {
				log.Debugf("Scan error: %s", err.Error())
				time.Sleep(scanRetryRate)
			} else if p != nil {
				s.mtx.Lock()
				s.reportedDevs[p.PeerSpec.Ble] = p.HwId
				s.mtx.Unlock()

				s.cfg.ScanCb(*p)
			}
		}
	}()

	return nil
}

func (s *BleScanner) suspend() error {
	discoverer := s.discoverer
	bos := s.bos

	if discoverer != nil {
		discoverer.Stop()
	}

	if bos != nil {
		bos.Close()
	}

	// Block until scan is fully terminated.
	s.suspendBlocker.Wait(nmxutil.DURATION_FOREVER)

	s.discoverer = nil
	s.bos = nil

	return nil
}

// Aborts the current scan but leaves the scanner enabled.  This function is
// called when a higher priority procedure (e.g., connecting) needs to acquire
// master privileges.  When the high priority procedures are complete, scanning
// will resume.
func (s *BleScanner) Preempt() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.suspend()
}

// Stops the scanner.  Scanning won't resume unless Start() gets called.
func (s *BleScanner) Stop() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if !s.enabled {
		return nmxutil.NewAlreadyError("Attempt to stop BLE scanner twice")
	}
	s.enabled = false

	return s.suspend()
}

// @return                      true if the specified device was found and
//                                  forgetten;
//                              false if the specified device is unknown.
func (s *BleScanner) ForgetDevice(hwId string) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for discoverer, h := range s.reportedDevs {
		if h == hwId {
			delete(s.reportedDevs, discoverer)
			return true
		}
	}

	return false
}

func (s *BleScanner) ForgetAllDevices() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for discoverer, _ := range s.reportedDevs {
		delete(s.reportedDevs, discoverer)
	}
}
