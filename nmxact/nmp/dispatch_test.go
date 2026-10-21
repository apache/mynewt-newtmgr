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

package nmp

import (
	"errors"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func encodeTestEchoRsp(t *testing.T, op uint8, seq uint8,
	payload string) []byte {

	t.Helper()

	rsp := NewEchoRsp()
	rsp.Payload = payload

	msg := &NmpMsg{
		Hdr: NmpHdr{
			Op:    op,
			Group: NMP_GROUP_DEFAULT,
			Id:    NMP_ID_DEF_ECHO,
			Seq:   seq,
		},
		Body: rsp,
	}

	b, err := EncodeNmpPlain(msg)
	if err != nil {
		t.Fatalf("EncodeNmpPlain failed: %v", err)
	}
	return b
}

func recvRsp(t *testing.T, nl *Listener) NmpRsp {
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

func recvErr(t *testing.T, nl *Listener) error {
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

func assertIdle(t *testing.T, nl *Listener) {
	t.Helper()

	if n := len(nl.RspChan); n != 0 {
		t.Errorf("listener has %d unexpected responses", n)
	}
	if n := len(nl.ErrChan); n != 0 {
		t.Errorf("listener has %d unexpected errors", n)
	}
}

func TestDispatcherAddListenerDuplicate(t *testing.T) {
	d := NewDispatcher(0)

	if _, err := d.AddListener(7); err != nil {
		t.Fatalf("AddListener failed: %v", err)
	}
	if _, err := d.AddListener(7); err == nil {
		t.Fatalf("duplicate AddListener succeeded")
	}
	if _, err := d.AddListener(8); err != nil {
		t.Fatalf("AddListener for other seq failed: %v", err)
	}
}

func TestDispatcherDispatchBySeq(t *testing.T) {
	d := NewDispatcher(0)

	nl1, err := d.AddListener(1)
	if err != nil {
		t.Fatal(err)
	}
	nl2, err := d.AddListener(2)
	if err != nil {
		t.Fatal(err)
	}

	if !d.Dispatch(encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 2, "two")) {
		t.Fatalf("Dispatch returned false for known seq")
	}

	rsp := recvRsp(t, nl2)
	echo, ok := rsp.(*EchoRsp)
	if !ok {
		t.Fatalf("got %T, want *EchoRsp", rsp)
	}
	if echo.Payload != "two" {
		t.Errorf("payload = %q, want %q", echo.Payload, "two")
	}
	if echo.Hdr().Seq != 2 {
		t.Errorf("seq = %d, want 2", echo.Hdr().Seq)
	}
	assertIdle(t, nl1)
}

func TestDispatcherDispatchNotDelivered(t *testing.T) {
	good := func(t *testing.T) []byte {
		return encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 5, "x")
	}

	tests := []struct {
		name string
		data func(t *testing.T) []byte
	}{
		{"unknown seq", func(t *testing.T) []byte {
			return encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 6, "x")
		}},
		{"request op", func(t *testing.T) []byte {
			return encodeTestEchoRsp(t, NMP_OP_WRITE, 5, "x")
		}},
		{"unknown group", func(t *testing.T) []byte {
			b := good(t)
			b[4] = 0x7f
			return b
		}},
		{"bad cbor body", func(t *testing.T) []byte {
			hdr := NmpHdr{
				Op:  NMP_OP_WRITE_RSP,
				Len: 1,
				Seq: 5,
			}
			return append(hdr.Bytes(), 0xff)
		}},
		{"too long", func(t *testing.T) []byte {
			return append(good(t), 0x00)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher(0)
			nl, err := d.AddListener(5)
			if err != nil {
				t.Fatal(err)
			}

			if d.Dispatch(tt.data(t)) {
				t.Errorf("Dispatch returned true")
			}
			assertIdle(t, nl)
		})
	}
}

func TestDispatcherDispatchFragmented(t *testing.T) {
	d := NewDispatcher(0)
	nl, err := d.AddListener(9)
	if err != nil {
		t.Fatal(err)
	}

	b := encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 9, "fragmented payload")

	// Split inside the header and inside the body.
	frags := [][]byte{b[:3], b[3 : NMP_HDR_SIZE+2], b[NMP_HDR_SIZE+2:]}
	for i, frag := range frags[:len(frags)-1] {
		if d.Dispatch(frag) {
			t.Fatalf("Dispatch returned true for partial fragment %d", i)
		}
	}
	if !d.Dispatch(frags[len(frags)-1]) {
		t.Fatalf("Dispatch returned false for final fragment")
	}

	echo := recvRsp(t, nl).(*EchoRsp)
	if echo.Payload != "fragmented payload" {
		t.Errorf("payload = %q", echo.Payload)
	}
}

