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
	"bytes"
	"testing"
)

// buildPkt builds a raw NMP packet with the given body.
func buildPkt(seq uint8, body []byte) []byte {
	hdr := NmpHdr{
		Op:    NMP_OP_READ_RSP,
		Group: NMP_GROUP_DEFAULT,
		Id:    NMP_ID_DEF_ECHO,
		Seq:   seq,
		Len:   uint16(len(body)),
	}
	return append(hdr.Bytes(), body...)
}

func TestReassemblerExact(t *testing.T) {
	pkt := buildPkt(1, []byte{1, 2, 3, 4})

	r := NewReassembler()
	got := r.RxFrag(pkt)
	if !bytes.Equal(got, pkt) {
		t.Fatalf("RxFrag() = % x, want % x", got, pkt)
	}
}

func TestReassemblerEmptyBody(t *testing.T) {
	pkt := buildPkt(1, nil)

	r := NewReassembler()
	got := r.RxFrag(pkt)
	if !bytes.Equal(got, pkt) {
		t.Fatalf("RxFrag() = % x, want % x", got, pkt)
	}
}

func TestReassemblerByteByByte(t *testing.T) {
	pkt := buildPkt(7, []byte("0123456789abcdef"))

	r := NewReassembler()
	for i := 0; i < len(pkt)-1; i++ {
		if got := r.RxFrag(pkt[i : i+1]); got != nil {
			t.Fatalf("byte %d: unexpected packet % x", i, got)
		}
	}

	got := r.RxFrag(pkt[len(pkt)-1:])
	if !bytes.Equal(got, pkt) {
		t.Fatalf("RxFrag() = % x, want % x", got, pkt)
	}
}

func TestReassemblerSplitHeader(t *testing.T) {
	pkt := buildPkt(3, []byte{9, 8, 7, 6, 5})

	for split := 1; split < len(pkt); split++ {
		r := NewReassembler()
		if got := r.RxFrag(pkt[:split]); got != nil {
			t.Fatalf("split=%d: unexpected packet after first frag", split)
		}
		got := r.RxFrag(pkt[split:])
		if !bytes.Equal(got, pkt) {
			t.Fatalf("split=%d: RxFrag() = % x, want % x", split, got, pkt)
		}
	}
}

func TestReassemblerOversize(t *testing.T) {
	pkt := buildPkt(1, []byte{1, 2})
	bad := append(append([]byte{}, pkt...), 0xff)

	r := NewReassembler()
	if got := r.RxFrag(bad); got != nil {
		t.Fatalf("oversize packet must be discarded, got % x", got)
	}

	// The reassembler must have been reset; a following valid packet is
	// received correctly.
	got := r.RxFrag(pkt)
	if !bytes.Equal(got, pkt) {
		t.Fatalf("RxFrag() = % x, want % x", got, pkt)
	}
}

func TestReassemblerBackToBack(t *testing.T) {
	pkt1 := buildPkt(1, []byte{1, 2, 3})
	pkt2 := buildPkt(2, []byte{4, 5, 6, 7})

	r := NewReassembler()
	if got := r.RxFrag(pkt1); !bytes.Equal(got, pkt1) {
		t.Fatalf("pkt1: RxFrag() = % x, want % x", got, pkt1)
	}
	if got := r.RxFrag(pkt2[:5]); got != nil {
		t.Fatalf("pkt2: unexpected packet % x", got)
	}
	if got := r.RxFrag(pkt2[5:]); !bytes.Equal(got, pkt2) {
		t.Fatalf("pkt2: RxFrag() = % x, want % x", got, pkt2)
	}
}
