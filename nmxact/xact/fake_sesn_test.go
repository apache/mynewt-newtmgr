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

package xact

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

// testTimeout bounds every blocking operation in the tests so that a hang
// turns into a test failure instead of a stuck test binary.
const testTimeout = 5 * time.Second

// fakeReq is a management request as seen by the fake session.
type fakeReq struct {
	Hdr nmp.NmpHdr

	// Request body as passed by the command (e.g., *nmp.ImageUploadReq).
	Body interface{}

	// Plain NMP encoding of the request (header + CBOR body).
	Plain []byte

	// CBOR body decoded into a generic map; useful to check which fields
	// are actually present on the wire.
	Fields map[string]interface{}

	// Length of the request encoded for the session's management protocol
	// (what a real transport would transmit).
	EncLen int
}

// fakeHandler emulates the device.  It returns the response to a request or
// an error which is reported the same way a real session would report it.
type fakeHandler func(req fakeReq) (nmp.NmpRsp, error)

// fakeCoapHandler emulates a CoAP server.  A nil message means no reply.
type fakeCoapHandler func(m coap.Message) coap.Message

// fakeSesn is an in-memory, scripted implementation of sesn.Sesn.
type fakeSesn struct {
	mtx sync.Mutex

	mtu       int
	proto     sesn.MgmtProto
	isTcp     bool
	txFilter  nmcoap.TxMsgFilter
	rxFilter  nmcoap.RxMsgFilter
	isOpen    bool
	openErr   error
	openCount int

	// If set, requests whose encoding exceeds the MTU are rejected like
	// a real datagram transport does.
	strictMtu bool

	handler fakeHandler
	reqs    []fakeReq

	abortErr  error
	abortSeqs []uint8

	coapHandler fakeCoapHandler
	coapTx      []coap.Message
	listeners   map[string]*nmcoap.Listener
}

func newFakeSesn(h fakeHandler) *fakeSesn {
	return &fakeSesn{
		mtu:       512,
		proto:     sesn.MGMT_PROTO_NMP,
		isOpen:    true,
		strictMtu: true,
		handler:   h,
		listeners: map[string]*nmcoap.Listener{},
	}
}

func (s *fakeSesn) Requests() []fakeReq {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return append([]fakeReq(nil), s.reqs...)
}

func (s *fakeSesn) Open() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.openCount++
	if s.openErr != nil {
		return s.openErr
	}
	s.isOpen = true
	return nil
}

func (s *fakeSesn) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.isOpen = false
	return nil
}

func (s *fakeSesn) IsOpen() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.isOpen
}

func (s *fakeSesn) setOpen(open bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.isOpen = open
}

func (s *fakeSesn) MtuIn() int  { return s.mtu }
func (s *fakeSesn) MtuOut() int { return s.mtu }

func (s *fakeSesn) MgmtProto() sesn.MgmtProto { return s.proto }
func (s *fakeSesn) CoapIsTcp() bool           { return s.isTcp }

func (s *fakeSesn) AbortRx(nmpSeq uint8) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.abortSeqs = append(s.abortSeqs, nmpSeq)
	return s.abortErr
}

func (s *fakeSesn) RxAccept() (sesn.Sesn, *sesn.SesnCfg, error) {
	return nil, nil, fmt.Errorf("RxAccept not supported by fake session")
}

func (s *fakeSesn) RxCoap(opt sesn.TxOptions) (coap.Message, error) {
	return nil, fmt.Errorf("RxCoap not supported by fake session")
}

