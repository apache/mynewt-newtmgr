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

	"github.com/runtimeco/go-coap"
	log "github.com/sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	listeners []*Listener
	rxer      Receiver
	logDepth  int
	mtx       sync.Mutex
}

func NewDispatcher(isTcp bool, logDepth int) *Dispatcher {
	d := &Dispatcher{
		rxer:     NewReceiver(isTcp),
		logDepth: logDepth + 2,
	}

	return d
}

func (d *Dispatcher) findListenerIdx(mc MsgCriteria) int {
	for i, lner := range d.listeners {
		if CompareMsgCriteria(lner.Criteria, mc) == 0 {
			return i
		}
	}

	return -1
}

func (d *Dispatcher) matchListener(mc MsgCriteria) *Listener {
	for _, lner := range d.listeners {
		if MatchMsgCriteria(lner.Criteria, mc) {
			return lner
		}
	}

	return nil
}

func (d *Dispatcher) AddListener(mc MsgCriteria) (*Listener, error) {
	nmxutil.LogAddCoapListener(d.logDepth, mc.String())

	d.mtx.Lock()
	defer d.mtx.Unlock()

	if idx := d.findListenerIdx(mc); idx != -1 {
		return nil, fmt.Errorf("duplicate CoAP listener: %s", mc.String())
	}

	lner := NewListener(mc)
	d.listeners = append(d.listeners, lner)
	SortListeners(d.listeners)

	return lner, nil
}

func (d *Dispatcher) RemoveListener(mc MsgCriteria) *Listener {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	idx := d.findListenerIdx(mc)
	if idx == -1 {
		return nil
	}
	lner := d.listeners[idx]

	d.listeners = append(d.listeners[:idx], d.listeners[idx+1:]...)

	nmxutil.LogRemoveCoapListener(d.logDepth, mc.String())
	lner.Close()

	return lner
}

// Returns true if the response was dispatched.
func (d *Dispatcher) Dispatch(data []byte) bool {
	// See if this fragment completes a packet.
	msg := d.rxer.Rx(data)
	if msg == nil {
		return false
	}

	d.mtx.Lock()
	defer d.mtx.Unlock()

	mc := CriteriaFromMsg(msg)
	lner := d.matchListener(mc)
	if lner == nil {
		log.Debugf("no listener for incoming CoAP message: %s", mc.String())
		return false
	}

	lner.RspChan <- msg
	return true
}

func (d *Dispatcher) ProcessCoapReq(data []byte) (coap.Message, error) {
	m := d.rxer.Rx(data)
	if m == nil {
		return nil, nil
	}
	return m, nil
}

func (d *Dispatcher) ErrorOne(mc MsgCriteria, err error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	lner := d.matchListener(mc)
	if lner == nil {
		return fmt.Errorf("no CoAP listener: %s", mc.String())
	}

	lner.ErrChan <- err

	return nil
}

func (d *Dispatcher) ErrorAll(err error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	for _, lner := range d.listeners {
		lner.ErrChan <- err
	}
}
