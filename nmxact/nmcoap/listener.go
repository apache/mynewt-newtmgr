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
	"bytes"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/runtimeco/go-coap"
)

type MsgCriteria struct {
	Token []byte
	Path  string
}

type Listener struct {
	Criteria MsgCriteria
	RspChan  chan coap.Message
	ErrChan  chan error
	tmoChan  chan time.Time
	timer    *time.Timer
}

func (mc *MsgCriteria) String() string {
	s := "token="
	if mc.Token == nil {
		s += "nil"
	} else {
		s += hex.EncodeToString(mc.Token)
	}

	s += " path="
	if mc.Path == "" {
		s += "nil"
	} else {
		s += mc.Path
	}

	return s
}

func CompareMsgCriteria(mc1 MsgCriteria, mc2 MsgCriteria) int {
	// First sort key: path.
	if diff := strings.Compare(mc1.Path, mc2.Path); diff != 0 {
		return diff
	}

	// Second sort key: token.
	if mc1.Token == nil && mc2.Token != nil {
		return -1
	}

	if mc1.Token != nil && mc2.Token == nil {
		return 1
	}

	if mc1.Token != nil {
		if diff := bytes.Compare(mc1.Token, mc2.Token); diff != 0 {
			return diff
		}
	}

	return 0
}

// Determines if a listener matches an incoming message.
func MatchMsgCriteria(listenc MsgCriteria, msgc MsgCriteria) bool {
	// First sort key: path.
	if listenc.Path != "" && listenc.Path != msgc.Path {
		return false
	}

	// Second sort key: token.
	if listenc.Token != nil {
		if bytes.Compare(listenc.Token, msgc.Token) != 0 {
			return false
		}
	}

	return true
}

func CriteriaFromMsg(msg coap.Message) MsgCriteria {
	return MsgCriteria{
		Token: msg.Token(),
		Path:  msg.PathString(),
	}
}

func NewListener(mc MsgCriteria) *Listener {
	return &Listener{
		Criteria: mc,
		RspChan:  make(chan coap.Message, 1),
		ErrChan:  make(chan error, 1),
		tmoChan:  make(chan time.Time, 1),
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

type listenerSorter struct {
	listeners []*Listener
}

func (s listenerSorter) Len() int {
	return len(s.listeners)
}
func (s listenerSorter) Swap(i, j int) {
	s.listeners[i], s.listeners[j] = s.listeners[j], s.listeners[i]
}
func (s listenerSorter) Less(i, j int) bool {
	li := s.listeners[i]
	lj := s.listeners[j]

	return CompareMsgCriteria(li.Criteria, lj.Criteria) < 0
}

func SortListeners(listeners []*Listener) {
	sorter := listenerSorter{
		listeners: listeners,
	}

	// Reverse the sort order; most specific must come first.
	sort.Sort(sort.Reverse(sorter))
}