func TestDispatcherErrorOne(t *testing.T) {
	d := NewDispatcher(0)
	nl1, err := d.AddListener(1)
	if err != nil {
		t.Fatal(err)
	}
	nl2, err := d.AddListener(2)
	if err != nil {
		t.Fatal(err)
	}

	want := errors.New("boom")
	if err := d.ErrorOne(1, want); err != nil {
		t.Fatalf("ErrorOne failed: %v", err)
	}
	if got := recvErr(t, nl1); got != want {
		t.Errorf("got error %v, want %v", got, want)
	}
	assertIdle(t, nl2)

	if err := d.ErrorOne(3, want); err == nil {
		t.Errorf("ErrorOne for unknown seq succeeded")
	}
}

func TestDispatcherErrorAll(t *testing.T) {
	d := NewDispatcher(0)

	var nls []*Listener
	for seq := uint8(10); seq < 13; seq++ {
		nl, err := d.AddListener(seq)
		if err != nil {
			t.Fatal(err)
		}
		nls = append(nls, nl)
	}

	want := errors.New("all")
	d.ErrorAll(want)

	for i, nl := range nls {
		if got := recvErr(t, nl); got != want {
			t.Errorf("listener %d: got error %v, want %v", i, got, want)
		}
	}
}

func TestDispatcherRemoveListener(t *testing.T) {
	d := NewDispatcher(0)
	nl, err := d.AddListener(4)
	if err != nil {
		t.Fatal(err)
	}

	if got := d.RemoveListener(4); got != nl {
		t.Fatalf("RemoveListener returned %p, want %p", got, nl)
	}
	if got := d.RemoveListener(4); got != nil {
		t.Fatalf("second RemoveListener returned %p, want nil", got)
	}

	// Channels are closed on removal.
	if _, ok := <-nl.RspChan; ok {
		t.Errorf("RspChan not closed")
	}
	if _, ok := <-nl.ErrChan; ok {
		t.Errorf("ErrChan not closed")
	}

	if d.Dispatch(encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 4, "x")) {
		t.Errorf("Dispatch to removed listener returned true")
	}
	if err := d.ErrorOne(4, errors.New("x")); err == nil {
		t.Errorf("ErrorOne to removed listener succeeded")
	}

	// The seq can be reused.
	nl2, err := d.AddListener(4)
	if err != nil {
		t.Fatalf("re-adding listener failed: %v", err)
	}
	if !d.Dispatch(encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 4, "again")) {
		t.Fatalf("Dispatch to re-added listener returned false")
	}
	if echo := recvRsp(t, nl2).(*EchoRsp); echo.Payload != "again" {
		t.Errorf("payload = %q", echo.Payload)
	}
}

func TestListenerAfterTimeout(t *testing.T) {
	nl := NewListener()

	select {
	case _, ok := <-nl.AfterTimeout(time.Millisecond):
		if !ok {
			t.Fatalf("timeout channel closed")
		}
	case <-time.After(testTimeout):
		t.Fatalf("timeout did not fire")
	}

	nl.Close()
}

func TestListenerCloseStopsTimer(t *testing.T) {
	nl := NewListener()
	nl.AfterTimeout(time.Hour)
	nl.Close()

	if nl.timer.Stop() {
		t.Errorf("timer still active after Close")
	}
}

// A device may send more than one response with the same sequence number
// (e.g. a retransmission).  The RX path must not block on the extra
// responses, otherwise the listener can never be removed.
func TestDispatcherDuplicateRspDoesNotBlock(t *testing.T) {
	d := NewDispatcher(0)
	nl, err := d.AddListener(3)
	if err != nil {
		t.Fatal(err)
	}

	b := encodeTestEchoRsp(t, NMP_OP_WRITE_RSP, 3, "dup")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			d.Dispatch(b)
		}
	}()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatalf("Dispatch of duplicate response blocked")
	}

	if echo := recvRsp(t, nl).(*EchoRsp); echo.Payload != "dup" {
		t.Errorf("payload = %q", echo.Payload)
	}

	removed := make(chan struct{})
	go func() {
		d.RemoveListener(3)
		close(removed)
	}()

	select {
	case <-removed:
	case <-time.After(testTimeout):
		t.Fatalf("RemoveListener blocked")
	}
}

// The timer callback may run concurrently with Close.  It must neither
// race with Close nor panic with "send on closed channel".  The race is
// timing dependent, so try many times.
func TestListenerTimeoutRacesClose(t *testing.T) {
	for i := 0; i < 2000; i++ {
		nl := NewListener()
		nl.AfterTimeout(0)
		nl.Close()
	}
}
