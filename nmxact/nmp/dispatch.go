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
	"encoding/hex"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
)

type NmpListener struct {
	RspChan chan NmpRsp
	ErrChan chan error
}

func NewNmpListener() *NmpListener {
	return &NmpListener{
		RspChan: make(chan NmpRsp, 1),
		ErrChan: make(chan error, 1),
	}
}

type NmpDispatcher struct {
	seqListenerMap map[uint8]*NmpListener
	reassembler    *Reassembler
	mutex          sync.Mutex
}

func NewNmpDispatcher() *NmpDispatcher {
	return &NmpDispatcher{
		seqListenerMap: map[uint8]*NmpListener{},
		reassembler:    NewReassembler(),
	}
}

func (nd *NmpDispatcher) AddListener(seq uint8, rl *NmpListener) error {
	nd.mutex.Lock()
	defer nd.mutex.Unlock()

	if _, ok := nd.seqListenerMap[seq]; ok {
		return fmt.Errorf("Duplicate NMP listener; seq=%d", seq)
	}

	nd.seqListenerMap[seq] = rl
	return nil
}

func (nd *NmpDispatcher) removeListenerNoLock(seq uint8) *NmpListener {
	rl := nd.seqListenerMap[seq]
	if rl != nil {
		delete(nd.seqListenerMap, seq)
	}
	return rl
}

func (nd *NmpDispatcher) RemoveListener(seq uint8) *NmpListener {
	nd.mutex.Lock()
	defer nd.mutex.Unlock()

	return nd.removeListenerNoLock(seq)
}

func (nd *NmpDispatcher) FakeRxError(seq uint8, err error) error {
	nd.mutex.Lock()
	defer nd.mutex.Unlock()

	rl := nd.seqListenerMap[seq]
	if rl == nil {
		return fmt.Errorf("No NMP listener for seq %d", seq)
	}

	rl.ErrChan <- err
	nd.removeListenerNoLock(seq)

	return nil
}

// Returns true if the response was dispatched.
func (nd *NmpDispatcher) DispatchRsp(r NmpRsp) bool {
	log.Debugf("Received nmp rsp: %+v", r)

	rl := nd.seqListenerMap[r.Hdr().Seq]
	if rl == nil {
		return false
	}

	rl.RspChan <- r
	nd.removeListenerNoLock(r.Hdr().Seq)

	return true
}

// Returns true if the response was dispatched.
func (nd *NmpDispatcher) Dispatch(data []byte) bool {
	nd.mutex.Lock()
	defer nd.mutex.Unlock()

	pkt := nd.reassembler.RxFrag(data)
	if pkt == nil {
		return false
	}

	rsp, err := DecodeRsp(pkt)
	if err != nil {
		log.Printf("Failure decoding NMP rsp: %s\npacket=\n%s", err.Error(),
			hex.Dump(data))
		return false
	}

	if rsp == nil {
		// Packet wasn't a response.
		return false
	}

	return nd.DispatchRsp(rsp)
}
