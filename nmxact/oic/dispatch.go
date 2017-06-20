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

package oic

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type Token struct {
	Len  int
	Data [8]byte
}

func NewToken(rawToken []byte) (Token, error) {
	ot := Token{}

	if len(rawToken) > 8 {
		return ot, fmt.Errorf("Invalid CoAP token: too long (%d bytes)",
			len(rawToken))
	}

	ot.Len = len(rawToken)
	copy(ot.Data[:], rawToken)

	return ot, nil
}

type Listener struct {
	RspChan chan *coap.Message
	ErrChan chan error
	tmoChan chan time.Time
	timer   *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		RspChan: make(chan *coap.Message, 1),
		ErrChan: make(chan error, 1),
		tmoChan: make(chan time.Time, 1),
	}
}

func (ol *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		ol.tmoChan <- time.Now()
	}
	ol.timer = time.AfterFunc(tmo, fn)
	return ol.tmoChan
}

type Dispatcher struct {
	tokenListenerMap map[Token]*Listener
	reassembler      *Reassembler
	logDepth         int
	mtx              sync.Mutex
}

func NewDispatcher(isTcp bool, logDepth int) *Dispatcher {
	d := &Dispatcher{
		tokenListenerMap: map[Token]*Listener{},
		logDepth:         logDepth + 2,
	}

	if isTcp {
		d.reassembler = NewReassembler()
	}

	return d
}

func (d *Dispatcher) AddListener(token []byte) (*Listener, error) {
	nmxutil.LogAddOicListener(d.logDepth, token)

	d.mtx.Lock()
	defer d.mtx.Unlock()

	ot, err := NewToken(token)
	if err != nil {
		return nil, err
	}

	if _, ok := d.tokenListenerMap[ot]; ok {
		return nil, fmt.Errorf("Duplicate OIC listener; token=%#v", token)
	}

	ol := NewListener()
	d.tokenListenerMap[ot] = ol
	return ol, nil
}

func (d *Dispatcher) RemoveListener(token []byte) *Listener {
	nmxutil.LogRemoveOicListener(d.logDepth, token)

	d.mtx.Lock()
	defer d.mtx.Unlock()

	ot, err := NewToken(token)
	if err != nil {
		return nil
	}

	ol := d.tokenListenerMap[ot]
	delete(d.tokenListenerMap, ot)

	return ol
}

// Returns true if the response was dispatched.
func (d *Dispatcher) Dispatch(data []byte) bool {
	var msg *coap.Message

	if d.reassembler != nil {
		// TCP.
		tm := d.reassembler.RxFrag(data)
		if tm == nil {
			return false
		}

		msg = &tm.Message
	} else {
		// UDP.
		m, err := coap.ParseMessage(data)
		if err != nil {
			log.Printf("CoAP parse failure: %s", err.Error())
			return false
		}
		msg = &m
	}

	ot, err := NewToken(msg.Token)
	if err != nil {
		return false
	}

	d.mtx.Lock()
	ol := d.tokenListenerMap[ot]
	d.mtx.Unlock()

	if ol == nil {
		log.Printf("No listener for incoming OIC message; token=%#v", ot)
		return false
	}

	ol.RspChan <- msg
	return true
}

func (d *Dispatcher) ErrorOne(token Token, err error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	ol := d.tokenListenerMap[token]
	if ol == nil {
		return fmt.Errorf("No OIC listener for token %#v", token)
	}

	ol.ErrChan <- err

	return nil
}

func (d *Dispatcher) ErrorAll(err error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	for _, ol := range d.tokenListenerMap {
		ol.ErrChan <- err
	}
}
