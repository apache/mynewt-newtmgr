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

package omp

import (
	"errors"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

const testTimeout = 5 * time.Second

// Builds a CoAP datagram carrying an OMP echo response, as a device would
// send it.
func encodeTestOmpRsp(t *testing.T, code coap.COAPCode, seq uint8,
	payload string) []byte {

	t.Helper()

	hdr := nmp.NmpHdr{
		Op:    nmp.NMP_OP_WRITE_RSP,
		Group: nmp.NMP_GROUP_DEFAULT,
		Id:    nmp.NMP_ID_DEF_ECHO,
		Seq:   seq,
	}
	body := map[string]interface{}{
		"_h": hdr.Bytes(),
		"r":  payload,
		"rc": 0,
	}

	var cbor []byte
	enc := codec.NewEncoderBytes(&cbor, new(codec.CborHandle))
	if err := enc.Encode(body); err != nil {
		t.Fatalf("cbor encode failed: %v", err)
	}

	m := coap.NewDgramMessage(coap.MessageParams{
		Type:    coap.Acknowledgement,
		Code:    code,
		Token:   nmxutil.SeqToToken(seq),
		Payload: cbor,
	})
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("CoAP encode failed: %v", err)
	}
	return b
}

func recvRsp(t *testing.T, nl *nmp.Listener) nmp.NmpRsp {
	t.Helper()

	select {
	case rsp := <-nl.RspChan:
		return rsp
	case err := <-nl.ErrChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for response")
	}
	return nil
}

func recvErr(t *testing.T, nl *nmp.Listener) error {
	t.Helper()

	select {
	case err := <-nl.ErrChan:
		return err
	case rsp := <-nl.RspChan:
		t.Fatalf("unexpected response: %+v", rsp)
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for error")
	}
	return nil
}

func newTestDispatcher(t *testing.T, f nmcoap.RxMsgFilter) *Dispatcher {
	t.Helper()

	d, err := NewDispatcher(f, false, 0)
	if err != nil {
		t.Fatalf("NewDispatcher failed: %v", err)
	}
	t.Cleanup(func() { stopWithTimeout(t, d) })
	return d
}

func stopWithTimeout(t *testing.T, d *Dispatcher) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		d.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatalf("Dispatcher.Stop blocked")
	}
}

func TestDispatcherNmpRsp(t *testing.T) {
	d := newTestDispatcher(t, nil)

	nl, err := d.AddNmpListener(42)
	if err != nil {
		t.Fatalf("AddNmpListener failed: %v", err)
	}
	other, err := d.AddNmpListener(43)
	if err != nil {
		t.Fatalf("AddNmpListener failed: %v", err)
	}

	if !d.Dispatch(encodeTestOmpRsp(t, coap.Content, 42, "hello")) {
		t.Fatalf("Dispatch returned false")
	}

	rsp := recvRsp(t, nl)
	echo, ok := rsp.(*nmp.EchoRsp)
	if !ok {
		t.Fatalf("got %T, want *nmp.EchoRsp", rsp)
	}
	if echo.Payload != "hello" {
		t.Errorf("payload = %q, want %q", echo.Payload, "hello")
	}
	if echo.Hdr().Seq != 42 {
		t.Errorf("seq = %d, want 42", echo.Hdr().Seq)
	}

	if len(other.RspChan) != 0 || len(other.ErrChan) != 0 {
		t.Errorf("unrelated listener received data")
	}

	if d.Dispatch(encodeTestOmpRsp(t, coap.Content, 44, "x")) {
		t.Errorf("Dispatch for unknown seq returned true")
	}
}

func TestDispatcherAddNmpListenerDuplicate(t *testing.T) {
	d := newTestDispatcher(t, nil)

	if _, err := d.AddNmpListener(1); err != nil {
		t.Fatalf("AddNmpListener failed: %v", err)
	}
	if _, err := d.AddNmpListener(1); err == nil {
		t.Fatalf("duplicate AddNmpListener succeeded")
	}
}

func TestDispatcherErrorCode(t *testing.T) {
	d := newTestDispatcher(t, nil)

	nl, err := d.AddNmpListener(5)
	if err != nil {
		t.Fatal(err)
	}

	d.Dispatch(encodeTestOmpRsp(t, coap.NotFound, 5, "x"))
	if err := recvErr(t, nl); err == nil {
		t.Errorf("got nil error for 4.04 response")
	}
}

