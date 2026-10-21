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
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
)

func sign(i int) int {
	switch {
	case i < 0:
		return -1
	case i > 0:
		return 1
	default:
		return 0
	}
}

func TestCompareMsgCriteria(t *testing.T) {
	tests := []struct {
		a, b MsgCriteria
		want int
	}{
		{MsgCriteria{}, MsgCriteria{}, 0},
		{MsgCriteria{Path: "a"}, MsgCriteria{Path: "a"}, 0},
		{MsgCriteria{Path: "a"}, MsgCriteria{Path: "b"}, -1},
		{MsgCriteria{Path: "b"}, MsgCriteria{Path: "a"}, 1},
		{MsgCriteria{Path: ""}, MsgCriteria{Path: "a"}, -1},

		// Path is the primary key.
		{MsgCriteria{Path: "a", Token: []byte{9}},
			MsgCriteria{Path: "b", Token: []byte{1}}, -1},

		{MsgCriteria{Token: []byte{1}}, MsgCriteria{Token: []byte{1}}, 0},
		{MsgCriteria{Token: []byte{1}}, MsgCriteria{Token: []byte{2}}, -1},
		{MsgCriteria{Token: []byte{2}}, MsgCriteria{Token: []byte{1}}, 1},
		{MsgCriteria{Token: []byte{1}}, MsgCriteria{Token: []byte{1, 0}},
			-1},

		// A nil token is a wildcard and sorts before any concrete token,
		// including an empty one.
		{MsgCriteria{Token: nil}, MsgCriteria{Token: []byte{1}}, -1},
		{MsgCriteria{Token: []byte{1}}, MsgCriteria{Token: nil}, 1},
		{MsgCriteria{Token: nil}, MsgCriteria{Token: []byte{}}, -1},
		{MsgCriteria{Token: []byte{}}, MsgCriteria{Token: []byte{}}, 0},
	}

	for _, tt := range tests {
		got := sign(CompareMsgCriteria(tt.a, tt.b))
		if got != tt.want {
			t.Errorf("Compare(%s, %s) = %d, want %d",
				tt.a.String(), tt.b.String(), got, tt.want)
		}
	}
}

func TestMatchMsgCriteria(t *testing.T) {
	tests := []struct {
		name     string
		listen   MsgCriteria
		msg      MsgCriteria
		expected bool
	}{
		{"wildcard", MsgCriteria{},
			MsgCriteria{Path: "x", Token: []byte{1}}, true},
		{"path match", MsgCriteria{Path: "x"},
			MsgCriteria{Path: "x", Token: []byte{1}}, true},
		{"path mismatch", MsgCriteria{Path: "x"},
			MsgCriteria{Path: "y"}, false},
		{"token match", MsgCriteria{Token: []byte{1, 2}},
			MsgCriteria{Path: "x", Token: []byte{1, 2}}, true},
		{"token mismatch", MsgCriteria{Token: []byte{1, 2}},
			MsgCriteria{Token: []byte{1, 3}}, false},
		{"token missing", MsgCriteria{Token: []byte{1}},
			MsgCriteria{}, false},
		{"path and token", MsgCriteria{Path: "x", Token: []byte{1}},
			MsgCriteria{Path: "x", Token: []byte{1}}, true},
		{"path ok token bad", MsgCriteria{Path: "x", Token: []byte{1}},
			MsgCriteria{Path: "x", Token: []byte{2}}, false},

		// A 0-length token and a nil token are equal ("nmxact: treat
		// 0-length and nil tokens as equal").
		{"empty listener nil msg", MsgCriteria{Token: []byte{}},
			MsgCriteria{Token: nil}, true},
		{"empty listener empty msg", MsgCriteria{Token: []byte{}},
			MsgCriteria{Token: []byte{}}, true},
		{"empty listener token msg", MsgCriteria{Token: []byte{}},
			MsgCriteria{Token: []byte{1}}, false},
	}

	for _, tt := range tests {
		if got := MatchMsgCriteria(tt.listen, tt.msg); got != tt.expected {
			t.Errorf("%s: Match(%s, %s) = %v, want %v", tt.name,
				tt.listen.String(), tt.msg.String(), got, tt.expected)
		}
	}
}

func TestSortListeners(t *testing.T) {
	wild := NewListener(MsgCriteria{})
	pathA := NewListener(MsgCriteria{Path: "a"})
	pathATok1 := NewListener(MsgCriteria{Path: "a", Token: []byte{1}})
	pathATok2 := NewListener(MsgCriteria{Path: "a", Token: []byte{2}})
	pathB := NewListener(MsgCriteria{Path: "b"})
	tok1 := NewListener(MsgCriteria{Token: []byte{1}})

	ls := []*Listener{wild, pathATok1, tok1, pathB, pathA, pathATok2}
	SortListeners(ls)

	want := []*Listener{pathB, pathATok2, pathATok1, pathA, tok1, wild}
	for i := range want {
		if ls[i] != want[i] {
			for j, l := range ls {
				t.Logf("[%d] %s", j, l.Criteria.String())
			}
			t.Fatalf("index %d: got %s, want %s", i,
				ls[i].Criteria.String(), want[i].Criteria.String())
		}
	}

	// After sorting, the first matching listener is the most specific one.
	msg := MsgCriteria{Path: "a", Token: []byte{1}}
	for _, l := range ls {
		if MatchMsgCriteria(l.Criteria, msg) {
			if l != pathATok1 {
				t.Fatalf("first match = %s, want %s",
					l.Criteria.String(), pathATok1.Criteria.String())
			}
			break
		}
	}
}

func TestMsgCriteriaString(t *testing.T) {
	tests := []struct {
		mc   MsgCriteria
		want string
	}{
		{MsgCriteria{}, "token=nil path=nil"},
		{MsgCriteria{Token: []byte{0xab, 0x01}, Path: "omgr"},
			"token=ab01 path=omgr"},
		{MsgCriteria{Token: []byte{}}, "token= path=nil"},
	}

	for _, tt := range tests {
		if got := tt.mc.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestCriteriaFromMsg(t *testing.T) {
	m := coap.NewDgramMessage(coap.MessageParams{
		Code:  coap.Content,
		Token: []byte{5, 6},
	})
	m.SetPathString("/a/b")

	mc := CriteriaFromMsg(m)
	if mc.Path != "a/b" || string(mc.Token) != "\x05\x06" {
		t.Fatalf("unexpected criteria %s", mc.String())
	}
}

func TestListenerClose(t *testing.T) {
	l := NewListener(MsgCriteria{})
	tmo := l.AfterTimeout(time.Hour)
	l.Close()

	if _, ok := <-l.RspChan; ok {
		t.Fatalf("RspChan not closed")
	}
	if _, ok := <-l.ErrChan; ok {
		t.Fatalf("ErrChan not closed")
	}
	if _, ok := <-tmo; ok {
		t.Fatalf("timeout chan not closed")
	}
}
