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
)

type OicToken struct {
	Len  int
	Data [8]byte
}

func NewOicToken(rawToken []byte) (OicToken, error) {
	ot := OicToken{}

	if len(rawToken) > 8 {
		return ot, fmt.Errorf("Invalid CoAP token: too long (%d bytes)",
			len(rawToken))
	}

	ot.Len = len(rawToken)
	copy(ot.Data[:], rawToken)

	return ot, nil
}

type OicListener struct {
	RspChan chan *coap.Message
	ErrChan chan error
	tmoChan chan time.Time
	timer   *time.Timer
}

func NewOicListener() *OicListener {
	return &OicListener{
		RspChan: make(chan *coap.Message, 1),
		ErrChan: make(chan error, 1),
		tmoChan: make(chan time.Time, 1),
	}
}

func (ol *OicListener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		ol.tmoChan <- time.Now()
	}
	ol.timer = time.AfterFunc(tmo, fn)
	return ol.tmoChan
}

type OicDispatcher struct {
	tokenListenerMap map[OicToken]*OicListener
	mtx              sync.Mutex
	reassembler      *Reassembler
}

func NewOicDispatcher(isTcp bool) *OicDispatcher {
	od := &OicDispatcher{
		tokenListenerMap: map[OicToken]*OicListener{},
	}

	if isTcp {
		od.reassembler = NewReassembler()
	}

	return od
}

func (od *OicDispatcher) AddListener(token []byte, ol *OicListener) error {
	od.mtx.Lock()
	defer od.mtx.Unlock()

	ot, err := NewOicToken(token)
	if err != nil {
		return err
	}

	if _, ok := od.tokenListenerMap[ot]; ok {
		return fmt.Errorf("Duplicate OIC listener; token=%#v", token)
	}

	od.tokenListenerMap[ot] = ol
	return nil
}

func (od *OicDispatcher) RemoveListener(token []byte) *OicListener {
	od.mtx.Lock()
	defer od.mtx.Unlock()

	ot, err := NewOicToken(token)
	if err != nil {
		return nil
	}

	ol := od.tokenListenerMap[ot]
	delete(od.tokenListenerMap, ot)

	return ol
}

// Returns true if the response was dispatched.
func (od *OicDispatcher) Dispatch(data []byte) bool {
	var msg *coap.Message

	if od.reassembler != nil {
		// TCP.
		tm := od.reassembler.RxFrag(data)
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

	ot, err := NewOicToken(msg.Token)
	if err != nil {
		return false
	}

	od.mtx.Lock()
	ol := od.tokenListenerMap[ot]
	od.mtx.Unlock()

	if ol == nil {
		log.Printf("No listener for incoming OIC message; token=%#v", ot)
		return false
	}

	ol.RspChan <- msg
	return true
}
