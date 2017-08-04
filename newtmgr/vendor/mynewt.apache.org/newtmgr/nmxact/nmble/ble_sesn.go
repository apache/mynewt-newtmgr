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

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

var dummyNotifyListener = NewNotifyListener()

type BleSesn struct {
	cfg      sesn.SesnCfg
	bx       *BleXport
	conn     *Conn
	mgmtChrs BleMgmtChrs
	txvr     *mgmt.Transceiver

	wg           sync.WaitGroup
	closeBlocker nmxutil.Blocker
	stopChan     chan struct{}
}

func (s *BleSesn) init() error {
	s.conn = NewConn(s.bx)

	s.stopChan = make(chan struct{})

	txvr, err := mgmt.NewTransceiver(s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr
	return nil
}

func NewBleSesn(bx *BleXport, cfg sesn.SesnCfg) (*BleSesn, error) {
	mgmtChrs, err := BuildMgmtChrs(cfg.MgmtProto)
	if err != nil {
		return nil, err
	}

	s := &BleSesn{
		cfg:      cfg,
		bx:       bx,
		mgmtChrs: mgmtChrs,
	}

	if err := s.init(); err != nil {
		return nil, err
	}

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
		defer s.closeBlocker.Unblock()

		// Block until disconnect.
		err := <-s.conn.DisconnectChan()
		nmxutil.Assert(!s.IsOpen())

		// Signal error to all listeners.
		close(s.stopChan)
		s.wg.Done()
		s.wg.Wait()

		if s.cfg.OnCloseCb != nil {
			s.cfg.OnCloseCb(s, err)
		}
	}()
}

func (s *BleSesn) getChr(chrId *BleChrId) (*Characteristic, error) {
	if chrId == nil {
		return nil, fmt.Errorf("BLE session not configured with required "+
			"characteristic: %s", *chrId)
	}

	chr := s.conn.Profile().FindChrByUuid(*chrId)
	if chr == nil {
		return nil, fmt.Errorf("BLE peer doesn't support required "+
			"characteristic: %s", *chrId)
	}

	return chr, nil
}

func (s *BleSesn) createNotifyListener(chrId *BleChrId) *NotifyListener {
	chr, _ := s.getChr(chrId)
	if chr == nil {
		return dummyNotifyListener
	}

	nl := s.conn.ListenForNotifications(chr)
	if nl == nil {
		return dummyNotifyListener
	}

	return nl
}

func (s *BleSesn) notifyListen() {
	nmpRspNl := s.createNotifyListener(s.mgmtChrs.NmpRspChr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case err := <-nmpRspNl.ErrChan:
				s.txvr.ErrorAll(err)
				return

			case n, ok := <-nmpRspNl.NotifyChan:
				if ok {
					s.txvr.DispatchNmpRsp(n.Data)
				}

			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *BleSesn) openOnce() (bool, error) {
	if s.IsOpen() {
		return false, nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
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

	return err
}

func (s *BleSesn) OpenConnected(
	connHandle uint16, eventListener *Listener) error {

	if err := s.conn.Inherit(connHandle, eventListener); err != nil {
		if !nmxutil.IsSesnAlreadyOpen(err) {
			s.closeBlocker.Unblock()
		}
		return err
	}

	// Ensure subsequent calls to Close() block.
	s.closeBlocker.Block()

	// Listen for disconnect in the background.
	s.disconnectListen()

	// Listen for incoming notifications in the background.
	s.notifyListen()

	return nil
}

func (s *BleSesn) Close() error {
	if err := s.conn.Stop(); err != nil {
		return err
	}

	// Block until close completes.
	s.closeBlocker.Wait()
	return nil
}

func (s *BleSesn) IsOpen() bool {
	return s.conn.IsConnected()
}

func (s *BleSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	switch s.cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return nmp.EncodeNmpPlain(m)

	case sesn.MGMT_PROTO_OMP:
		return omp.EncodeOmpTcp(m)

	default:
		return nil,
			fmt.Errorf("invalid management protocol: %+v", s.cfg.MgmtProto)
	}
}

// Blocking.
func (s *BleSesn) TxNmpOnce(req *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	chr, err := s.getChr(s.mgmtChrs.NmpReqChr)
	if err != nil {
		return nil, err
	}

	txRaw := func(b []byte) error {
		return s.conn.WriteChrNoRsp(chr, b, "nmp")
	}

	return s.txvr.TxNmp(txRaw, req, opt.Timeout)
}

func (s *BleSesn) MtuIn() int {
	return s.conn.AttMtu() -
		NOTIFY_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (s *BleSesn) MtuOut() int {
	mtu := s.conn.AttMtu() -
		WRITE_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
	return util.IntMin(mtu, BLE_ATT_ATTR_MAX_LEN)
}

func (s *BleSesn) ConnInfo() (BleConnDesc, error) {
	return s.conn.ConnInfo(), nil
}

func (s *BleSesn) GetResourceOnce(resType sesn.ResourceType, uri string,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	token := nmxutil.NextToken()
	req, err := oic.CreateGet(true, uri, token)
	if err != nil {
		return 0, nil, err
	}

	chrId := ResChrIdLookup(s.mgmtChrs, resType)
	chr, err := s.getChr(chrId)
	if err != nil {
		return 0, nil, err
	}

	txRaw := func(b []byte) error {
		return s.conn.WriteChrNoRsp(chr, b, "oic")
	}

	rsp, err := s.txvr.TxOic(txRaw, req, opt.Timeout)
	if err != nil {
		return 0, nil, err
	}

	return rsp.Code(), rsp.Payload(), nil
}

func (s *BleSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte, opt sesn.TxOptions) (coap.COAPCode, error) {

	return 0, fmt.Errorf("SerialPlainSesn.PutResourceOnce() unsupported")
}
