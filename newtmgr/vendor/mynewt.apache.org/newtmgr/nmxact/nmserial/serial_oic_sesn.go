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

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialOicSesn struct {
	sx     *SerialXport
	d      *omp.Dispatcher
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex
}

func NewSerialOicSesn(sx *SerialXport) *SerialOicSesn {
	return &SerialOicSesn{
		sx: sx,
	}
}

func (sos *SerialOicSesn) Open() error {
	sos.m.Lock()
	defer sos.m.Unlock()

	if sos.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	d, err := omp.NewDispatcher(false, 3)
	if err != nil {
		return err
	}
	sos.d = d
	sos.isOpen = true
	return nil
}

func (sos *SerialOicSesn) Close() error {
	sos.m.Lock()
	defer sos.m.Unlock()

	if !sos.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}

	sos.d.Stop()
	sos.isOpen = false

	return nil
}

func (sos *SerialOicSesn) IsOpen() bool {
	sos.m.Lock()
	defer sos.m.Unlock()

	return sos.isOpen
}

func (sos *SerialOicSesn) MtuIn() int {
	return 1024 - omp.OMP_MSG_OVERHEAD
}

func (sos *SerialOicSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return 512*3/4 - omp.OMP_MSG_OVERHEAD
}

func (sos *SerialOicSesn) AbortRx(seq uint8) error {
	sos.d.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (sos *SerialOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (sos *SerialOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	sos.m.Lock()
	defer sos.m.Unlock()

	if !sos.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed serial session")
	}

	nl, err := sos.d.AddNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer sos.d.RemoveNmpListener(m.Hdr.Seq)

	reqb, err := sos.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if err := sos.sx.Tx(reqb); err != nil {
		return nil, err
	}

	for {
		rspb, err := sos.sx.Rx()
		if err != nil {
			return nil, err
		}

		// Now wait for newtmgr response.
		if sos.d.Dispatch(rspb) {
			select {
			case err := <-nl.ErrChan:
				return nil, err
			case rsp := <-nl.RspChan:
				return rsp, nil
			}
		}
	}
}

func (sos *SerialOicSesn) GetResourceOnce(resType sesn.ResourceType,
	uri string, opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	token := nmxutil.NextToken()

	ol, err := sos.d.AddOicListener(token)
	if err != nil {
		return 0, nil, err
	}
	defer sos.d.RemoveOicListener(token)

	req, err := oic.EncodeGet(false, uri, token)
	if err != nil {
		return 0, nil, err
	}

	if err := sos.sx.Tx(req); err != nil {
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

func (sos *SerialOicSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte,
	opt sesn.TxOptions) (coap.COAPCode, error) {

	return 0, fmt.Errorf("SerialOicSesn.PutResourceOnce() unsupported")
}
