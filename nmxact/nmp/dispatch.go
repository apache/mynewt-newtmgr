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
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type Listener struct {
	RspChan chan NmpRsp
	ErrChan chan error
	tmoChan chan time.Time
	timer   *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		RspChan: make(chan NmpRsp, 1),
		ErrChan: make(chan error, 1),
		tmoChan: make(chan time.Time, 1),
	}
}

func (nl *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		nl.tmoChan <- time.Now()
	}
	nl.timer = time.AfterFunc(tmo, fn)
	return nl.tmoChan
}

func (nl *Listener) Close() {
	if nl.timer != nil {
		nl.timer.Stop()
	}

	close(nl.RspChan)
	close(nl.ErrChan)
	close(nl.tmoChan)
}

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	seqListenerMap map[uint8]*Listener
	reassembler    *Reassembler
	logDepth       int
	mtx            sync.Mutex
}

func NewDispatcher(logDepth int) *Dispatcher {
	return &Dispatcher{
		seqListenerMap: map[uint8]*Listener{},
		reassembler:    NewReassembler(),
		logDepth:       logDepth + 2,
	}
}

func (d *Dispatcher) AddListener(seq uint8) (*Listener, error) {
	nmxutil.LogAddNmpListener(d.logDepth, seq)

	d.mtx.Lock()
	defer d.mtx.Unlock()

	if _, ok := d.seqListenerMap[seq]; ok {
		return nil, fmt.Errorf("Duplicate NMP listener; seq=%d", seq)
	}

	nl := NewListener()
	d.seqListenerMap[seq] = nl
	return nl, nil
}

func (d *Dispatcher) RemoveListener(seq uint8) *Listener {
	nmxutil.LogRemoveNmpListener(d.logDepth, seq)

	d.mtx.Lock()
	defer d.mtx.Unlock()

	nl := d.seqListenerMap[seq]
	if nl != nil {
		nl.Close()
		delete(d.seqListenerMap, seq)
	}
	return nl
}

func decodeRsp(pkt []byte) (NmpRsp, error) {
	hdr, err := DecodeNmpHdr(pkt)
	if err != nil {
		return nil, err
	}

	// Ignore incoming non-responses.  This is necessary for devices that echo
	// received requests over serial.
	if hdr.Op != NMP_OP_READ_RSP && hdr.Op != NMP_OP_WRITE_RSP {
		return nil, nil
	}

	body := pkt[NMP_HDR_SIZE:]
	return DecodeRspBody(hdr, body)
}

// Returns true if the response was dispatched.
func (d *Dispatcher) DispatchRsp(r NmpRsp) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	log.Debugf("Received nmp rsp: %+v", r)

	nl := d.seqListenerMap[r.Hdr().Seq]
	if nl == nil {
		log.Debugf("No listener for incoming NMP message")
		return false
	}

	nl.RspChan <- r

	return true
}

// Returns true if the response was dispatched.
func (d *Dispatcher) Dispatch(data []byte) bool {
	pkt := d.reassembler.RxFrag(data)
	if pkt == nil {
		return false
	}

	rsp, err := decodeRsp(pkt)
	if err != nil {
		log.Debugf("Failure decoding NMP rsp: %s\npacket=\n%s", err.Error(),
			hex.Dump(data))
		return false
	}

	if rsp == nil {
		// Packet wasn't a response.
		return false
	}

	return d.DispatchRsp(rsp)
}

func (d *Dispatcher) ErrorOne(seq uint8, err error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	nl := d.seqListenerMap[seq]
	if nl == nil {
		return fmt.Errorf("No NMP listener for seq %d", seq)
	}

	nl.ErrChan <- err

	return nil
}

func (d *Dispatcher) ErrorAll(err error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	for _, nl := range d.seqListenerMap {
		nl.ErrChan <- err
	}
}
