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
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialPlainSesn struct {
	sx     *SerialXport
	d      *nmp.Dispatcher
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex
}

func NewSerialPlainSesn(sx *SerialXport) *SerialPlainSesn {
	return &SerialPlainSesn{
		sx: sx,
		d:  nmp.NewDispatcher(1),
	}
}

func (sps *SerialPlainSesn) Open() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if sps.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	sps.isOpen = true
	return nil
}

func (sps *SerialPlainSesn) Close() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if !sps.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}
	sps.isOpen = false
	return nil
}

func (sps *SerialPlainSesn) IsOpen() bool {
	sps.m.Lock()
	defer sps.m.Unlock()

	return sps.isOpen
}

func (sps *SerialPlainSesn) MtuIn() int {
	return 1024
}

func (sps *SerialPlainSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return 512 * 3 / 4
}

func (sps *SerialPlainSesn) AbortRx(seq uint8) error {
	return sps.d.ErrorOne(seq, fmt.Errorf("Rx aborted"))
}

func (sps *SerialPlainSesn) addListener(seq uint8) (
	*nmp.Listener, error) {

	nl, err := sps.d.AddListener(seq)
	if err != nil {
		return nil, err
	}

	return nl, nil
}

func (sps *SerialPlainSesn) removeListener(seq uint8) {
	sps.d.RemoveListener(seq)
}

func (sps *SerialPlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(m)
}

func (sps *SerialPlainSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	sps.m.Lock()
	defer sps.m.Unlock()

	if !sps.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed serial session")
	}

	nl, err := sps.addListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer sps.removeListener(m.Hdr.Seq)

	reqb, err := sps.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if err := sps.sx.Tx(reqb); err != nil {
		return nil, err
	}

	for {
		rspb, err := sps.sx.Rx()
		if err != nil {
			return nil, err
		}

		// Now wait for newtmgr response.
		if sps.d.Dispatch(rspb) {
			select {
			case err := <-nl.ErrChan:
				return nil, err
			case rsp := <-nl.RspChan:
				return rsp, nil
			}
		}
	}
}

func (sps *SerialPlainSesn) GetResourceOnce(resType sesn.ResourceType,
	uri string, opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	return 0, nil, fmt.Errorf("SerialPlainSesn.GetResourceOnce() unsupported")
}

func (sps *SerialPlainSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte,
	opt sesn.TxOptions) (coap.COAPCode, error) {

	return 0, fmt.Errorf("SerialPlainSesn.PutResourceOnce() unsupported")
}
