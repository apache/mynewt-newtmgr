// +build !windows

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

package bll

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"github.com/runtimeco/go-coap"
	"golang.org/x/net/context"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllSesn struct {
	cfg BllSesnCfg

	cln    ble.Client
	txvr   *mgmt.Transceiver
	mtx    sync.Mutex
	attMtu uint16

	nmpReqChr    *ble.Characteristic
	nmpRspChr    *ble.Characteristic
	publicReqChr *ble.Characteristic
	publicRspChr *ble.Characteristic
	unauthReqChr *ble.Characteristic
	unauthRspChr *ble.Characteristic
	secureReqChr *ble.Characteristic
	secureRspChr *ble.Characteristic
}

func NewBllSesn(cfg BllSesnCfg) *BllSesn {
	return &BllSesn{
		cfg: cfg,
	}
}

func (s *BllSesn) listenDisconnect() {
	go func() {
		<-s.cln.Disconnected()

		s.mtx.Lock()
		s.txvr.ErrorAll(fmt.Errorf("disconnected"))
		s.mtx.Unlock()

		s.cln = nil
	}()
}

func (s *BllSesn) connect() error {
	log.Debugf("Connecting to peer")
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		s.cfg.ConnTimeout))

	var err error
	s.cln, err = ble.Connect(ctx, s.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return fmt.Errorf("Failed to connect to peer after %s",
				s.cfg.ConnTimeout.String())
		} else {
			return err
		}
	}

	s.listenDisconnect()

	return nil
}

func findChr(profile *ble.Profile, svcUuid bledefs.BleUuid,
	chrUuid bledefs.BleUuid) (*ble.Characteristic, error) {

	for _, s := range profile.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return nil, err
		}

		if bledefs.CompareUuids(uuid, svcUuid) == 0 {
			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return nil, err
				}

				if bledefs.CompareUuids(uuid, chrUuid) == 0 {
					return c, nil
				}
			}
		}
	}

	return nil, nil
}

func (s *BllSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := s.cln.DiscoverProfile(true)
	if err != nil {
		return err
	}

	nmpSvcUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainSvcUuid)
	nmpChrUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainChrUuid)

	ompSvcUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecSvcUuid)
	ompReqChrUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecReqChrUuid)
	ompRspChrUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecRspChrUuid)

	unauthSvcUuid, _ := bledefs.ParseUuid(bledefs.UnauthSvcUuid)
	unauthReqChrUuid, _ := bledefs.ParseUuid(bledefs.UnauthReqChrUuid)
	unauthRspChrUuid, _ := bledefs.ParseUuid(bledefs.UnauthRspChrUuid)

	switch s.cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		s.nmpReqChr, err = findChr(p, nmpSvcUuid, nmpChrUuid)
		if err != nil {
			return err
		}
		s.nmpRspChr = s.nmpReqChr

	case sesn.MGMT_PROTO_OMP:
		s.nmpReqChr, err = findChr(p, ompSvcUuid, ompReqChrUuid)
		if err != nil {
			return err
		}

		s.nmpRspChr, err = findChr(p, ompSvcUuid, ompRspChrUuid)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid management protocol: %s", s.cfg.MgmtProto)
	}

	s.unauthReqChr, err = findChr(p, unauthSvcUuid, unauthReqChrUuid)
	if err != nil {
		return err
	}

	s.unauthRspChr, err = findChr(p, unauthSvcUuid, unauthRspChrUuid)
	if err != nil {
		return err
	}

	return nil
}

// Subscribes to the peer's characteristic implementing NMP.
func (s *BllSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		s.txvr.DispatchNmpRsp(data)
	}

	if s.nmpRspChr != nil {
		if err := s.cln.Subscribe(s.nmpRspChr, false,
			onNotify); err != nil {

			return err
		}
	}

	if s.unauthRspChr != nil {
		if err := s.cln.Subscribe(s.unauthRspChr, false,
			onNotify); err != nil {

			return err
		}
	}

	return nil
}

