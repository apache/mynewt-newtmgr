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
	"github.com/runtimeco/go-coap"

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
	scanBlocker    nmxutil.Blocker
	suspendBlocker nmxutil.Blocker

	mtx sync.Mutex

	// Protected by the mutex.
	discoverer   *Discoverer
	reportedDevs map[BleDev]string
	failedDevs   map[BleDev]struct{}
	ses          *BleSesn
	enabled      bool
}

func NewBleScanner(bx *BleXport) *BleScanner {
	return &BleScanner{
		bx:           bx,
		reportedDevs: map[BleDev]string{},
		failedDevs:   map[BleDev]struct{}{},
	}
}

func (s *BleScanner) isEnabled() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.enabled
}

// Performs a compare-and-swap of the enabled state.
func (s *BleScanner) toggleEnabled(to bool) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.enabled == to {
		return false
	}

	s.enabled = to
	return true
}

func (s *BleScanner) setSession(ses *BleSesn) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	nmxutil.Assert(s.ses == nil || ses == nil)
	s.ses = ses
}

func (s *BleScanner) addReportedDev(dev BleDev, hwid string) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.reportedDevs[dev] = hwid
}

func (s *BleScanner) addFailedDev(dev BleDev) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.failedDevs[dev] = struct{}{}
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
			defer s.mtx.Unlock()

			dev = &r.Sender
			s.discoverer.Stop()
		}
	}
	if err := s.discoverer.Start(advRptCb); err != nil {
		return nil, err
	}

	return dev, nil
}

func (s *BleScanner) connect(dev BleDev) error {
	s.cfg.SesnCfg.PeerSpec.Ble = dev
	ses, err := NewBleSesn(s.bx, s.cfg.SesnCfg, MASTER_PRIO_SCAN)
	if err != nil {
		return err
	}

	s.setSession(ses)
	if err := s.ses.Open(); err != nil {
		return err
	}

	return nil
}

func (s *BleScanner) readHwId() (string, error) {
	c := xact.NewGetResCmd()
	c.Path = "dev"
	c.Typ = sesn.RES_TYPE_PUBLIC

	res, err := c.Run(s.ses)
	if err != nil {
		return "", fmt.Errorf("failed to read hardware ID; %s", err.Error())
	}
	cres := res.(*xact.GetResResult)
	if cres.Code != coap.Content {
		return "",
			fmt.Errorf("failed to read hardware ID; CoAP status=%s",
				cres.Code.String())
	}

	m, err := nmxutil.DecodeCborMap(cres.Value)
	if err != nil {
		return "", fmt.Errorf("failed to read hardware ID; %s", err.Error())
	}

	itf := m["hwid"]
	if itf == nil {
		return "", fmt.Errorf("failed to read hardware ID; \"hwid\" " +
			"item missing from dev resource")
	}

	str, ok := itf.(string)
	if !ok {
		return "", fmt.Errorf("failed to read hardware ID; invalid \"hwid\" "+
			"item: %#v", itf)
	}

	return str, nil
}

func (s *BleScanner) scan() (*scan.ScanPeer, error) {
	// Ensure subsequent calls to suspend() block until scanning has stopped.
	s.scanBlocker.Start()
	defer s.scanBlocker.Unblock(nil)

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
	defer func() {
		s.ses.Close()
		s.setSession(nil)
	}()

	// Now that we have successfully connected to this device, we will report
	// it regardless of success or failure.
	peer := scan.ScanPeer{
		HwId: "",
		PeerSpec: sesn.PeerSpec{
			Ble: *dev,
		},
	}

	hwId, err := s.readHwId()
	if err != nil {
		return &peer, err
	}

	peer.HwId = hwId
	return &peer, nil
}

// Caller must lock mutex.
func (s *BleScanner) seenDevice(dev BleDev) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if _, ok := s.reportedDevs[dev]; ok {
		return true
	}

	if _, ok := s.failedDevs[dev]; ok {
		return true
	}

	return false
}

func (s *BleScanner) Start(cfg scan.Cfg) error {
	if !s.toggleEnabled(true) {
		return nmxutil.NewAlreadyError("Attempt to start BLE scanner twice")
	}

	// Wrap predicate with logic that discards duplicates.
	innerPred := cfg.Ble.ScanPred
	cfg.Ble.ScanPred = func(adv BleAdvReport) bool {
		// Filter devices that have already been reported.
		if s.seenDevice(adv.Sender) {
			return false
		} else {
			return innerPred(adv)
		}
	}
	s.cfg = cfg

	// Start background scanning.
	go func() {
		for {
			// Wait for suspend-in-progress to complete, if any.
			s.suspendBlocker.Wait(nmxutil.DURATION_FOREVER, nil)

			if !s.isEnabled() {
				break
			}

			p, err := s.scan()
			if err != nil {
				log.Debugf("Scan error: %s", err.Error())
				if p != nil {
					s.addFailedDev(p.PeerSpec.Ble)
				}
				time.Sleep(scanRetryRate)
			} else if p != nil {
				s.addReportedDev(p.PeerSpec.Ble, p.HwId)
				s.cfg.ScanCb(*p)
			}
		}
	}()

	return nil
}

func (s *BleScanner) suspend() error {
	s.suspendBlocker.Start()
	defer s.suspendBlocker.Unblock(nil)

	discoverer := s.discoverer
	ses := s.ses

	if discoverer != nil {
		discoverer.Stop()
	}

	if ses != nil {
		ses.Close()
	}

	// Block until scan is fully terminated.
	s.scanBlocker.Wait(nmxutil.DURATION_FOREVER, nil)

	s.discoverer = nil
	s.ses = nil

	return nil
}

// Aborts the current scan but leaves the scanner enabled.  This function is
// called when a higher priority procedure (e.g., connecting) needs to acquire
// master privileges.  When the high priority procedures are complete, scanning
// will resume.
func (s *BleScanner) Preempt() error {
	return s.suspend()
}

// Stops the scanner.  Scanning won't resume unless Start() gets called.
func (s *BleScanner) Stop() error {
	if !s.toggleEnabled(false) {
		return nmxutil.NewAlreadyError("Attempt to stop BLE scanner twice")
	}

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
	for discoverer, _ := range s.failedDevs {
		delete(s.failedDevs, discoverer)
	}
}
