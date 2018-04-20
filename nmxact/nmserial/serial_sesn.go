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

package nmserial

import (
	"fmt"
	"sync"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialSesn struct {
	cfg    sesn.SesnCfg
	sx     *SerialXport
	txvr   *mgmt.Transceiver
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex

	txFilterCb nmcoap.MsgFilter
	rxFilterCb nmcoap.MsgFilter
}

func NewSerialSesn(sx *SerialXport, cfg sesn.SesnCfg) (*SerialSesn, error) {
	s := &SerialSesn{
		cfg:        cfg,
		sx:         sx,
		txFilterCb: cfg.TxFilterCb,
		rxFilterCb: cfg.RxFilterCb,
	}

	txvr, err := mgmt.NewTransceiver(cfg.TxFilterCb, cfg.RxFilterCb, false, cfg.MgmtProto, 3)
	if err != nil {
		return nil, err
	}
	s.txvr = txvr

	return s, nil
}

func (s *SerialSesn) Open() error {
	s.m.Lock()
	defer s.m.Unlock()

	if s.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	txvr, err := mgmt.NewTransceiver(s.cfg.TxFilterCb, s.cfg.RxFilterCb, false, s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr

	s.isOpen = true
	return nil
}

func (s *SerialSesn) Close() error {
	s.m.Lock()
	defer s.m.Unlock()

	if !s.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}

	s.txvr.ErrorAll(fmt.Errorf("closed"))
	s.txvr.Stop()
	s.isOpen = false

	return nil
}

func (s *SerialSesn) IsOpen() bool {
	s.m.Lock()
	defer s.m.Unlock()

	return s.isOpen
}

func (s *SerialSesn) MtuIn() int {
	return 1024 - omp.OMP_MSG_OVERHEAD
}

func (s *SerialSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return s.sx.cfg.Mtu*3/4 - omp.OMP_MSG_OVERHEAD
}

func (s *SerialSesn) AbortRx(seq uint8) error {
	s.txvr.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (s *SerialSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	s.m.Lock()
	defer s.m.Unlock()

	if !s.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed serial session")
	}

	txFn := func(b []byte) error {
		if err := s.sx.Tx(b); err != nil {
			return err
		}

		rsp, err := s.sx.Rx()
		if err != nil {
			return err
		}
		s.txvr.DispatchNmpRsp(rsp)
		return nil
	}

	return s.txvr.TxNmp(txFn, m, s.MtuOut(), opt.Timeout)
}

func (s *SerialSesn) TxCoapOnce(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	txFn := func(b []byte) error {
		if err := s.sx.Tx(b); err != nil {
			return err
		}

		rsp, err := s.sx.Rx()
		if err != nil {
			return err
		}
		s.txvr.DispatchCoap(rsp)
		return nil
	}

	rsp, err := s.txvr.TxOic(txFn, m, s.MtuOut(), opt.Timeout)
	if err != nil {
		return 0, nil, err
	} else if rsp == nil {
		return 0, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), nil
	}
}

func (s *SerialSesn) TxCoapObserve(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions, NotifCb sesn.GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {
	return 0, nil, nil, nil
}

func (s *SerialSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *SerialSesn) CoapIsTcp() bool {
	return false
}

func (s *SerialSesn) RxAccept() (sesn.Sesn, *sesn.SesnCfg, error) {
	return nil, nil, fmt.Errorf("Op not implemented yet")
}

func (s *SerialSesn) RxCoap() (coap.Message, error) {
	return nil, fmt.Errorf("Op not implemented yet")
}

func (s *SerialSesn) Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter) {
	return s.txFilterCb, s.rxFilterCb
}