func TestDispatcherRxFilter(t *testing.T) {
	filterErr := errors.New("filter failed")
	calls := make(chan struct{}, 4)
	fail := false

	f := nmcoap.RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		calls <- struct{}{}
		if fail {
			return nil, filterErr
		}
		return m, nil
	})

	d := newTestDispatcher(t, f)
	if d.RxFilter() == nil {
		t.Fatalf("RxFilter returned nil")
	}

	nl, err := d.AddNmpListener(6)
	if err != nil {
		t.Fatal(err)
	}

	d.Dispatch(encodeTestOmpRsp(t, coap.Content, 6, "filtered"))
	if echo := recvRsp(t, nl).(*nmp.EchoRsp); echo.Payload != "filtered" {
		t.Errorf("payload = %q", echo.Payload)
	}
	if len(calls) != 1 {
		t.Errorf("filter called %d times, want 1", len(calls))
	}

	// The goroutine reading "fail" is synchronized through the CoAP
	// listener channel written by Dispatch below.
	fail = true
	d.Dispatch(encodeTestOmpRsp(t, coap.Content, 6, "x"))
	if err := recvErr(t, nl); err != filterErr {
		t.Errorf("got error %v, want %v", err, filterErr)
	}
}

func TestDispatcherIgnoresRequests(t *testing.T) {
	d := newTestDispatcher(t, nil)

	nl, err := d.AddNmpListener(7)
	if err != nil {
		t.Fatal(err)
	}

	// A request echoed back by the device is ignored; the response that
	// follows is delivered.
	d.Dispatch(encodeTestOmpRsp(t, coap.PUT, 7, "request"))
	d.Dispatch(encodeTestOmpRsp(t, coap.Content, 7, "response"))

	if echo := recvRsp(t, nl).(*nmp.EchoRsp); echo.Payload != "response" {
		t.Errorf("payload = %q, want %q", echo.Payload, "response")
	}
}

func TestDispatcherErrorAll(t *testing.T) {
	d := newTestDispatcher(t, nil)

	var nls []*nmp.Listener
	for seq := uint8(20); seq < 23; seq++ {
		nl, err := d.AddNmpListener(seq)
		if err != nil {
			t.Fatal(err)
		}
		nls = append(nls, nl)
	}

	want := errors.New("all")
	d.ErrorAll(want)

	for i, nl := range nls {
		if got := recvErr(t, nl); got != want {
			t.Errorf("listener %d: got %v, want %v", i, got, want)
		}
	}
}

func TestDispatcherErrorOneNmp(t *testing.T) {
	d := newTestDispatcher(t, nil)

	nl, err := d.AddNmpListener(8)
	if err != nil {
		t.Fatal(err)
	}

	want := errors.New("one")
	if err := d.ErrorOneNmp(8, want); err != nil {
		t.Fatalf("ErrorOneNmp failed: %v", err)
	}
	if got := recvErr(t, nl); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if err := d.ErrorOneNmp(9, want); err == nil {
		t.Errorf("ErrorOneNmp for unknown seq succeeded")
	}
}

func TestDispatcherRemoveNmpListener(t *testing.T) {
	d := newTestDispatcher(t, nil)

	nl, err := d.AddNmpListener(10)
	if err != nil {
		t.Fatal(err)
	}

	if got := d.RemoveNmpListener(10); got != nl {
		t.Fatalf("RemoveNmpListener returned %p, want %p", got, nl)
	}
	if got := d.RemoveNmpListener(10); got != nil {
		t.Fatalf("second RemoveNmpListener returned %p, want nil", got)
	}
	if err := d.ErrorOneNmp(10, errors.New("x")); err == nil {
		t.Errorf("ErrorOneNmp after removal succeeded")
	}
}

func TestDispatcherStop(t *testing.T) {
	d, err := NewDispatcher(nil, false, 0)
	if err != nil {
		t.Fatal(err)
	}

	for seq := uint8(30); seq < 33; seq++ {
		if _, err := d.AddNmpListener(seq); err != nil {
			t.Fatal(err)
		}
	}

	stopWithTimeout(t, d)

	// Stop is idempotent.
	stopWithTimeout(t, d)
}

func TestDispatcherCoapListener(t *testing.T) {
	d := newTestDispatcher(t, nil)

	mc := nmcoap.MsgCriteria{Token: []byte{0xee}}
	l, err := d.AddCoapListener(mc)
	if err != nil {
		t.Fatal(err)
	}

	m := coap.NewDgramMessage(coap.MessageParams{
		Type:    coap.Acknowledgement,
		Code:    coap.Content,
		Token:   []byte{0xee},
		Payload: []byte("raw"),
	})
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !d.Dispatch(b) {
		t.Fatalf("Dispatch returned false")
	}
	select {
	case got := <-l.RspChan:
		if string(got.Payload()) != "raw" {
			t.Errorf("payload = %q", got.Payload())
		}
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for CoAP message")
	}

	if got := d.RemoveCoapListener(mc); got != l {
		t.Errorf("RemoveCoapListener returned %p, want %p", got, l)
	}
}
