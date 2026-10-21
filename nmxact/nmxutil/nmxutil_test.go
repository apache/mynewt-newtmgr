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

package nmxutil

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

func TestFragment(t *testing.T) {
	data := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name string
		b    []byte
		mtu  int
		want [][]byte
	}{
		{"empty", []byte{}, 4, [][]byte{}},
		{"nil", nil, 4, [][]byte{}},
		{"mtu 1", data[:3], 1, [][]byte{{0}, {1}, {2}}},
		{"uneven", data, 4,
			[][]byte{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9}}},
		{"even", data, 5,
			[][]byte{{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}}},
		{"exact", data, 10, [][]byte{data}},
		{"larger mtu", data, 100, [][]byte{data}},

		// No usable MTU: the buffer is not fragmented.
		{"mtu 0", data, 0, [][]byte{data}},
		{"negative mtu", data, -5, [][]byte{data}},
		{"mtu 0 empty", []byte{}, 0, [][]byte{}},
	}

	for _, tt := range tests {
		got := Fragment(tt.b, tt.mtu)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Fragment() = %v, want %v", tt.name, got, tt.want)
		}

		// Fragments always reassemble into the original buffer.
		if j := bytes.Join(got, nil); !bytes.Equal(j, tt.b) {
			t.Errorf("%s: joined fragments = %v, want %v", tt.name, j, tt.b)
		}
	}
}

func TestNextNmpSeqWrap(t *testing.T) {
	seqMutex.Lock()
	nextNmpSeq = 0xfe
	nmpSeqBeenRead = true
	seqMutex.Unlock()

	for _, want := range []uint8{0xfe, 0xff, 0x00, 0x01} {
		if got := NextNmpSeq(); got != want {
			t.Fatalf("NextNmpSeq() = %#x, want %#x", got, want)
		}
	}
}

func TestNextTokenWrap(t *testing.T) {
	seqMutex.Lock()
	nextOicSeq = 0xff
	oicSeqBeenRead = true
	seqMutex.Unlock()

	for _, want := range []uint8{0xff, 0x00, 0x01} {
		got := NextToken()
		if !bytes.Equal(got, []byte{want}) {
			t.Fatalf("NextToken() = % x, want %02x", got, want)
		}
	}
}

func TestSeqToToken(t *testing.T) {
	for _, seq := range []uint8{0, 1, 0x7f, 0xff} {
		if got := SeqToToken(seq); !bytes.Equal(got, []byte{seq}) {
			t.Errorf("SeqToToken(%d) = % x", seq, got)
		}
	}
}

func TestGetNextId(t *testing.T) {
	a := GetNextId()
	b := GetNextId()
	if b != a+1 {
		t.Fatalf("GetNextId() = %d after %d", b, a)
	}
}

func TestCborMapRoundTrip(t *testing.T) {
	in := map[string]interface{}{
		"s": "str",
		"b": []byte{1, 2},
		"t": true,
	}

	b, err := EncodeCborMap(in)
	if err != nil {
		t.Fatalf("EncodeCborMap(): %v", err)
	}

	out, err := DecodeCborMap(b)
	if err != nil {
		t.Fatalf("DecodeCborMap(): %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip = %#v, want %#v", out, in)
	}
}

func TestCborRoundTrip(t *testing.T) {
	in := []interface{}{"a", uint64(5), int64(-3)}

	b, err := EncodeCbor(in)
	if err != nil {
		t.Fatalf("EncodeCbor(): %v", err)
	}

	out, err := DecodeCbor(b)
	if err != nil {
		t.Fatalf("DecodeCbor(): %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip = %#v, want %#v", out, in)
	}
}

func TestDecodeCborInvalid(t *testing.T) {
	bad := [][]byte{
		// Truncated map.
		{0xa1, 0x61},
		// Reserved additional info.
		{0x1c},
	}

	for _, b := range bad {
		if _, err := DecodeCbor(b); err == nil {
			t.Errorf("DecodeCbor(% x): expected error", b)
		}
		if _, err := DecodeCborMap(b); err == nil {
			t.Errorf("DecodeCborMap(% x): expected error", b)
		}
	}

	// Valid CBOR, but not a map.
	if _, err := DecodeCborMap([]byte{0x01}); err == nil {
		t.Errorf("DecodeCborMap(int): expected error")
	}
}

func TestStopAndDrainTimer(t *testing.T) {
	// Timer still running.
	tm := time.NewTimer(time.Hour)
	StopAndDrainTimer(tm)

	// Timer already expired but its value not received; must not block.
	tm = time.NewTimer(time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		StopAndDrainTimer(tm)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("StopAndDrainTimer blocked")
	}
}