func decodeCborMap(b []byte) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	dec := codec.NewDecoderBytes(b, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *fakeSesn) TxRxMgmt(m *nmp.NmpMsg,
	timeout time.Duration) (nmp.NmpRsp, error) {

	enc, err := mgmt.EncodeMgmt(s, m)
	if err != nil {
		return nil, err
	}
	plain, err := nmp.EncodeNmpPlain(m)
	if err != nil {
		return nil, err
	}
	fields, err := decodeCborMap(plain[nmp.NMP_HDR_SIZE:])
	if err != nil {
		return nil, err
	}

	req := fakeReq{
		Hdr:    m.Hdr,
		Body:   m.Body,
		Plain:  plain,
		Fields: fields,
		EncLen: len(enc),
	}

	s.mtx.Lock()
	open := s.isOpen
	s.reqs = append(s.reqs, req)
	h := s.handler
	strict := s.strictMtu
	s.mtx.Unlock()

	if !open {
		return nil, fmt.Errorf("fake session not open")
	}
	if strict && len(enc) > s.mtu {
		return nil, fmt.Errorf("Request too big")
	}
	if h == nil {
		return nil, fmt.Errorf("no handler")
	}

	return h(req)
}

// TxRxMgmtAsync behaves like the serial and naked BLE sessions: the
// transaction is performed synchronously and the result is pushed to the
// provided channels before returning.
func (s *fakeSesn) TxRxMgmtAsync(m *nmp.NmpMsg, timeout time.Duration,
	ch chan nmp.NmpRsp, errc chan error) error {

	rsp, err := s.TxRxMgmt(m, timeout)
	if err != nil {
		errc <- err
	} else {
		ch <- rsp
	}
	return nil
}

func (s *fakeSesn) ListenCoap(
	mc nmcoap.MsgCriteria) (*nmcoap.Listener, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	key := mc.String()
	if _, ok := s.listeners[key]; ok {
		return nil, fmt.Errorf("duplicate listener: %s", key)
	}
	l := nmcoap.NewListener(mc)
	s.listeners[key] = l
	return l, nil
}

// StopListenCoap only forgets the listener.  It does not close it to keep
// the fake free of timer related races in the CoAP listener.
func (s *fakeSesn) StopListenCoap(mc nmcoap.MsgCriteria) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	delete(s.listeners, mc.String())
}

func (s *fakeSesn) TxCoap(m coap.Message) error {
	s.mtx.Lock()
	s.coapTx = append(s.coapTx, m)
	h := s.coapHandler
	var l *nmcoap.Listener
	for _, cand := range s.listeners {
		mc := nmcoap.CriteriaFromMsg(m)
		if nmcoap.MatchMsgCriteria(cand.Criteria, mc) {
			l = cand
		}
	}
	s.mtx.Unlock()

	if h == nil {
		return nil
	}
	rsp := h(m)
	if rsp != nil && l != nil {
		l.RspChan <- rsp
	}
	return nil
}

func (s *fakeSesn) Filters() (nmcoap.TxMsgFilter, nmcoap.RxMsgFilter) {
	return s.txFilter, s.rxFilter
}

func (s *fakeSesn) SetFilters(txFilter nmcoap.TxMsgFilter,
	rxFilter nmcoap.RxMsgFilter) {

	s.txFilter = txFilter
	s.rxFilter = rxFilter
}

// rspFor builds a response to req by encoding fields as CBOR and decoding
// them with the regular NMP response decoder.  This yields the response
// type a real session would produce.
func rspFor(req fakeReq, fields map[string]interface{}) (nmp.NmpRsp, error) {
	body := []byte{}
	enc := codec.NewEncoderBytes(&body, new(codec.CborHandle))
	if err := enc.Encode(fields); err != nil {
		return nil, err
	}

	hdr := req.Hdr
	hdr.Op++
	hdr.Len = uint16(len(body))
	return nmp.DecodeRspBody(&hdr, body)
}

// runCmd runs c with a timeout guard.
func runCmd(t *testing.T, c Cmd, s sesn.Sesn) (Result, error) {
	t.Helper()

	type result struct {
		res Result
		err error
	}
	done := make(chan result, 1)
	go func() {
		res, err := c.Run(s)
		done <- result{res, err}
	}()

	select {
	case r := <-done:
		return r.res, r.err
	case <-time.After(testTimeout):
		t.Fatalf("%T.Run() did not complete within %v", c, testTimeout)
		return nil, nil
	}
}
