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
	"testing"
)

func TestListenerKeys(t *testing.T) {
	k := TgtPortKey("0011223344556677", 5, "rx")
	if k.Target != "00112233445566775" || k.Type != "rx" {
		t.Fatalf("TgtPortKey() = %+v", k)
	}

	k = TgtKey("0011223344556677", "req")
	if k.Target != "0011223344556677" || k.Type != "req" {
		t.Fatalf("TgtKey() = %+v", k)
	}

	k = TypeKey("rx")
	if k.Target != "" || k.Type != "rx" {
		t.Fatalf("TypeKey() = %+v", k)
	}
}

func TestListenerMap(t *testing.T) {
	lm := NewListenerMap()

	tgt := "0011223344556677"
	specific := NewListener()
	generic := NewListener()

	if _, l := lm.FindListener(tgt, 1, "rx"); l != nil {
		t.Fatalf("empty map returned listener")
	}

	if err := lm.AddListener(TypeKey("rx"), generic); err != nil {
		t.Fatalf("AddListener: %v", err)
	}

	// Falls back to the type-only key.
	key, l := lm.FindListener(tgt, 1, "rx")
	if l != generic || key != TypeKey("rx") {
		t.Fatalf("FindListener() = %+v, %p; want generic", key, l)
	}

	if err := lm.AddListener(TgtPortKey(tgt, 1, "rx"), specific); err != nil {
		t.Fatalf("AddListener: %v", err)
	}

	// Target+port key takes precedence.
	key, l = lm.FindListener(tgt, 1, "rx")
	if l != specific || key != TgtPortKey(tgt, 1, "rx") {
		t.Fatalf("FindListener() = %+v, %p; want specific", key, l)
	}

	// Other port or type.
	if _, l := lm.FindListener(tgt, 2, "rx"); l != generic {
		t.Fatalf("other port: got %p, want generic", l)
	}
	if _, l := lm.FindListener(tgt, 1, "tx"); l != nil {
		t.Fatalf("other type: got %p, want nil", l)
	}

	// Duplicates are rejected.
	if err := lm.AddListener(TypeKey("rx"), NewListener()); err == nil {
		t.Fatalf("duplicate key accepted")
	}
	if err := lm.AddListener(TypeKey("other"), generic); err == nil {
		t.Fatalf("duplicate listener accepted")
	}

	// Removal by listener.
	if k := lm.RemoveListener(specific); k == nil ||
		*k != TgtPortKey(tgt, 1, "rx") {

		t.Fatalf("RemoveListener() = %v", k)
	}
	if k := lm.RemoveListener(specific); k != nil {
		t.Fatalf("second RemoveListener() = %v, want nil", k)
	}
	if _, l := lm.FindListener(tgt, 1, "rx"); l != generic {
		t.Fatalf("after removal: got %p, want generic", l)
	}

	// Removal by key.
	if l := lm.RemoveKey(TypeKey("rx")); l != generic {
		t.Fatalf("RemoveKey() = %p, want generic", l)
	}
	if l := lm.RemoveKey(TypeKey("rx")); l != nil {
		t.Fatalf("second RemoveKey() = %p, want nil", l)
	}
	if _, l := lm.FindListener(tgt, 1, "rx"); l != nil {
		t.Fatalf("empty map returned listener")
	}

	// A removed listener can be added again.
	if err := lm.AddListener(TypeKey("rx"), generic); err != nil {
		t.Fatalf("re-add: %v", err)
	}
}

func TestListenerSlice(t *testing.T) {
	ls := NewListenerSlice()
	tgt := "0011223344556677"

	if _, s := ls.FindListener(tgt, "req"); s != nil {
		t.Fatalf("empty slice map returned %v", s)
	}

	g1 := NewListener()
	g2 := NewListener()
	spec := NewListener()

	for _, l := range []*Listener{g1, g2} {
		if err := ls.AddListener(TypeKey("req"), l); err != nil {
			t.Fatalf("AddListener: %v", err)
		}
	}

	key, s := ls.FindListener(tgt, "req")
	if key != TypeKey("req") || len(s) != 2 {
		t.Fatalf("FindListener() = %+v, %v", key, s)
	}

	if err := ls.AddListener(TgtKey(tgt, "req"), spec); err != nil {
		t.Fatalf("AddListener: %v", err)
	}
	key, s = ls.FindListener(tgt, "req")
	if key != TgtKey(tgt, "req") || len(s) != 1 || s[0] != spec {
		t.Fatalf("FindListener() = %+v, %v; want specific", key, s)
	}

	// The same listener cannot be added twice.
	if err := ls.AddListener(TypeKey("other"), g1); err == nil {
		t.Fatalf("duplicate listener accepted")
	}

	// Removing one of several listeners keeps the others.
	if k := ls.RemoveListener(g1); k == nil || *k != TypeKey("req") {
		t.Fatalf("RemoveListener() = %v", k)
	}
	if k := ls.RemoveListener(g1); k != nil {
		t.Fatalf("second RemoveListener() = %v, want nil", k)
	}
	if _, s := ls.FindListener("other", "req"); len(s) != 1 || s[0] != g2 {
		t.Fatalf("after removal: %v, want [g2]", s)
	}

	// Removing the last listener removes the key.
	ls.RemoveListener(g2)
	if _, s := ls.FindListener("other", "req"); s != nil {
		t.Fatalf("after removing all: %v, want nil", s)
	}
	ls.RemoveListener(spec)
	if _, s := ls.FindListener(tgt, "req"); s != nil {
		t.Fatalf("after removing all: %v, want nil", s)
	}
}

func TestListenerRefCnt(t *testing.T) {
	l := NewListener()
	l.RefCnt++

	l.MsgChan <- []byte{1}

	// First Close only drops a reference.
	l.Close()
	select {
	case b, ok := <-l.MsgChan:
		if !ok || len(b) != 1 {
			t.Fatalf("MsgChan closed too early")
		}
	default:
		t.Fatalf("queued message lost")
	}

	// Second Close closes and drains the channels.
	l.MsgChan <- []byte{2}
	l.MtuChan <- 100
	l.Close()
	if _, ok := <-l.MsgChan; ok {
		t.Fatalf("MsgChan not closed")
	}
	if _, ok := <-l.MtuChan; ok {
		t.Fatalf("MtuChan not closed")
	}
	if _, ok := <-l.ConnChan; ok {
		t.Fatalf("ConnChan not closed")
	}
}
