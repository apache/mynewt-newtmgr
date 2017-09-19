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

package nmble

import (
	"fmt"
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type ListenerKey struct {
	// seq-key only.
	Seq BleSeq

	// tch-key only (type-conn-handle).
	Type       MsgType
	ConnHandle int
}

// Listener that matches by sequence number.
func SeqKey(seq BleSeq) ListenerKey {
	return ListenerKey{
		Seq:        seq,
		Type:       -1,
		ConnHandle: -1,
	}
}

// Listener that matches by TCH (type and conn-handle).
func TchKey(typ MsgType, connHandle int) ListenerKey {
	return ListenerKey{
		Seq:        BLE_SEQ_NONE,
		Type:       typ,
		ConnHandle: connHandle,
	}
}

type Listener struct {
	MsgChan chan Msg
	ErrChan chan error
	TmoChan chan time.Time
	Acked   bool

	timer *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		MsgChan: make(chan Msg, 16),
		ErrChan: make(chan error, 1),
		TmoChan: make(chan time.Time, 1),
	}
}

func (bl *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		if !bl.Acked {
			bl.TmoChan <- time.Now()
		}
	}
	bl.timer = time.AfterFunc(tmo, fn)
	return bl.TmoChan
}

func (bl *Listener) Close() {
	// This provokes a race condition.  The timer may get initialized at any
	// time.
	if bl.timer != nil {
		bl.timer.Stop()
	}

	// Mark the command as acked in case the race condition mentioned above
	// occurred.  If the timer goes off, nothing will happen.
	bl.Acked = true

	close(bl.MsgChan)
	for {
		if _, ok := <-bl.MsgChan; !ok {
			break
		}
	}

	close(bl.ErrChan)
	for {
		if _, ok := <-bl.ErrChan; !ok {
			break
		}
	}

	close(bl.TmoChan)
	for {
		if _, ok := <-bl.TmoChan; !ok {
			break
		}
	}
}

// Not thread safe.
type ListenerMap struct {
	k2l map[ListenerKey]*Listener
	l2k map[*Listener]ListenerKey
}

func NewListenerMap() *ListenerMap {
	return &ListenerMap{
		k2l: map[ListenerKey]*Listener{},
		l2k: map[*Listener]ListenerKey{},
	}
}

func (lm *ListenerMap) FindListener(seq BleSeq, typ MsgType, connHandle int) (
	ListenerKey, *Listener) {

	var key ListenerKey

	// First, find by sequence number.
	key = SeqKey(seq)
	if listener := lm.k2l[key]; listener != nil {
		return key, listener
	}

	// Otherwise, find by other fields.
	key = TchKey(typ, connHandle)
	if listener := lm.k2l[key]; listener != nil {
		return key, listener
	}
	key = TchKey(typ, -1)
	if listener := lm.k2l[key]; listener != nil {
		return key, listener
	}

	return key, nil
}

func (lm *ListenerMap) AddListener(key ListenerKey, listener *Listener) error {
	if _, ok := lm.k2l[key]; ok {
		nmxutil.Assert(false)
		return fmt.Errorf("Duplicate BLE listener: %#v", key)
	}

	if _, ok := lm.l2k[listener]; ok {
		nmxutil.Assert(false)
		return fmt.Errorf("Duplicate BLE listener: %#v", key)
	}

	lm.k2l[key] = listener
	lm.l2k[listener] = key

	return nil
}

func (lm *ListenerMap) deleteListener(key ListenerKey, listener *Listener) {
	nmxutil.Assert(lm.k2l[key] == listener)
	nmxutil.Assert(lm.l2k[listener] == key)
	delete(lm.k2l, key)
	delete(lm.l2k, listener)
}

func (lm *ListenerMap) RemoveListener(listener *Listener) *ListenerKey {
	key, ok := lm.l2k[listener]
	if !ok {
		return nil
	}

	lm.deleteListener(key, listener)
	return &key
}

func (lm *ListenerMap) RemoveKey(key ListenerKey) *Listener {
	listener := lm.k2l[key]
	if listener == nil {
		return nil
	}

	lm.deleteListener(key, listener)
	return listener
}

func (lm *ListenerMap) ExtractAll() []*Listener {
	listeners := make([]*Listener, 0, len(lm.l2k))

	for listener, _ := range lm.l2k {
		listeners = append(listeners, listener)
	}

	lm.k2l = map[ListenerKey]*Listener{}
	lm.l2k = map[*Listener]ListenerKey{}

	return listeners
}
