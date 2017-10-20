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

package mtech_lora

import (
	"bytes"
	"fmt"
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type ListenerKey struct {
	Target string
	Type string
}

func TgtKey(tgt string, msgType string) ListenerKey {
	return ListenerKey{
		Target: tgt,
		Type: msgType,
	}
}

func TypeKey(msgType string) ListenerKey {
	return ListenerKey{
		Target: "",
		Type: msgType,
	}
}

type Listener struct {
	MsgChan chan []byte
	ErrChan chan error
	TmoChan chan time.Time
	Acked   bool

	Data     *bytes.Buffer
	NextFrag uint8
	Crc      uint16

	timer *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		MsgChan: make(chan []byte, 16),
		ErrChan: make(chan error, 1),
		TmoChan: make(chan time.Time, 1),

		Data: bytes.NewBuffer([]byte{}),
	}
}

func (ll *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		if !ll.Acked {
			ll.TmoChan <- time.Now()
		}
	}
	ll.timer = time.AfterFunc(tmo, fn)
	return ll.TmoChan
}

func (ll *Listener) Close() {
	// This provokes a race condition.  The timer may get initialized at any
	// time.
	if ll.timer != nil {
		ll.timer.Stop()
	}

	// Mark the command as acked in case the race condition mentioned above
	// occurred.  If the timer goes off, nothing will happen.
	ll.Acked = true

	close(ll.MsgChan)
	for {
		if _, ok := <-ll.MsgChan; !ok {
			break
		}
	}

	close(ll.ErrChan)
	for {
		if _, ok := <-ll.ErrChan; !ok {
			break
		}
	}

	close(ll.TmoChan)
	for {
		if _, ok := <-ll.TmoChan; !ok {
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

func (lm *ListenerMap) FindListener(tgt string, msgType string) (
	ListenerKey, *Listener) {

	var key ListenerKey

	key = TgtKey(tgt, msgType)
	if listener := lm.k2l[key]; listener != nil {
		return key, listener
	}

	key = TypeKey(msgType)
	if listener := lm.k2l[key]; listener != nil {
		return key, listener
	}

	return key, nil
}

func (lm *ListenerMap) AddListener(key ListenerKey, listener *Listener) error {
	if _, ok := lm.k2l[key]; ok {
		nmxutil.Assert(false)
		return fmt.Errorf("Duplicate Lora listener: %#v", key)
	}

	if _, ok := lm.l2k[listener]; ok {
		nmxutil.Assert(false)
		return fmt.Errorf("Duplicate Lora listener: %#v", key)
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
