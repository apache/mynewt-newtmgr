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

package udp

import (
	"fmt"
	"net"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type UdpSesn struct {
	cfg  sesn.SesnCfg
	addr *net.UDPAddr
	conn *net.UDPConn
	txvr *mgmt.Transceiver

	txFilterCb nmcoap.MsgFilter
	rxFilterCb nmcoap.MsgFilter
}

func NewUdpSesn(cfg sesn.SesnCfg) (*UdpSesn, error) {
	s := &UdpSesn{
		cfg:        cfg,
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

func (s *UdpSesn) Open() error {
	if s.conn != nil {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open UDP session")
	}

	conn, addr, err := Listen(s.cfg.PeerSpec.Udp,
		func(data []byte) {
			s.txvr.DispatchNmpRsp(data)
		})
	if err != nil {
		return err
	}

	s.addr = addr
	s.conn = conn
	return nil
}

func (s *UdpSesn) Close() error {
	if s.conn == nil {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened UDP session")
	}

	s.conn.Close()
	s.txvr.ErrorAll(fmt.Errorf("closed"))
	s.txvr.Stop()
	s.conn = nil
	s.addr = nil
	return nil
}

func (s *UdpSesn) IsOpen() bool {
	return s.conn != nil
}

func (s *UdpSesn) MtuIn() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (s *UdpSesn) MtuOut() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (s *UdpSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !s.IsOpen() {
		return nil, fmt.Errorf("Attempt to transmit over closed UDP session")
	}

	txRaw := func(b []byte) error {
		_, err := s.conn.WriteToUDP(b, s.addr)
		return err
	}
	return s.txvr.TxNmp(txRaw, m, s.MtuOut(), opt.Timeout)
}

func (s *UdpSesn) AbortRx(seq uint8) error {
	s.txvr.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (s *UdpSesn) TxCoapOnce(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	txRaw := func(b []byte) error {
		_, err := s.conn.WriteToUDP(b, s.addr)
		return err
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

func (s *UdpSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *UdpSesn) TxCoapObserve(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions, NotifyCb sesn.GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {

	txRaw := func(b []byte) error {
		_, err := s.conn.WriteToUDP(b, s.addr)
		return err
	}

	rsp, err := s.txvr.TxOicObserve(txRaw, m, s.MtuOut(), opt.Timeout, NotifyCb, stopsignal)
	if err != nil {
		return 0, nil, nil, err
	} else if rsp == nil {
		return 0, nil, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), rsp.Token(), nil
	}
}

func (s *UdpSesn) CoapIsTcp() bool {
	return false
}

func (s *UdpSesn) RxAccept() (sesn.Sesn, *sesn.SesnCfg, error) {
	return nil, nil, fmt.Errorf("Op not implemented yet")
}

func (s *UdpSesn) RxCoap(opt sesn.TxOptions) (coap.Message, error) {
	return nil, fmt.Errorf("Op not implemented yet")
}

func (s *UdpSesn) Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter) {
	return s.txFilterCb, s.rxFilterCb
}
