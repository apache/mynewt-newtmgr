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
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BleSesn struct {
	cfg      sesn.SesnCfg
	bx       *BleXport
	prio     MasterPrio
	conn     *Conn
	mgmtChrs BleMgmtChrs
	txvr     *mgmt.Transceiver

	wg           sync.WaitGroup
	closeBlocker nmxutil.Blocker
	stopChan     chan struct{}
}

func (s *BleSesn) init() error {
	s.conn = NewConn(s.bx, s.prio)
	s.stopChan = make(chan struct{})

	if s.txvr != nil {
		s.txvr.Stop()
	}

	txvr, err := mgmt.NewTransceiver(true, s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr

	return nil
}

func NewBleSesn(bx *BleXport, cfg sesn.SesnCfg, prio MasterPrio) (
	*BleSesn, error) {

	mgmtChrs, err := BuildMgmtChrs(cfg.MgmtProto)
	if err != nil {
		return nil, err
	}

	s := &BleSesn{
		cfg:      cfg,
		bx:       bx,
		prio:     prio,
		mgmtChrs: mgmtChrs,
	}

	s.init()

	return s, nil
}

func (s *BleSesn) AbortRx(seq uint8) error {
	s.txvr.AbortRx(seq)
	return nil
}

func (s *BleSesn) disconnectListen() {
	// Listen for disconnect in the background.
	s.wg.Add(1)
	go func() {
		// If the session is being closed, unblock the close() call.
		defer s.closeBlocker.Unblock(nil)

		// Block until disconnect.
		err := <-s.conn.DisconnectChan()
		nmxutil.Assert(!s.IsOpen())

		s.bx.removeSesn(s.conn.connHandle)

		// Signal error to all listeners.
		s.txvr.ErrorAll(err)
		s.txvr.Stop()

		// Stop all go routines.
		close(s.stopChan)
		s.wg.Done()
		s.wg.Wait()

		if s.cfg.OnCloseCb != nil {
			s.cfg.OnCloseCb(s, err)
		}
	}()
}

func (s *BleSesn) errIfClosed() error {
	if !s.IsOpen() {
		return nmxutil.NewSesnClosedError("Attempt to use closed BLE session")
	}
	return nil
}

func (s *BleSesn) getChr(chrId *BleChrId) (*Characteristic, error) {
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

func (s *BleSesn) createNotifyListener(chrId *BleChrId) (
	*NotifyListener, error) {

	chr, err := s.getChr(chrId)
	if err != nil {
		return nil, err
	}

	return s.conn.ListenForNotifications(chr)
}

func (s *BleSesn) notifyListenOnce(chrId *BleChrId,
	dispatchCb func(b []byte)) {

	nl, err := s.createNotifyListener(chrId)
	if err != nil {
		log.Debugf("error listening for notifications: %s", err.Error())
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case <-nl.ErrChan:
				return

			case n, ok := <-nl.NotifyChan:
				if !ok {
					return
				}
				dispatchCb(n.Data)

			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *BleSesn) notifyListen() {
	s.notifyListenOnce(s.mgmtChrs.ResUnauthRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.ResSecureRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.ResPublicRspChr, s.txvr.DispatchCoap)
	s.notifyListenOnce(s.mgmtChrs.NmpRspChr, s.txvr.DispatchNmpRsp)
}

func (s *BleSesn) openOnce() (bool, error) {
	if s.IsOpen() {
		return false, nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open BLE session")
	}

	if err := s.init(); err != nil {
		return false, err
	}

	// Ensure subsequent calls to Close() block.
	s.closeBlocker.Block()

	// Listen for disconnect in the background.
	s.disconnectListen()

	if err := s.conn.Connect(
		s.bx,
		s.cfg.Ble.OwnAddrType,
		s.cfg.PeerSpec.Ble,
		s.cfg.Ble.Central.ConnTimeout); err != nil {

		return false, err
	}

	if err := s.conn.ExchangeMtu(); err != nil {
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

	if s.cfg.Ble.EncryptWhen == BLE_ENCRYPT_ALWAYS {
		if err := s.conn.InitiateSecurity(); err != nil {
			return false, err
		}
	}

	// Listen for incoming notifications in the background.
	s.notifyListen()

	return false, nil
}

func (s *BleSesn) Open() error {
	var err error
	for i := 0; i < s.cfg.Ble.Central.ConnTries; i++ {
		var retry bool

		retry, err = s.openOnce()
		if err != nil {
			s.Close()
		}

		if !retry {
			break
		}
	}

	if err != nil {
		s.bx.addSesn(s.conn.connHandle, s)
	}

	return err
}

func (s *BleSesn) OpenConnected(
	connHandle uint16, eventListener *Listener) error {

	if err := s.conn.Inherit(connHandle, eventListener); err != nil {
		if !nmxutil.IsSesnAlreadyOpen(err) {
			s.closeBlocker.Unblock(nil)
		}
		return err
	}

	// Ensure subsequent calls to Close() block.
	s.closeBlocker.Block()

	// Listen for disconnect in the background.
	s.disconnectListen()

	// Listen for incoming notifications in the background.
	s.notifyListen()

	s.bx.addSesn(connHandle, s)

	return nil
}

func (s *BleSesn) Close() error {
	if err := s.conn.Stop(); err != nil {
		return err
	}

	// Block until close completes.
	s.closeBlocker.Wait(nmxutil.DURATION_FOREVER)
	return nil
}

func (s *BleSesn) IsOpen() bool {
	return s.conn.IsConnected()
}

func (s *BleSesn) MtuIn() int {
	return int(s.conn.AttMtu()) - NOTIFY_CMD_BASE_SZ
}

func (s *BleSesn) MtuOut() int {
	return util.IntMin(s.MtuIn(), BLE_ATT_ATTR_MAX_LEN)
}

func (s *BleSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *BleSesn) ConnInfo() (BleConnDesc, error) {
	return s.conn.ConnInfo(), nil
}

func (s *BleSesn) TxNmpOnce(req *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if err := s.errIfClosed(); err != nil {
		return nil, err
	}

	chr, err := s.getChr(s.mgmtChrs.NmpReqChr)
	if err != nil {
		return nil, err
	}

	txRaw := func(b []byte) error {
		return s.conn.WriteChrNoRsp(chr, b, "nmp")
	}

	return s.txvr.TxNmp(txRaw, req, s.MtuOut(), opt.Timeout)
}

func (s *BleSesn) TxCoapOnce(m coap.Message,
	resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	if err := s.errIfClosed(); err != nil {
		return 0, nil, err
	}

	chrId := ResChrReqIdLookup(s.mgmtChrs, resType)
	chr, err := s.getChr(chrId)
	if err != nil {
		return 0, nil, err
	}
	txRaw := func(b []byte) error {
		return s.conn.WriteChrNoRsp(chr, b, "coap")
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
