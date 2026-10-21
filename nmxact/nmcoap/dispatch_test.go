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
	"testing"
)

// A single TCP chunk containing two complete messages must dispatch both
// of them immediately, not just the first one.
func TestDispatcherTcpBackToBack(t *testing.T) {
	d := NewDispatcher(true, 0)

	l1, err := d.AddListener(MsgCriteria{Token: []byte{1}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	l2, err := d.AddListener(MsgCriteria{Token: []byte{2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer d.RemoveListener(l1.Criteria)
	defer d.RemoveListener(l2.Criteria)

	b1 := tcpMsg(t, []byte{1}, "", []byte("one"))
	b2 := tcpMsg(t, []byte{2}, "", []byte("two"))
	if !d.Dispatch(append(append([]byte{}, b1...), b2...)) {
		t.Fatalf("Dispatch() = false, want true")
	}

	for i, l := range []*Listener{l1, l2} {
		select {
		case m := <-l.RspChan:
			if m.Token()[0] != byte(i+1) {
				t.Fatalf("listener %d got token % x", i+1, m.Token())
			}
		default:
			t.Fatalf("listener %d did not receive its message", i+1)
		}
	}
}

func TestDispatcherNoListener(t *testing.T) {
	d := NewDispatcher(true, 0)
	if d.Dispatch(tcpMsg(t, []byte{1}, "", nil)) {
		t.Fatalf("Dispatch() = true without a listener")
	}
}
