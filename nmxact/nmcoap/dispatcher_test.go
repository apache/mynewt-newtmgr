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

package nmcoap

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
)

const testTimeout = 5 * time.Second

func encodeTestMsg(t *testing.T, isTcp bool, token []byte,
	uri string, payload string) []byte {

	t.Helper()

	// Built by hand rather than with CreateMsg, which cannot create a
	// message without a URI.
	m := buildMessage(isTcp, coap.MessageParams{
		Type:    coap.Acknowledgement,
		Code:    coap.Content,
		Token:   token,
		Payload: []byte(payload),
	})
	if uri != "" {
		m.SetPathString(uri)
	}

	b, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	return b
}

func recvMsg(t *testing.T, l *Listener) coap.Message {
	t.Helper()

	select {
	case m := <-l.RspChan:
		return m
	case err := <-l.ErrChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for CoAP message")
	}
	return nil
}

func recvErr(t *testing.T, l *Listener) error {
	t.Helper()

	select {
	case err := <-l.ErrChan:
		return err
	case m := <-l.RspChan:
		t.Fatalf("unexpected message: %v", m)
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for error")
	}
	return nil
}

func assertIdle(t *testing.T, name string, l *Listener) {
	t.Helper()

	if n := len(l.RspChan); n != 0 {
		t.Errorf("%s: %d unexpected messages", name, n)
	}
	if n := len(l.ErrChan); n != 0 {
		t.Errorf("%s: %d unexpected errors", name, n)
	}
}

func TestDispatcherAddListenerDuplicate(t *testing.T) {
	d := NewDispatcher(false, 0)

	mc := MsgCriteria{Token: []byte{1, 2}, Path: "res"}
	if _, err := d.AddListener(mc); err != nil {
		t.Fatalf("AddListener failed: %v", err)
	}

	// Same criteria, different token slice instance.
	dup := MsgCriteria{Token: []byte{1, 2}, Path: "res"}
	if _, err := d.AddListener(dup); err == nil {
		t.Fatalf("duplicate AddListener succeeded")
	}

	others := []MsgCriteria{
		{Token: []byte{1, 3}, Path: "res"},
		{Token: nil, Path: "res"},
		{Token: []byte{1, 2}, Path: ""},
		{Token: nil, Path: ""},
	}
	for _, o := range others {
		if _, err := d.AddListener(o); err != nil {
			t.Errorf("AddListener(%s) failed: %v", o.String(), err)
		}
	}
}

func TestDispatcherMatch(t *testing.T) {
	tok1 := []byte{0xa1}
	tok2 := []byte{0xa2}
	tok3 := []byte{0xa3}

	tests := []struct {
		name  string
		token []byte
		uri   string
		want  string // name of the listener expected to receive; "" none
	}{
		{"token only", tok1, "", "tok1"},
		{"token with other path", tok1, "other", "tok1"},
		{"path and token most specific", tok2, "res", "res+tok2"},
		{"path with unknown token", tok3, "res", "res"},
		{"path wins over token-only", tok1, "res", "res"},
		{"no match", tok3, "other", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher(false, 0)

			lners := map[string]*Listener{}
			add := func(name string, mc MsgCriteria) {
				l, err := d.AddListener(mc)
				if err != nil {
					t.Fatalf("AddListener(%s) failed: %v", name, err)
				}
				lners[name] = l
			}
			add("tok1", MsgCriteria{Token: tok1})
			add("res", MsgCriteria{Path: "res"})
			add("res+tok2", MsgCriteria{Token: tok2, Path: "res"})

			ok := d.Dispatch(encodeTestMsg(t, false, tt.token, tt.uri, "p"))
			if ok != (tt.want != "") {
				t.Fatalf("Dispatch returned %v", ok)
			}

			for name, l := range lners {
				if name != tt.want {
					assertIdle(t, name, l)
					continue
				}

				m := recvMsg(t, l)
				if !bytes.Equal(m.Token(), tt.token) {
					t.Errorf("token = %x, want %x", m.Token(), tt.token)
				}
				if string(m.Payload()) != "p" {
					t.Errorf("payload = %q", m.Payload())
				}
			}
		})
	}
}

func TestDispatcherDispatchGarbage(t *testing.T) {
	d := NewDispatcher(false, 0)
	l, err := d.AddListener(MsgCriteria{})
	if err != nil {
		t.Fatal(err)
	}

	if d.Dispatch([]byte{0x00, 0x01}) {
		t.Errorf("Dispatch of garbage returned true")
	}
	assertIdle(t, "catch-all", l)
}

