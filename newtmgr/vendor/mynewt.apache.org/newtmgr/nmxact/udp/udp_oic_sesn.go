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

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type UdpOicSesn struct {
	cfg  sesn.SesnCfg
	addr *net.UDPAddr
	conn *net.UDPConn
	d    *omp.Dispatcher
}

func NewUdpOicSesn(cfg sesn.SesnCfg) *UdpOicSesn {
	uos := &UdpOicSesn{
		cfg: cfg,
	}

	return uos
}

func (uos *UdpOicSesn) Open() error {
	if uos.conn != nil {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open UDP session")
	}

	conn, addr, err := Listen(uos.cfg.PeerSpec.Udp,
		func(data []byte) {
			uos.d.Dispatch(data)
		})
	if err != nil {
		return err
	}

	d, err := omp.NewDispatcher(false, 3)
	if err != nil {
		return err
	}
	uos.d = d
	uos.addr = addr
	uos.conn = conn
	return nil
}

func (uos *UdpOicSesn) Close() error {
	if uos.conn == nil {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened UDP session")
	}

	uos.conn.Close()
	uos.d.Stop()
	uos.conn = nil
	uos.addr = nil
	return nil
}

func (uos *UdpOicSesn) IsOpen() bool {
	return uos.conn != nil
}

func (uos *UdpOicSesn) MtuIn() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (uos *UdpOicSesn) MtuOut() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (uos *UdpOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (uos *UdpOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !uos.IsOpen() {
		return nil, fmt.Errorf("Attempt to transmit over closed UDP session")
	}

	nl, err := uos.d.AddNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer uos.d.RemoveNmpListener(m.Hdr.Seq)

	b, err := uos.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if _, err := uos.conn.WriteToUDP(b, uos.addr); err != nil {
		return nil, err
	}

	select {
	case err := <-nl.ErrChan:
		return nil, err
	case rsp := <-nl.RspChan:
		return rsp, nil
	case <-nl.AfterTimeout(opt.Timeout):
		msg := fmt.Sprintf(
			"NMP timeout; op=%d group=%d id=%d seq=%d peer=%#v",
			b[0], b[4]+b[5]<<8, b[7], b[6], uos.addr)

		return nil, nmxutil.NewRspTimeoutError(msg)
	}
}

func (uos *UdpOicSesn) AbortRx(seq uint8) error {
	uos.d.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (uos *UdpOicSesn) GetResourceOnce(resType sesn.ResourceType, uri string,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	token := nmxutil.NextToken()

	ol, err := uos.d.AddOicListener(token)
	if err != nil {
		return 0, nil, err
	}
	defer uos.d.RemoveOicListener(token)

	req, err := oic.EncodeGet(false, uri, token)
	if err != nil {
		return 0, nil, err
	}

	if _, err := uos.conn.WriteToUDP(req, uos.addr); err != nil {
		return 0, nil, err
	}

	for {
		select {
		case err := <-ol.ErrChan:
			return 0, nil, err
		case rsp := <-ol.RspChan:
			return rsp.Code(), rsp.Payload(), nil
		case <-ol.AfterTimeout(opt.Timeout):
			return 0, nil, nmxutil.NewRspTimeoutError("OIC timeout")
		}
	}
}

func (uos *UdpOicSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte,
	opt sesn.TxOptions) (coap.COAPCode, error) {

	return 0, fmt.Errorf("UdpOicSesn.PutResourceOnce() unsupported")
}
