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

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/task"
)

type NakedSesnState int

const (
	// Session not open and no open in progress.
	NS_STATE_CLOSED NakedSesnState = iota

	// Open in progress.
	NS_STATE_OPENING_ACTIVE

	// After a failed open; additional retries remain.
	NS_STATE_OPENING_IDLE

	// Open complete.
	NS_STATE_OPEN
)

// Implements a BLE session that does not acquire the master resource on
// connect.  The user of this type must acquire the resource manually.
type NakedSesn struct {
	cfg      sesn.SesnCfg
	bx       *BleXport
	conn     *Conn
	mgmtChrs BleMgmtChrs
	txvr     *mgmt.Transceiver
	tq       task.TaskQueue

	wg sync.WaitGroup

	stopChan chan struct{}

	// Protects `enabled` and `opening`.
	mtx sync.Mutex

	state NakedSesnState

	shuttingDown bool

	smIo SmIo

	txFilterCb nmcoap.MsgFilter
	rxFilterCb nmcoap.MsgFilter
}

func (s *NakedSesn) init() error {
	s.conn = NewConn(s.bx)
	s.stopChan = make(chan struct{})

	if s.txvr != nil {
		s.txvr.Stop()
	}

	txvr, err := mgmt.NewTransceiver(s.txFilterCb, s.rxFilterCb, true, s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr

	s.tq.Stop(fmt.Errorf("Ensuring task is stopped"))
	if err := s.tq.Start(10); err != nil {
		nmxutil.Assert(false)
		return err
	}

	return nil
}

func NewNakedSesn(bx *BleXport, cfg sesn.SesnCfg) (*NakedSesn, error) {
	mgmtChrs, err := BuildMgmtChrs(cfg.MgmtProto)
	if err != nil {
		return nil, err
	}

	s := &NakedSesn{
		cfg:        cfg,
		bx:         bx,
		mgmtChrs:   mgmtChrs,
		txFilterCb: cfg.TxFilterCb,
		rxFilterCb: cfg.RxFilterCb,
	}

	s.init()

	return s, nil
}

func (s *NakedSesn) runTask(fn func() error) error {
	err := s.tq.Run(fn)
	if err == task.InactiveError {
		return nmxutil.NewXportError("attempt to use closed BLE session")
	}
	return err
}

func (s *NakedSesn) shutdown(cause error) error {
	initiate := func() error {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		if s.shuttingDown || s.state == NS_STATE_CLOSED || s.state == NS_STATE_OPENING_IDLE {
			return nmxutil.NewSesnClosedError(
				"Attempt to close an already-closed session")
		}
		s.shuttingDown = true

		return nil
	}

	if err := initiate(); err != nil {
		return err
	}
	defer func() {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		s.shuttingDown = false
	}()

	// Stop the task queue to flush all pending events.
	s.tq.StopNoWait(cause)

	s.conn.Stop()

	if s.IsOpen() {
		s.bx.RemoveSesn(s.conn.connHandle)
	}

	// Signal error to all listeners.
	s.txvr.ErrorAll(cause)
	s.txvr.Stop()

	// Stop Goroutines associated with notification listeners.
	close(s.stopChan)

	// Block until close completes.
	s.wg.Wait()

	// Call the on-close callback if the session was fully open.
	s.mtx.Lock()
	fullyOpen := s.state == NS_STATE_OPEN
	if fullyOpen {
		s.state = NS_STATE_CLOSED
	} else {
		s.state = NS_STATE_OPENING_IDLE
	}
	s.mtx.Unlock()

	if fullyOpen && s.cfg.OnCloseCb != nil {
		s.cfg.OnCloseCb(s, cause)
	}

	return nil
}

func (s *NakedSesn) enqueueShutdown(cause error) chan error {
	return s.tq.Enqueue(func() error { return s.shutdown(cause) })
}

func (s *NakedSesn) initiateSecurity() error {
	if err := s.conn.InitiateSecurity(); err != nil {
		if serr := ToSecurityErr(err); serr != nil {
			return serr
		} else {
			return err
		}
	}
	return nil
}

func (s *NakedSesn) Open() error {
	initiate := func() error {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		if s.state != NS_STATE_CLOSED {
			return nmxutil.NewSesnAlreadyOpenError(
				"Attempt to open an already-open BLE session")
		}

		s.state = NS_STATE_OPENING_ACTIVE
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}

	var err error
	for i := 0; i < s.cfg.Ble.Central.ConnTries; i++ {
		var retry bool

		retry, err = s.openOnce()
		if err != nil {
			s.shutdown(err)
		}

		if !retry {
			break
		}
	}

	if err != nil {
		s.mtx.Lock()
		s.state = NS_STATE_CLOSED
		s.mtx.Unlock()
		return err
	}

	s.bx.AddSesn(s.conn.connHandle, s)

	s.mtx.Lock()
	s.state = NS_STATE_OPEN
	s.mtx.Unlock()

	return nil
}

func (s *NakedSesn) OpenConnected(
	connHandle uint16, eventListener *Listener) error {

	initiate := func() error {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		if s.state != NS_STATE_CLOSED {
			return nmxutil.NewSesnAlreadyOpenError(
				"Attempt to open an already-open BLE session")
		}

		s.state = NS_STATE_OPENING_ACTIVE
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}
	defer func() {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		s.state = NS_STATE_CLOSED
	}()

	if err := s.init(); err != nil {
		return err
	}

	if err := s.conn.Inherit(connHandle, eventListener); err != nil {
		return err
	}

	// Listen for disconnect in the background.
	s.disconnectListen()

	// Listen for incoming notifications in the background.
	s.notifyListen()

	// Listen for authentication IO requests in the background.
	s.smIoDemandListen()

	if s.cfg.Ble.EncryptWhen == BLE_ENCRYPT_ALWAYS {
		if err := s.initiateSecurity(); err != nil {
			return err
		}
	}

	// Give a record of this open session to the transport.
	s.bx.AddSesn(connHandle, s)

	s.mtx.Lock()
	s.state = NS_STATE_OPEN
	s.mtx.Unlock()

	return nil
}

func (s *NakedSesn) failIfNotOpen() error {
	if !s.IsOpen() {
		return nmxutil.NewSesnClosedError("Attempt to use closed session")
	}
	return nil
}

func (s *NakedSesn) TxNmpOnce(req *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if err := s.failIfNotOpen(); err != nil {
		return nil, err
	}

	var rsp nmp.NmpRsp

	fn := func() error {
		chr, err := s.getChr(s.mgmtChrs.NmpReqChr)
		if err != nil {
			return err
		}

		txRaw := func(b []byte) error {
			if s.cfg.Ble.WriteRsp {
				return s.conn.WriteChr(chr, b, "nmp")
			} else {
				return s.conn.WriteChrNoRsp(chr, b, "nmp")
			}
		}

		rsp, err = s.txvr.TxNmp(txRaw, req, s.MtuOut(), opt.Timeout)
		return err
	}

	if err := s.runTask(fn); err != nil {
		return nil, err
	}

	return rsp, nil
}

func (s *NakedSesn) TxCoapOnce(m coap.Message,
	resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	if err := s.failIfNotOpen(); err != nil {
		return 0, nil, err
	}

	var rspCode coap.COAPCode
	var rspPayload []byte

	fn := func() error {
		chrId := ResChrReqIdLookup(s.mgmtChrs, resType)
		chr, err := s.getChr(chrId)
		if err != nil {
			return err
		}

		encReqd, authReqd, err := ResTypeSecReqs(resType)
		if err != nil {
			return err
		}
		if err := s.ensureSecurity(encReqd, authReqd); err != nil {
			return err
		}

		txRaw := func(b []byte) error {
			if s.cfg.Ble.WriteRsp {
				return s.conn.WriteChr(chr, b, "coap")
			} else {
				return s.conn.WriteChrNoRsp(chr, b, "coap")
			}
		}

		rsp, err := s.txvr.TxOic(txRaw, m, s.MtuOut(), opt.Timeout)
		if err == nil && rsp != nil {
			rspCode = rsp.Code()
			rspPayload = rsp.Payload()
		}
		return err
	}

	if err := s.runTask(fn); err != nil {
		return 0, nil, err
	}

	return rspCode, rspPayload, nil
}

func (s *NakedSesn) TxCoapObserve(m coap.Message, resType sesn.ResourceType, opt sesn.TxOptions,
	NotifyCb sesn.GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {
	if err := s.failIfNotOpen(); err != nil {
		return 0, nil, nil, err
	}

	var rspCode coap.COAPCode
	var rspPayload []byte
	var rspToken []byte

	fn := func() error {
		chrId := ResChrReqIdLookup(s.mgmtChrs, resType)
		chr, err := s.getChr(chrId)
		if err != nil {
			return err
		}

		encReqd, authReqd, err := ResTypeSecReqs(resType)
		if err != nil {
			return err
		}
		if err := s.ensureSecurity(encReqd, authReqd); err != nil {
			return err
		}

		txRaw := func(b []byte) error {
			if s.cfg.Ble.WriteRsp {
				return s.conn.WriteChr(chr, b, "coap")
			} else {
				return s.conn.WriteChrNoRsp(chr, b, "coap")
			}
		}

		rsp, err := s.txvr.TxOicObserve(txRaw, m, s.MtuOut(), opt.Timeout, NotifyCb, stopsignal)
		if err == nil && rsp != nil {
			rspCode = rsp.Code()
			rspPayload = rsp.Payload()
			rspToken = rsp.Token()
		}
		return err
	}

	if err := s.runTask(fn); err != nil {
		return 0, nil, nil, err
	}

	return rspCode, rspPayload, rspToken, nil
}

func (s *NakedSesn) AbortRx(seq uint8) error {
	if err := s.failIfNotOpen(); err != nil {
		return err
	}

	fn := func() error {
		s.txvr.AbortRx(seq)
		return nil
	}
	return s.runTask(fn)
}

func (s *NakedSesn) Close() error {
	if err := s.failIfNotOpen(); err != nil {
		return err
	}

	fn := func() error {
		return s.shutdown(fmt.Errorf("BLE session manually closed"))
	}

	return s.runTask(fn)
}

func (s *NakedSesn) IsOpen() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.state == NS_STATE_OPEN
}

func (s *NakedSesn) MtuIn() int {
	return int(s.conn.AttMtu()) - NOTIFY_CMD_BASE_SZ
}

func (s *NakedSesn) MtuOut() int {
	return util.IntMin(s.MtuIn(), BLE_ATT_ATTR_MAX_LEN)
}

func (s *NakedSesn) CoapIsTcp() bool {
	return true
}

func (s *NakedSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *NakedSesn) ConnInfo() (BleConnDesc, error) {
	if err := s.failIfNotOpen(); err != nil {
		return BleConnDesc{}, err
	}

	return s.conn.ConnInfo(), nil
}

func (s *NakedSesn) SetOobKey(key []byte) {
	s.smIo.Oob = key
}

func (s *NakedSesn) openOnce() (bool, error) {
	s.mtx.Lock()
	s.state = NS_STATE_OPENING_ACTIVE
	s.mtx.Unlock()

	if err := s.init(); err != nil {
		return false, err
	}

	// Listen for disconnect in the background.
	s.disconnectListen()

	if err := s.conn.Connect(
		s.cfg.Ble.OwnAddrType,
		s.cfg.PeerSpec.Ble,
		s.cfg.Ble.Central.ConnTimeout); err != nil {

		// An ENOTCONN error code implies the "conn_find" request failed
		// because the connection dropped immediately after being established.
		// If this happened, retry the connect procedure.
		bhdErr := nmxutil.ToBleHost(err)
		retry := bhdErr != nil && bhdErr.Status == ERR_CODE_ENOTCONN
		return retry, err
	}

	if err := s.conn.ExchangeMtu(); err != nil {
		// An ENOTCONN error code implies the connection dropped before the
		// first ACL data transmission.  If this happened, retry the connect
		// procedure.
		bhdErr := nmxutil.ToBleHost(err)
		retry := bhdErr != nil && bhdErr.Status == ERR_CODE_ENOTCONN
		return retry, err
	}

	if err := s.conn.DiscoverSvcs(); err != nil {
		return false, err
	}

	if chr, _ := s.getChr(s.mgmtChrs.NmpRspChr); chr != nil {
		if chr.SubscribeType() != 0 {
			if err := s.conn.Subscribe(chr); err != nil {
				return false, err
			}
		}
	}

	// Listen for incoming notifications in the background.
	s.notifyListen()

	// Listen for authentication IO requests in the background.
	s.smIoDemandListen()

	if s.cfg.Ble.EncryptWhen == BLE_ENCRYPT_ALWAYS {
		if err := s.initiateSecurity(); err != nil {
			return false, err
		}
	}

	return false, nil
}

func (s *NakedSesn) smRespondIo(dmnd SmIoDemand) error {
	io := SmIo{
		Action: dmnd.Action,
	}

	switch dmnd.Action {
	case BLE_SM_ACTION_OOB:
		if s.smIo.Oob == nil {
			return fmt.Errorf("OOB key requested but none configured; " +
				"allowing pairing procedure to time out")
		}
		io.Oob = s.smIo.Oob

	case BLE_SM_ACTION_INPUT, BLE_SM_ACTION_DISP, BLE_SM_ACTION_NUMCMP:
		return fmt.Errorf("Unsupported SM IO method requested: %s",
			io.Action.String())

	default:
		return fmt.Errorf("Unknown SM IO method requested: %v", io.Action)
	}

	return s.conn.SmInjectIo(io)
}

// Listens for disconnect in the background.
func (s *NakedSesn) disconnectListen() {
	discChan := s.conn.DisconnectChan()

	// Terminates on:
	// * Receive from connection disconnect-channel.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Block until disconnect.
		err := <-discChan
		s.enqueueShutdown(err)
	}()
}