func TestDispatcherDispatchTcp(t *testing.T) {
	d := NewDispatcher(true, 0)
	tok := []byte{0x10, 0x20}
	l, err := d.AddListener(MsgCriteria{Token: tok})
	if err != nil {
		t.Fatal(err)
	}

	b := encodeTestMsg(t, true, tok, "", "tcp payload")
	if d.Dispatch(b[:1]) {
		t.Fatalf("Dispatch of partial TCP message returned true")
	}
	if !d.Dispatch(b[1:]) {
		t.Fatalf("Dispatch of completed TCP message returned false")
	}
	if m := recvMsg(t, l); string(m.Payload()) != "tcp payload" {
		t.Errorf("payload = %q", m.Payload())
	}
}

func TestDispatcherRemoveListener(t *testing.T) {
	d := NewDispatcher(false, 0)
	tok := []byte{7}
	mc := MsgCriteria{Token: tok, Path: "x"}

	l, err := d.AddListener(mc)
	if err != nil {
		t.Fatal(err)
	}

	if got := d.RemoveListener(MsgCriteria{Token: []byte{7}, Path: "x"}); got != l {
		t.Fatalf("RemoveListener returned %p, want %p", got, l)
	}
	if got := d.RemoveListener(mc); got != nil {
		t.Fatalf("second RemoveListener returned %p, want nil", got)
	}

	if _, ok := <-l.RspChan; ok {
		t.Errorf("RspChan not closed")
	}
	if _, ok := <-l.ErrChan; ok {
		t.Errorf("ErrChan not closed")
	}

	if d.Dispatch(encodeTestMsg(t, false, tok, "x", "p")) {
		t.Errorf("Dispatch to removed listener returned true")
	}

	if _, err := d.AddListener(mc); err != nil {
		t.Errorf("re-adding listener failed: %v", err)
	}
}

func TestDispatcherErrorOne(t *testing.T) {
	d := NewDispatcher(false, 0)
	l1, err := d.AddListener(MsgCriteria{Token: []byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	l2, err := d.AddListener(MsgCriteria{Token: []byte{2}})
	if err != nil {
		t.Fatal(err)
	}

	want := errors.New("one")
	if err := d.ErrorOne(MsgCriteria{Token: []byte{1}}, want); err != nil {
		t.Fatalf("ErrorOne failed: %v", err)
	}
	if got := recvErr(t, l1); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	assertIdle(t, "l2", l2)

	if err := d.ErrorOne(MsgCriteria{Token: []byte{3}}, want); err == nil {
		t.Errorf("ErrorOne without matching listener succeeded")
	}
}

func TestDispatcherErrorAll(t *testing.T) {
	d := NewDispatcher(false, 0)

	var lners []*Listener
	for _, mc := range []MsgCriteria{
		{Token: []byte{1}},
		{Path: "a"},
		{Token: []byte{2}, Path: "a"},
	} {
		l, err := d.AddListener(mc)
		if err != nil {
			t.Fatal(err)
		}
		lners = append(lners, l)
	}

	want := errors.New("all")
	d.ErrorAll(want)

	for i, l := range lners {
		if got := recvErr(t, l); got != want {
			t.Errorf("listener %d: got %v, want %v", i, got, want)
		}
	}
}

func TestListenerAfterTimeout(t *testing.T) {
	l := NewListener(MsgCriteria{})

	select {
	case _, ok := <-l.AfterTimeout(time.Millisecond):
		if !ok {
			t.Fatalf("timeout channel closed")
		}
	case <-time.After(testTimeout):
		t.Fatalf("timeout did not fire")
	}

	l.Close()
}

func TestListenerCloseBeforeTimeout(t *testing.T) {
	l := NewListener(MsgCriteria{})
	l.AfterTimeout(time.Hour)
	l.Close()

	if l.timer.Stop() {
		t.Errorf("timer still active after Close")
	}
}

// Regression test for 827bef2: the timer callback may run concurrently
// with Close.  It must neither race with Close nor panic with "send on
// closed channel".  The race is timing dependent, so try many times.
func TestListenerTimeoutRacesClose(t *testing.T) {
	for i := 0; i < 2000; i++ {
		l := NewListener(MsgCriteria{})
		l.AfterTimeout(0)
		l.Close()
	}
}
