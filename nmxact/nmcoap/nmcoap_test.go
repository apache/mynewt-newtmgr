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
	"testing"

	"github.com/runtimeco/go-coap"
)

func TestParseOp(t *testing.T) {
	tests := []struct {
		in   string
		want coap.COAPCode
	}{
		{"get", coap.GET},
		{"GET", coap.GET},
		{"Put", coap.PUT},
		{"post", coap.POST},
		{"DeLeTe", coap.DELETE},
	}

	for _, tt := range tests {
		got, err := ParseOp(tt.in)
		if err != nil {
			t.Errorf("ParseOp(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseOp(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	for _, bad := range []string{"", "fetch", "gett", " get"} {
		if _, err := ParseOp(bad); err == nil {
			t.Errorf("ParseOp(%q): expected error", bad)
		}
	}
}

func TestObserveCodeSpec(t *testing.T) {
	tests := []struct {
		o    ObserveCode
		want int
	}{
		{OBSERVE_NONE, -1},
		{OBSERVE_START, 0},
		{OBSERVE_STOP, 1},
		{ObserveCode(42), -1},
	}

	for _, tt := range tests {
		if got := tt.o.Spec(); got != tt.want {
			t.Errorf("ObserveCode(%d).Spec() = %d, want %d",
				int(tt.o), got, tt.want)
		}
	}
}

func TestNextMessageId(t *testing.T) {
	a := NextMessageId()
	b := NextMessageId()
	if b != a+1 {
		t.Fatalf("NextMessageId() = %d after %d, want %d", b, a, a+1)
	}
}

func TestValidateToken(t *testing.T) {
	for n := 0; n <= 8; n++ {
		if err := validateToken(make([]byte, n)); err != nil {
			t.Errorf("len=%d: unexpected error: %v", n, err)
		}
	}
	if err := validateToken(make([]byte, 9)); err == nil {
		t.Errorf("len=9: expected error")
	}
}

// encodeDecode encodes m and parses it back with the matching transport
// parser.
func encodeDecode(t *testing.T, isTcp bool, m coap.Message) coap.Message {
	t.Helper()

	b, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}

	if isTcp {
		tm, rest, err := coap.PullTcp(b)
		if err != nil || tm == nil || len(rest) != 0 {
			t.Fatalf("PullTcp() = %v, % x, %v", tm, rest, err)
		}
		return tm
	}

	dm, err := coap.ParseDgramMessage(b)
	if err != nil {
		t.Fatalf("ParseDgramMessage(): %v", err)
	}
	return dm
}

func TestCreateMsg(t *testing.T) {
	tests := []struct {
		name    string
		mp      MsgParams
		path    string
		query   []string
		observe interface{}
	}{
		{
			name: "plain",
			mp: MsgParams{Code: coap.GET, Uri: "/a/b",
				Token: []byte{1, 2}},
			path: "a/b",
		},
		{
			name: "query",
			mp: MsgParams{Code: coap.PUT, Uri: "res?x=1&y=2",
				Token: []byte{3}, Payload: []byte("pl")},
			path:  "res",
			query: []string{"x=1&y=2"},
		},
		{
			name: "question mark in query",
			mp: MsgParams{Code: coap.POST, Uri: "res?a?b",
				Token: []byte{4}},
			path:  "res",
			query: []string{"a?b"},
		},
		{
			name: "observe start",
			mp: MsgParams{Code: coap.GET, Uri: "obs",
				Observe: OBSERVE_START, Token: []byte{5}},
			path:    "obs",
			observe: uint32(0),
		},
		{
			name: "observe stop",
			mp: MsgParams{Code: coap.GET, Uri: "obs",
				Observe: OBSERVE_STOP, Token: []byte{6}},
			path:    "obs",
			observe: uint32(1),
		},
	}

	for _, tt := range tests {
		for _, isTcp := range []bool{false, true} {
			m, err := CreateMsg(isTcp, tt.mp)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}

			_, gotTcp := m.(*coap.TcpMessage)
			if gotTcp != isTcp {
				t.Fatalf("%s: got %T, isTcp=%v", tt.name, m, isTcp)
			}

			m = encodeDecode(t, isTcp, m)

			if m.Code() != tt.mp.Code {
				t.Errorf("%s: code = %v, want %v", tt.name, m.Code(),
					tt.mp.Code)
			}
			if !bytes.Equal(m.Token(), tt.mp.Token) {
				t.Errorf("%s: token = % x, want % x", tt.name, m.Token(),
					tt.mp.Token)
			}
			if !bytes.Equal(m.Payload(), tt.mp.Payload) {
				t.Errorf("%s: payload = % x, want % x", tt.name,
					m.Payload(), tt.mp.Payload)
			}
			if m.PathString() != tt.path {
				t.Errorf("%s: path = %q, want %q", tt.name,
					m.PathString(), tt.path)
			}

			var q []string
			for _, o := range m.Options(coap.URIQuery) {
				q = append(q, o.(string))
			}
			if len(q) != len(tt.query) ||
				(len(q) > 0 && q[0] != tt.query[0]) {

				t.Errorf("%s: query = %v, want %v", tt.name, q, tt.query)
			}

			if obs := m.Option(coap.Observe); obs != tt.observe {
				t.Errorf("%s: observe = %#v, want %#v", tt.name, obs,
					tt.observe)
			}
		}
	}
}

func TestCreateMsgDefaultToken(t *testing.T) {
	m1, err := CreateMsg(false, MsgParams{Code: coap.GET, Uri: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m2, err := CreateMsg(false, MsgParams{Code: coap.GET, Uri: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(m1.Token()) != 1 || len(m2.Token()) != 1 {
		t.Fatalf("tokens = % x, % x; want 1 byte each",
			m1.Token(), m2.Token())
	}
	if m2.Token()[0] != m1.Token()[0]+1 {
		t.Fatalf("tokens = % x, % x; want consecutive",
			m1.Token(), m2.Token())
	}
}

func TestRxFilterFunc(t *testing.T) {
	in := coap.NewDgramMessage(coap.MessageParams{Code: coap.GET})
	out := coap.NewDgramMessage(coap.MessageParams{Code: coap.PUT})

	f := RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		if m != in {
			t.Fatalf("filter got unexpected message")
		}
		return out, nil
	})

	var rf RxMsgFilter = f
	got, err := rf.Filter(in)
	if err != nil || got != out {
		t.Fatalf("Filter() = %v, %v", got, err)
	}
}
