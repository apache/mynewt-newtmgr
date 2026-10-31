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
	log "github.com/sirupsen/logrus"
)

type Reassembler struct {
	cur []byte
}

func NewReassembler() *Reassembler {
	return &Reassembler{}
}

// Indicates whether the buffer starts with a plausible response header.
// Requests are never received; data from the middle of a packet is rejected
// this way with high probability.
func rspHdrValid(b []byte) bool {
	op := b[0] & 0x07
	if op != NMP_OP_READ_RSP && op != NMP_OP_WRITE_RSP {
		return false
	}

	// The two bits above the opcode hold the SMP version; the others are
	// reserved.
	return b[0]&0xe0 == 0
}

func (r *Reassembler) RxFrag(frag []byte) []byte {
	pending := len(r.cur) > 0

	r.cur = append(r.cur, frag...)
	pkt, ok := r.check()
	if !ok && pending {
		// The pending packet is incomplete (e.g., its last fragment was
		// lost) and this fragment does not continue it.  A packet always
		// starts at the beginning of a fragment, so try this fragment on
		// its own instead of staying out of sync.
		log.Debugf("dropping incomplete nmp packet")
		r.cur = append([]byte(nil), frag...)
		pkt, ok = r.check()
	}
	if !ok {
		r.cur = nil
	}

	return pkt
}

// Returns the complete packet, if any; false if the buffered data is not a
// valid packet.
func (r *Reassembler) check() ([]byte, bool) {
	hdr, err := DecodeNmpHdr(r.cur)
	if err != nil {
		// Incomplete header.
		return nil, len(r.cur) == 0 || rspHdrValid(r.cur)
	}

	if !rspHdrValid(r.cur) {
		log.Debugf("received invalid nmp header: % x", r.cur[:NMP_HDR_SIZE])
		return nil, false
	}

	actualLen := len(r.cur) - NMP_HDR_SIZE
	if actualLen > int(hdr.Len) {
		// More data than expected.  Discard packet.
		log.Debugf("received invalid nmp packet; hdr.len=%d actualLen=%d",
			hdr.Len, actualLen)
		return nil, false
	}

	if actualLen < int(hdr.Len) {
		// More fragments to come.
		return nil, true
	}

	// Packet complete
	pkt := r.cur
	r.cur = nil
	return pkt, true
}
