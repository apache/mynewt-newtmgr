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
	"github.com/go-ble/ble"
	"github.com/runtimeco/go-coap"
	"golang.org/x/net/context"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

// A session that uses the host machine's native BLE support.
type BllSesn struct {
	cfg BllSesnCfg

	// The native BLE client.  All accesses must be protected by the mutex.
	cln ble.Client

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

func (s *BllSesn) getCln() (ble.Client, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.cln == nil {
		return nil, fmt.Errorf("disconnected")
	}

	return s.cln, nil
}

func (s *BllSesn) setCln(c ble.Client) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.cln = c
}

func (s *BllSesn) listenDisconnect() {
	go func() {
		cln, err := s.getCln()
		if err != nil {
			return
		}

		<-cln.Disconnected()
		s.txvr.ErrorAll(fmt.Errorf("disconnected"))
		s.txvr.Stop()
		s.setCln(nil)
	}()
}

func (s *BllSesn) txConnect(f ble.AdvFilter) (ble.Client, error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		s.cfg.ConnTimeout))

	client, err := ble.Connect(ctx, s.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("Failed to connect to peer after %s",
				s.cfg.ConnTimeout.String())
		} else {
			return nil, err
		}
	}

	return client, nil
}

func (s *BllSesn) txDiscoverProfile() (*ble.Profile, error) {
	cln, err := s.getCln()
	if err != nil {
		return nil, err
	}

	return cln.DiscoverProfile(true)
}

func (s *BllSesn) txSubscribe(
	c *ble.Characteristic,
	ind bool,
	fn ble.NotificationHandler) error {

	cln, err := s.getCln()
	if err != nil {
		return err
	}
	return cln.Subscribe(c, ind, fn)
}

func (s *BllSesn) txExchangeMtu(mtu uint16) (uint16, error) {
	cln, err := s.getCln()
	if err != nil {
		return 0, err
	}
	return exchangeMtu(cln, uint16(mtu))
}

func (s *BllSesn) txCancelConnection() error {
	cln, err := s.getCln()
	if err != nil {
		return err
	}
	return cln.CancelConnection()
}

func (s *BllSesn) txWriteCharacteristic(
	c *ble.Characteristic,
	b []byte,
	noRsp bool) error {

	cln, err := s.getCln()
	if err != nil {
		return err
	}
	return cln.WriteCharacteristic(c, b, noRsp)
}

func (s *BllSesn) connect() error {
	log.Debugf("Connecting to peer")

	cln, err := s.txConnect(s.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return fmt.Errorf("Failed to connect to peer after %s",
				s.cfg.ConnTimeout.String())
		} else {
			return err
		}
	}

	s.setCln(cln)
	s.listenDisconnect()

	return nil
}

func findChr(profile *ble.Profile, chrId *bledefs.BleChrId) (
	*ble.Characteristic, error) {

	if chrId == nil {
		return nil, fmt.Errorf("BLE session not configured with required " +
			"characteristic")
	}

	for _, s := range profile.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return nil, err
		}

		if bledefs.CompareUuids(uuid, chrId.SvcUuid) == 0 {
			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return nil, err
				}

				if bledefs.CompareUuids(uuid, chrId.ChrUuid) == 0 {
					return c, nil
				}
			}
		}
	}

	return nil, nil
}

func (s *BllSesn) discoverAll() error {
	log.Debugf("Discovering profile")

	p, err := s.txDiscoverProfile()
	if err != nil {
		return err
	}

	mgmtChrs, err := nmble.BuildMgmtChrs(s.cfg.MgmtProto)
	if err != nil {
		return err
	}

	s.nmpReqChr, _ = findChr(p, mgmtChrs.NmpReqChr)
	s.nmpRspChr, _ = findChr(p, mgmtChrs.NmpRspChr)
	s.publicReqChr, _ = findChr(p, mgmtChrs.ResPublicReqChr)
	s.publicRspChr, _ = findChr(p, mgmtChrs.ResPublicRspChr)
	s.unauthReqChr, _ = findChr(p, mgmtChrs.ResUnauthReqChr)
	s.unauthRspChr, _ = findChr(p, mgmtChrs.ResUnauthRspChr)
	s.secureReqChr, _ = findChr(p, mgmtChrs.ResSecureReqChr)
	s.secureRspChr, _ = findChr(p, mgmtChrs.ResSecureRspChr)

	return nil
}