func (s *BllSesn) exchangeMtu() error {
	mtu, err := exchangeMtu(s.cln, s.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	s.attMtu = mtu
	return nil
}

// @return bool                 Whether to retry the open attempt; false
//                                  on success.
//         error                The cause of a failed open; nil on success.
func (s *BllSesn) openOnce() (bool, error) {
	if s.IsOpen() {
		return false, nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

	txvr, err := mgmt.NewTransceiver(s.cfg.MgmtProto, 3)
	if err != nil {
		return false, err
	}
	s.txvr = txvr

	if err := s.connect(); err != nil {
		return false, err
	}

	if err := s.exchangeMtu(); err != nil {
		return true, err
	}

	if err := s.discoverAll(); err != nil {
		return false, err
	}

	if err := s.subscribe(); err != nil {
		return false, err
	}

	return false, nil
}

func (s *BllSesn) Open() error {
	var err error

	for i := 0; i < s.cfg.ConnTries; i++ {
		var retry bool

		retry, err = s.openOnce()
		if !retry {
			break
		}
	}

	if err != nil {
		// Ensure the session is closed.
		s.Close()
		return err
	}

	return nil
}

func (s *BllSesn) Close() error {
	if !s.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := s.cln.CancelConnection(); err != nil {
		return err
	}

	s.cln = nil

	return nil
}

// Indicates whether the session is currently open.
func (s *BllSesn) IsOpen() bool {
	return s.cln != nil
}

// Retrieves the maximum data payload for incoming NMP responses.
func (s *BllSesn) MtuIn() int {
	mtu, _ := nmble.MtuIn(s.cfg.MgmtProto, s.attMtu)
	return mtu
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (s *BllSesn) MtuOut() int {
	mtu, _ := nmble.MtuOut(s.cfg.MgmtProto, s.attMtu)
	return mtu
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (s *BllSesn) AbortRx(nmpSeq uint8) error {
	s.txvr.ErrorOne(nmpSeq, fmt.Errorf("Rx aborted"))
	return nil
}

func (s *BllSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return nmble.EncodeMgmtMsg(s.cfg.MgmtProto, msg)
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (s *BllSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !s.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	if s.nmpReqChr == nil || s.nmpRspChr == nil {
		return nil, fmt.Errorf("Cannot send NMP request; peer doesn't " +
			"support request or response characteristic")
	}

	txRaw := func(b []byte) error {
		return s.cln.WriteCharacteristic(s.nmpReqChr, b, true)
	}

	return s.txvr.TxNmp(txRaw, msg, opt.Timeout)
}

func (s *BllSesn) resReqChr(resType sesn.ResourceType) (
	*ble.Characteristic, error) {

	m := map[sesn.ResourceType]*ble.Characteristic{
		sesn.RES_TYPE_PUBLIC: s.publicReqChr,
		sesn.RES_TYPE_UNAUTH: s.unauthReqChr,
		sesn.RES_TYPE_SECURE: s.secureReqChr,
	}

	chr := m[resType]
	if chr == nil {
		return nil, fmt.Errorf("BLE session not configured with "+
			"characteristic for %s resources", resType)
	}

	return chr, nil
}

func (s *BllSesn) GetResourceOnce(resType sesn.ResourceType, uri string,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	chr, err := s.resReqChr(resType)
	if err != nil {
		return 0, nil, err
	}

	token := nmxutil.NextToken()
	req, err := oic.CreateGet(true, uri, token)
	if err != nil {
		return 0, nil, err
	}

	txRaw := func(b []byte) error {
		return s.cln.WriteCharacteristic(chr, b, true)
	}

	rsp, err := s.txvr.TxOic(txRaw, req, opt.Timeout)
	if err != nil {
		return 0, nil, err
	} else if rsp == nil {
		return 0, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), nil
	}
}

func (s *BllSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte, opt sesn.TxOptions) (coap.COAPCode, error) {

	chr, err := s.resReqChr(resType)
	if err != nil {
		return 0, err
	}

	token := nmxutil.NextToken()
	req, err := oic.CreatePut(true, uri, token, value)
	if err != nil {
		return 0, err
	}

	txRaw := func(b []byte) error {
		return s.cln.WriteCharacteristic(chr, b, true)
	}

	rsp, err := s.txvr.TxOic(txRaw, req, opt.Timeout)
	if err != nil {
		return 0, err
	} else if rsp == nil {
		return 0, nil
	} else {
		return rsp.Code(), nil
	}
}
