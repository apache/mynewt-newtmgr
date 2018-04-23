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
	RspChan chan coap.Message
	ErrChan chan error
	tmoChan chan time.Time
	timer   *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		RspChan: make(chan coap.Message, 1),
		ErrChan: make(chan error, 1),
		tmoChan: make(chan time.Time, 1),
	}
}

func (ol *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		if ol.tmoChan != nil {
			ol.tmoChan <- time.Now()
		}
	}
	ol.timer = time.AfterFunc(tmo, fn)
	return ol.tmoChan
}

func (ol *Listener) Close() {
	if ol.timer != nil {
		ol.timer.Stop()
	}

	close(ol.RspChan)
	close(ol.ErrChan)
	close(ol.tmoChan)
	ol.tmoChan = nil
}

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	tokenListenerMap map[Token]*Listener
	rxer             Receiver
	logDepth         int
	mtx              sync.Mutex
}

func NewDispatcher(isTcp bool, logDepth int) *Dispatcher {
	d := &Dispatcher{
		tokenListenerMap: map[Token]*Listener{},
		rxer:             NewReceiver(isTcp),
		logDepth:         logDepth + 2,
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
	d.mtx.Lock()
	defer d.mtx.Unlock()

	ot, err := NewToken(token)
	if err != nil {
		log.Errorf("Error creating OIC token: %s", err.Error())
		return nil
	}

	ol := d.tokenListenerMap[ot]
	if ol == nil {
		return nil
	}

	nmxutil.LogRemoveOicListener(d.logDepth, token)
	ol.Close()
	delete(d.tokenListenerMap, ot)

	return ol
}

func (d *Dispatcher) dispatchRsp(ot Token, msg coap.Message) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	ol := d.tokenListenerMap[ot]

	if ol == nil {
		log.Debugf("No listener for incoming OIC message; token=%#v", ot)
		return false
	}

	ol.RspChan <- msg
	return true
}

// Returns true if the response was dispatched.
func (d *Dispatcher) Dispatch(data []byte) bool {
	m := d.rxer.Rx(data)
	if m == nil {
		return false
	}

	ot, err := NewToken(m.Token())
	if err != nil {
		return false
	}

	return d.dispatchRsp(ot, m)
}

func (d *Dispatcher) ProcessCoapReq(data []byte) (coap.Message, error) {
	m := d.rxer.Rx(data)
	if m == nil {
		return nil, nil
	}
	return m, nil
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
