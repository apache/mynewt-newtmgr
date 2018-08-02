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

package mgmt

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type TxFn func(req []byte) error

type Transceiver struct {
	// Only for plain NMP; nil for OMP transceivers.
	nd *nmp.Dispatcher

	// Used for OMP and CoAP resource requests.
	od *omp.Dispatcher

	isTcp bool
	proto sesn.MgmtProto
	wg    sync.WaitGroup
}

func NewTransceiver(txFilterCb, rxFilterCb nmcoap.MsgFilter, isTcp bool, mgmtProto sesn.MgmtProto, logDepth int) (
	*Transceiver, error) {

	t := &Transceiver{
		isTcp: isTcp,
		proto: mgmtProto,
	}

	if mgmtProto == sesn.MGMT_PROTO_NMP {
		t.nd = nmp.NewDispatcher(logDepth)
	}

	od, err := omp.NewDispatcher(txFilterCb, rxFilterCb, isTcp, logDepth)
	if err != nil {
		return nil, err
	}
	t.od = od

	return t, nil
}

func (t *Transceiver) txPlain(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	nl, err := t.nd.AddListener(req.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer t.nd.RemoveListener(req.Hdr.Seq)

	b, err := nmp.EncodeNmpPlain(req)
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx NMP request: %s", hex.Dump(b))
	if t.isTcp == false && len(b) > mtu {
		return nil, fmt.Errorf("Request too big")
	}
	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case _, ok := <-nl.AfterTimeout(timeout):
			if ok {
				return nil, nmxutil.NewRspTimeoutError("NMP timeout")
			}
		}
	}
}

func (t *Transceiver) txOmp(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	nl, err := t.od.AddNmpListener(req.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer t.od.RemoveNmpListener(req.Hdr.Seq)

	var b []byte
	txFilterCb, _ := t.od.Filters()
	if t.isTcp {
		b, err = omp.EncodeOmpTcp(txFilterCb, req)
	} else {
		b, err = omp.EncodeOmpDgram(txFilterCb, req)
	}
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx OMP request: %s", hex.Dump(b))

	if t.isTcp == false && len(b) > mtu {
		return nil, fmt.Errorf("Request too big")
	}
	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case _, ok := <-nl.AfterTimeout(timeout):
			if ok {
				return nil, nmxutil.NewRspTimeoutError("NMP timeout")
			}
		}
	}
}

func (t *Transceiver) TxNmp(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	if t.nd != nil {
		return t.txPlain(txCb, req, mtu, timeout)
	} else {
		return t.txOmp(txCb, req, mtu, timeout)
	}
}

func (t *Transceiver) TxOic(txCb TxFn, req coap.Message, mtu int,
	timeout time.Duration) (coap.Message, error) {

	b, err := nmcoap.Encode(req)
	if err != nil {
		return nil, err
	}

	var rspExpected bool
	switch req.Type() {
	case coap.Confirmable:
		rspExpected = true
	case coap.NonConfirmable:
		rspExpected = req.Code() == coap.GET
	default:
		rspExpected = false
	}
	if t.proto == sesn.MGMT_PROTO_COAP_SERVER {
		rspExpected = false
	}

	var ol *nmcoap.Listener
	if rspExpected {
		ol, err = t.od.AddOicListener(req.Token())
		if err != nil {
			return nil, err
		}
		defer t.od.RemoveOicListener(req.Token())
	}

	log.Debugf("Tx OIC request: %s", hex.Dump(b))
	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	if !rspExpected {
		return nil, nil
	}

	for {
		select {
		case err := <-ol.ErrChan:
			return nil, err
		case rsp := <-ol.RspChan:
			return rsp, nil
		case _, ok := <-ol.AfterTimeout(timeout):
			if ok {
				return nil, nmxutil.NewRspTimeoutError("OIC timeout")
			}
		}
	}
}

func (t *Transceiver) TxOicObserve(txCb TxFn, req coap.Message, mtu int,
	timeout time.Duration, NotifyCb sesn.GetNotifyCb, stopsignal chan int) (coap.Message, error) {

	b, err := nmcoap.Encode(req)
	if err != nil {
		return nil, err
	}

	var ol *nmcoap.Listener

	ol, err = t.od.AddOicListener(req.Token())
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx OIC request: %s", hex.Dump(b))
	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			t.od.RemoveOicListener(req.Token())
			return nil, err
		}
	}

	var rsp coap.Message

	first := make(chan int)
	iter := 0

	go func() {
		defer t.od.RemoveOicListener(req.Token())

		for {
			select {
			case err = <-ol.ErrChan:
				log.Debugf("Error: %s", err)
				first <- 2
				return
			case rsp = <-ol.RspChan:
				if iter == 0 {
					first <- 1
					iter = 1
				} else {
					NotifyCb(req.PathString(), rsp.Code(), rsp.Payload(), rsp.Token())
				}

			case _, ok := <-ol.AfterTimeout(timeout):
				if ok && iter == 0 {
					err = nmxutil.NewRspTimeoutError("OIC timeout")
					first <- 2
					return
				} else {
					log.Debugf("Timeout")
				}

			case <-stopsignal:
				/* Observing stopped by user */
				return
			}
		}
	}()

	for {
		select {
		case a := <-first:
			if a == 1 {
				return rsp, nil
			} else {
				return nil, err
			}
		}
	}
}

func (t *Transceiver) DispatchNmpRsp(data []byte) {
	if t.nd != nil {
		t.nd.Dispatch(data)
	} else {
		log.Debugf("Rx OIC response: %s", hex.Dump(data))
		t.od.Dispatch(data)
	}
}

func (t *Transceiver) DispatchCoap(data []byte) {
	t.od.Dispatch(data)
}

func (t *Transceiver) ProcessCoapReq(data []byte) (coap.Message, error) {
	return t.od.ProcessCoapReq(data)
}

func (t *Transceiver) ErrorOne(seq uint8, err error) {
	if t.nd != nil {
		t.nd.ErrorOne(seq, err)
	} else {
		t.od.ErrorOneNmp(seq, err)
	}
}

func (t *Transceiver) ErrorAll(err error) {
	if t.nd != nil {
		t.nd.ErrorAll(err)
	}
	t.od.ErrorAll(err)
}

func (t *Transceiver) AbortRx(seq uint8) {
	t.ErrorOne(seq, fmt.Errorf("rx aborted"))
}

func (t *Transceiver) Stop() {
	t.od.Stop()
}

func (t *Transceiver) MgmtProto() sesn.MgmtProto {
	return t.proto
}
