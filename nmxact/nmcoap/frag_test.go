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

// Invalid CoAP-TCP message: 1-byte body containing a reserved option
// delta (15) that is not the payload marker.
var garbageTcp = []byte{0x10, 0x00, 0xf0}

func tcpMsg(t *testing.T, token []byte, path string,
	payload []byte) []byte {

	t.Helper()

	m := coap.NewTcpMessage(coap.MessageParams{
		Type:    coap.Confirmable,
		Code:    coap.Content,
		Token:   token,
		Payload: payload,
	})
	if path != "" {
		m.SetPathString(path)
	}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to encode CoAP: %v", err)
	}
	return b
}

func checkTcpMsg(t *testing.T, m *coap.TcpMessage, token []byte,
	payload []byte) {

	t.Helper()

	if m == nil {
		t.Fatalf("expected a message, got nil")
	}
	if !bytes.Equal(m.Token(), token) {
		t.Fatalf("token = % x, want % x", m.Token(), token)
	}
	if !bytes.Equal(m.Payload(), payload) {
		t.Fatalf("payload = % x, want % x", m.Payload(), payload)
	}
}

func TestReassemblerSingle(t *testing.T) {
	pl := []byte("hello world")
	b := tcpMsg(t, []byte{1, 2}, "omgr", pl)

	r := NewReassembler()
	m := r.RxFrag(b)
	checkTcpMsg(t, m, []byte{1, 2}, pl)
	if m.PathString() != "omgr" {
		t.Fatalf("path = %q, want omgr", m.PathString())
	}
}

func TestReassemblerFragmented(t *testing.T) {
	// Large enough to use the extended length field.
	pl := bytes.Repeat([]byte{0x5a}, 300)
	b := tcpMsg(t, []byte{7}, "omgr", pl)

	for _, fragSz := range []int{1, 2, 7, 20, len(b) - 1} {
		r := NewReassembler()
		var m *coap.TcpMessage
		for off := 0; off < len(b); off += fragSz {
			end := off + fragSz
			if end > len(b) {
				end = len(b)
			}

			m = r.RxFrag(b[off:end])
			if end < len(b) && m != nil {
				t.Fatalf("fragSz=%d: message returned early at off %d",
					fragSz, off)
			}
		}
		checkTcpMsg(t, m, []byte{7}, pl)
	}
}

func TestReassemblerGarbageRecovery(t *testing.T) {
	r := NewReassembler()
	if m := r.RxFrag(garbageTcp); m != nil {
		t.Fatalf("garbage produced a message: %v", m)
	}

	// The invalid data must have been dropped; the next valid message is
	// received normally.
	pl := []byte{1, 2, 3}
	m := r.RxFrag(tcpMsg(t, []byte{9}, "", pl))
	checkTcpMsg(t, m, []byte{9}, pl)
}

func TestReceiverUdp(t *testing.T) {
	rx := NewReceiver(false)

	m := coap.NewDgramMessage(coap.MessageParams{
		Type:      coap.Confirmable,
		Code:      coap.Changed,
		MessageID: 0x1234,
		Token:     []byte{4},
		Payload:   []byte("x"),
	})
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := rx.Rx(b)
	if got == nil {
		t.Fatalf("expected message")
	}
	if got.Code() != coap.Changed || got.MessageID() != 0x1234 ||
		!bytes.Equal(got.Token(), []byte{4}) {

		t.Fatalf("unexpected message %+v", got)
	}

	if got := rx.Rx([]byte{0x00}); got != nil {
		t.Fatalf("garbage produced a message: %v", got)
	}
}

func TestReceiverTcp(t *testing.T) {
	rx := NewReceiver(true)
	b := tcpMsg(t, []byte{3}, "a/b", []byte("p"))

	if got := rx.Rx(b[:2]); got != nil {
		t.Fatalf("partial message returned: %v", got)
	}
	got := rx.Rx(b[2:])
	if got == nil || got.PathString() != "a/b" {
		t.Fatalf("unexpected message %v", got)
	}
}

// Two messages received in a single buffer must both be returned, one per
// call.  Previously the second message was silently dropped.
func TestReassemblerBackToBack(t *testing.T) {
	b1 := tcpMsg(t, []byte{1}, "", []byte("first"))
	b2 := tcpMsg(t, []byte{2}, "", []byte("second"))
	b3 := tcpMsg(t, []byte{3}, "", []byte("third"))

	r := NewReassembler()
	buf := append(append([]byte{}, b1...), b2...)

	checkTcpMsg(t, r.RxFrag(buf), []byte{1}, []byte("first"))
	checkTcpMsg(t, r.RxFrag(nil), []byte{2}, []byte("second"))
	if m := r.RxFrag(nil); m != nil {
		t.Fatalf("unexpected third message: %v", m)
	}

	// First message complete plus the start of the next one.
	buf = append(append([]byte{}, b1...), b3[:3]...)
	checkTcpMsg(t, r.RxFrag(buf), []byte{1}, []byte("first"))
	checkTcpMsg(t, r.RxFrag(b3[3:]), []byte{3}, []byte("third"))
	if m := r.RxFrag(nil); m != nil {
		t.Fatalf("unexpected extra message: %v", m)
	}
}

func TestReceiverRxBuffered(t *testing.T) {
	udp := NewReceiver(false)
	if m := udp.RxBuffered(); m != nil {
		t.Fatalf("UDP receiver returned buffered message %v", m)
	}

	tcp := NewReceiver(true)
	if m := tcp.RxBuffered(); m != nil {
		t.Fatalf("empty TCP receiver returned message %v", m)
	}

	b1 := tcpMsg(t, []byte{1}, "", nil)
	b2 := tcpMsg(t, []byte{2}, "", nil)
	if m := tcp.Rx(append(append([]byte{}, b1...), b2...)); m == nil {
		t.Fatalf("expected first message")
	}
	m := tcp.RxBuffered()
	if m == nil || m.Token()[0] != 2 {
		t.Fatalf("expected second message, got %v", m)
	}
	if m := tcp.RxBuffered(); m != nil {
		t.Fatalf("unexpected message %v", m)
	}
}
