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
	log "github.com/Sirupsen/logrus"
)

type Reassembler struct {
	cur []byte
}

func NewReassembler() *Reassembler {
	return &Reassembler{}
}

func (r *Reassembler) RxFrag(frag []byte) []byte {
	r.cur = append(r.cur, frag...)

	hdr, err := DecodeNmpHdr(r.cur)
	if err != nil {
		// Incomplete header.
		return nil
	}

	actualLen := len(r.cur) - NMP_HDR_SIZE
	if actualLen > int(hdr.Len) {
		// More data than expected.  Discard packet.
		log.Debugf("received invalid nmp packet; hdr.len=%d actualLen=%d",
			hdr.Len, actualLen)
		r.cur = nil
		return nil
	}

	if actualLen < int(hdr.Len) {
		// More fragments to come.
		return nil
	}

	// Packet complete
	pkt := r.cur
	r.cur = nil
	return pkt
}