func (s *NakedSesn) smIoDemandListen() {
	// Terminates on:
	// * Receive from stop channel.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case dmnd, ok := <-s.conn.SmIoDemandChan():
				if ok {
					log.Debugf("Received SM IO demand for %s",
						dmnd.Action.String())
					s.smRespondIo(dmnd)
				}

			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *NakedSesn) getChr(chrId *BleChrId) (*Characteristic, error) {
	if chrId == nil {
		return nil, fmt.Errorf("BLE session not configured with required " +
			"characteristic")
	}

	chr := s.conn.Profile().FindChrByUuid(*chrId)
	if chr == nil {
		return nil, fmt.Errorf("BLE peer doesn't support required "+
			"characteristic: %s", chrId.String())
	}

	return chr, nil
}

func (s *NakedSesn) createNotifyListener(chrId *BleChrId) (
	*NotifyListener, error) {

	chr, err := s.getChr(chrId)
	if err != nil {
		return nil, err
	}

	return s.conn.ListenForNotifications(chr)
}

func (s *NakedSesn) notifyListenOnce(chrId *BleChrId,
	dispatchCb func(b []byte)) {

	nl, err := s.createNotifyListener(chrId)
	if err != nil {
		log.Debugf("error listening for notifications: %s", err.Error())
		return
	}

	stopChan := s.stopChan

	// Terminates on:
	// * Notify listener error.
	// * Receive from stop channel.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case <-nl.ErrChan:
				return

			case n, ok := <-nl.NotifyChan:
				if ok {
					dispatchCb(n.Data)
				}

			case <-stopChan:
				return
			}
		}
	}()
}