// Subscribes to the peer's characteristic implementing NMP.
func (s *BllSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")

	onNotify := func(data []byte) {
		s.txvr.DispatchNmpRsp(data)
	}

	if s.nmpRspChr != nil {
		if err := s.txSubscribe(s.nmpRspChr, false, onNotify); err != nil {
			return err
		}
	}

	if s.publicRspChr != nil {
		if err := s.txSubscribe(s.publicRspChr, false, onNotify); err != nil {
			return err
		}
	}

	if s.unauthRspChr != nil {
		if err := s.txSubscribe(s.unauthRspChr, false, onNotify); err != nil {
			return err
		}
	}

	if s.secureRspChr != nil {
		if err := s.txSubscribe(s.secureRspChr, false, onNotify); err != nil {
			return err
		}
	}

	return nil
}

func (s *BllSesn) exchangeMtu() error {
	mtu, err := s.txExchangeMtu(s.cfg.PreferredMtu)
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

	txvr, err := mgmt.NewTransceiver(true, s.cfg.MgmtProto, 3)
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
		if err != nil {
			// Ensure the session is closed.
			s.Close()
		}

		if !retry {
			break
		}
	}

	return err
}

func (s *BllSesn) Close() error {
	if !s.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := s.txCancelConnection(); err != nil {
		return err
	}

	s.setCln(nil)
	return nil
}

// Indicates whether the session is currently open.
func (s *BllSesn) IsOpen() bool {
	cln, _ := s.getCln()
	return cln != nil
}

// Retrieves the maximum data payload for incoming NMP responses.
func (s *BllSesn) MtuIn() int {
	return int(s.attMtu) - nmble.NOTIFY_CMD_BASE_SZ
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (s *BllSesn) MtuOut() int {
	return util.IntMin(s.MtuIn(), bledefs.BLE_ATT_ATTR_MAX_LEN)
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (s *BllSesn) AbortRx(nmpSeq uint8) error {
	s.txvr.ErrorOne(nmpSeq, fmt.Errorf("Rx aborted"))
	return nil
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
		return s.txWriteCharacteristic(s.nmpReqChr, b, true)
	}

	return s.txvr.TxNmp(txRaw, msg, s.MtuOut(), opt.Timeout)
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

func (s *BllSesn) TxCoapOnce(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	chr, err := s.resReqChr(resType)
	if err != nil {
		return 0, nil, err
	}

	txRaw := func(b []byte) error {
		return s.txWriteCharacteristic(chr, b, !s.cfg.WriteRsp)
	}

	rsp, err := s.txvr.TxOic(txRaw, m, s.MtuOut(), opt.Timeout)
	if err != nil {
		return 0, nil, err
	} else if rsp == nil {
		return 0, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), nil
	}
}

func (s *BllSesn) TxCoapObserve(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions, NotifCb sesn.GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {
	chr, err := s.resReqChr(resType)
	if err != nil {
		return 0, nil, nil, err
	}

	txRaw := func(b []byte) error {
		return s.txWriteCharacteristic(chr, b, !s.cfg.WriteRsp)
	}

	rsp, err := s.txvr.TxOicObserve(txRaw, m, s.MtuOut(), opt.Timeout, NotifCb, stopsignal)
	if err != nil {
		return 0, nil, nil, err
	} else if rsp == nil {
		return 0, nil, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), rsp.Token(), nil
	}
}

func (s *BllSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *BllSesn) CoapIsTcp() bool {
	return true
}
