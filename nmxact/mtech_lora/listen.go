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
	"strconv"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type ListenerKey struct {
	Target string
	Type   string
}

func TgtPortKey(tgt string, port uint8, msgType string) ListenerKey {
	return ListenerKey{
		Target: tgt + strconv.Itoa(int(port)),
		Type:   msgType,
	}
}

func TgtKey(tgt string, msgType string) ListenerKey {
	return ListenerKey{
		Target: tgt,
		Type:   msgType,
	}
}

func TypeKey(msgType string) ListenerKey {
	return ListenerKey{
		Target: "",
		Type:   msgType,
	}
}

type Listener struct {
	MsgChan  chan []byte
	MtuChan  chan int
	ConnChan chan *LoraSesn

	RefCnt   int
	Data     *bytes.Buffer
	NextFrag uint8
	Crc      uint16
}

func NewListener() *Listener {
	return &Listener{
		MsgChan:  make(chan []byte, 16),
		MtuChan:  make(chan int, 1),
		ConnChan: make(chan *LoraSesn, 4),

		RefCnt: 1,
		Data: bytes.NewBuffer([]byte{}),
	}
}

func (ll *Listener) Close() {

	ll.RefCnt--
	if ll.RefCnt > 0 {
		return
	}

	close(ll.MsgChan)
	for {
		if _, ok := <-ll.MsgChan; !ok {
			break
		}
	}

	close(ll.MtuChan)
	for {
		if _, ok := <-ll.MtuChan; !ok {
			break
		}
	}

	close(ll.ConnChan)
	for {
		l, ok := <-ll.ConnChan
		if !ok {
			break
		}
		l.Close()
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

func (lm *ListenerMap) FindListener(tgt string, port uint8, msgType string) (
	ListenerKey, *Listener) {

	var key ListenerKey

	key = TgtPortKey(tgt, port, msgType)
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

func (lm *ListenerMap) Dump() {
	fmt.Printf(" key -> listener\n")
	for key, l := range lm.k2l {
		fmt.Printf("  %s: %p,%d\n", key, l, l.RefCnt)
	}
	fmt.Printf(" listener -> key\n")
	for l, key := range lm.l2k {
		fmt.Printf("  %p %s\n", l, key)
	}
}

// not thread safe
type ListenerSlice struct {
	k2l map[ListenerKey][]*Listener
	l2k map[*Listener]ListenerKey
}

func NewListenerSlice() *ListenerSlice {
	return &ListenerSlice{
		k2l: map[ListenerKey][]*Listener{},
		l2k: map[*Listener]ListenerKey{},
	}
}

func (lm *ListenerSlice) FindListener(tgt string, msgType string) (
	ListenerKey, []*Listener) {

	var key ListenerKey

	key = TgtKey(tgt, msgType)
	if lSlice := lm.k2l[key]; lSlice != nil {
		return key, lSlice
	}

	key = TypeKey(msgType)
	if lSlice := lm.k2l[key]; lSlice != nil {
		return key, lSlice
	}

	return key, nil
}

func (lm *ListenerSlice) AddListener(key ListenerKey, listener *Listener) error {
	if _, ok := lm.l2k[listener]; ok {
		nmxutil.Assert(false)
		return fmt.Errorf("Duplicate Lora listener: %#v", key)
	}

	lm.k2l[key] = append(lm.k2l[key], listener)
	lm.l2k[listener] = key

	return nil
}

func (lm *ListenerSlice) deleteListener(key ListenerKey, listener *Listener) {
	nmxutil.Assert(lm.l2k[listener] == key)

	for i, elem := range lm.k2l[key] {
		if elem == listener {
			slen := len(lm.k2l[key])
			lm.k2l[key][i] = lm.k2l[key][slen-1]
			lm.k2l[key][slen-1] = nil
			lm.k2l[key] = lm.k2l[key][:slen-1]
			if slen == 1 {
				delete(lm.k2l, key)
			}
			break
		}
	}
	delete(lm.l2k, listener)
}

func (lm *ListenerSlice) RemoveListener(listener *Listener) *ListenerKey {
	key, ok := lm.l2k[listener]
	if !ok {
		return nil
	}

	lm.deleteListener(key, listener)
	return &key
}

func (ls *ListenerSlice) Dump() {
	fmt.Printf(" key -> listener\n")
	for key, sl := range ls.k2l {
		fmt.Printf("  %s\n\t", key)
		for i, elem := range sl {
			fmt.Printf("[%d %p] ", i, elem)
		}
		fmt.Printf("\n")
	}
	fmt.Printf(" listener -> key\n")
	for l, key := range ls.l2k {
		fmt.Printf("  %p %s\n", l, key)
	}
}