func (s *NakedSesn) notifyListen() {
	s.notifyListenOnce(s.mgmtChrs.ResUnauthRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.ResSecureRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.ResPublicRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.NmpRspChr, s.txvr.DispatchNmpRsp)
}

func (s *NakedSesn) checkSecurity(encReqd bool, authReqd bool) (bool, bool) {
	desc, _ := s.ConnInfo()

	return !encReqd || desc.Encrypted,
		!authReqd || desc.Authenticated
}

func (s *NakedSesn) ensureSecurity(encReqd bool, authReqd bool) error {
	encGood, authGood := s.checkSecurity(encReqd, authReqd)
	if encGood && authGood {
		return nil
	}

	if err := s.initiateSecurity(); err != nil {
		return err
	}

	// Ensure pairing meets characteristic's requirements.
	encGood, authGood = s.checkSecurity(encReqd, authReqd)
	if !encGood {
		return fmt.Errorf("Insufficient BLE security; " +
			"characteristic requires encryption")
	}

	if !authGood {
		return fmt.Errorf("Insufficient BLE security; " +
			"characteristic  requires authentication")
	}

	return nil
}

func (s *NakedSesn) RxAccept() (sesn.Sesn, *sesn.SesnCfg, error) {
	return nil, nil, fmt.Errorf("Op not implemented yet")
}

func (s *NakedSesn) RxCoap(opt sesn.TxOptions) (coap.Message, error) {
	return nil, fmt.Errorf("Op not implemented yet")
}

func (s *NakedSesn) Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter) {
	return s.txFilterCb, s.rxFilterCb
}
