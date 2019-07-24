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
	"time"

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
}

func NewUdpSesn(cfg sesn.SesnCfg) (*UdpSesn, error) {
	s := &UdpSesn{
		cfg: cfg,
	}
	txvr, err := mgmt.NewTransceiver(cfg.TxFilterCb, cfg.RxFilterCb, false,
		cfg.MgmtProto, 3)
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

func (s *UdpSesn) TxRxMgmt(m *nmp.NmpMsg,
	timeout time.Duration) (nmp.NmpRsp, error) {

	if !s.IsOpen() {
		return nil, fmt.Errorf("Attempt to transmit over closed UDP session")
	}

	txRaw := func(b []byte) error {
		_, err := s.conn.WriteToUDP(b, s.addr)
		return err
	}
	return s.txvr.TxRxMgmt(txRaw, m, s.MtuOut(), timeout)
}

func (s *UdpSesn) AbortRx(seq uint8) error {
	s.txvr.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (s *UdpSesn) TxCoap(m coap.Message) error {
	txRaw := func(b []byte) error {
		_, err := s.conn.WriteToUDP(b, s.addr)
		return err
	}

	return s.txvr.TxCoap(txRaw, m, s.MtuOut())
}

func (s *UdpSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *UdpSesn) ListenCoap(mc nmcoap.MsgCriteria) (*nmcoap.Listener, error) {
	return s.txvr.ListenCoap(mc)
}

func (s *UdpSesn) StopListenCoap(mc nmcoap.MsgCriteria) {
	s.txvr.StopListenCoap(mc)
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
	return s.txvr.Filters()
}

func (s *UdpSesn) SetFilters(txFilter nmcoap.MsgFilter,
	rxFilter nmcoap.MsgFilter) {

	s.txvr.SetFilters(txFilter, rxFilter)
}
