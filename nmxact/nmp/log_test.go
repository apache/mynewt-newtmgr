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

package nmp

import (
	"bytes"
	"testing"
)

func TestLogEntryTypeString(t *testing.T) {
	for typ, name := range LogEntryTypeStringMap {
		if got := LogEntryTypeToString(typ); got != name {
			t.Errorf("LogEntryTypeToString(%d) = %q, want %q",
				int(typ), got, name)
		}
		if got := typ.String(); got != name {
			t.Errorf("String(%d) = %q, want %q", int(typ), got, name)
		}

		got, err := LogEntryTypeFromString(name)
		if err != nil {
			t.Errorf("LogEntryTypeFromString(%q): %v", name, err)
		}
		if got != typ {
			t.Errorf("LogEntryTypeFromString(%q) = %d, want %d",
				name, int(got), int(typ))
		}
	}

	if got := LogEntryTypeToString(LogEntryType(42)); got != "???" {
		t.Errorf("unknown type: got %q, want ???", got)
	}
	if _, err := LogEntryTypeFromString("bogus"); err == nil {
		t.Errorf("expected error for unknown string")
	}
}

// LogEntryType must be marshalled as the raw string, not as a nested CBOR
// string.  Regression test for "Fix [un]marshal of log-entry-type".
func TestLogEntryTypeBinary(t *testing.T) {
	for typ, name := range LogEntryTypeStringMap {
		b, err := typ.MarshalBinary()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(b, []byte(name)) {
			t.Errorf("MarshalBinary(%d) = % x, want %q", int(typ), b, name)
		}

		var got LogEntryType
		if err := got.UnmarshalBinary([]byte(name)); err != nil {
			t.Errorf("UnmarshalBinary(%q): %v", name, err)
		}
		if got != typ {
			t.Errorf("UnmarshalBinary(%q) = %d, want %d",
				name, int(got), int(typ))
		}
	}

	var l LogEntryType
	if err := l.UnmarshalBinary([]byte("nope")); err == nil {
		t.Errorf("expected error for unknown type")
	}
}

func TestLogShowRspDecode(t *testing.T) {
	// Body as sent by a device: the entry type is a CBOR text string.
	body, err := BodyBytes(map[string]interface{}{
		"rc":         0,
		"next_index": 3,
		"logs": []interface{}{
			map[string]interface{}{
				"name": "reboot_log",
				"type": 1,
				"entries": []interface{}{
					map[string]interface{}{
						"index":  1,
						"ts":     1000,
						"module": 2,
						"level":  1,
						"type":   "cbor",
						"msg":    []byte{0xa0},
					},
					map[string]interface{}{
						"index":  2,
						"ts":     2000,
						"module": 3,
						"level":  0,
						"type":   "str",
						"msg":    []byte("hello"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hdr := &NmpHdr{Op: NMP_OP_READ_RSP, Group: NMP_GROUP_LOG,
		Id: NMP_ID_LOG_SHOW}
	rsp, err := DecodeRspBody(hdr, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := rsp.(*LogShowRsp)
	if r.NextIndex != 3 || len(r.Logs) != 1 ||
		len(r.Logs[0].Entries) != 2 {

		t.Fatalf("unexpected rsp %+v", r)
	}
	e := r.Logs[0].Entries
	if e[0].Type != LOG_ENTRY_TYPE_CBOR || e[1].Type != LOG_ENTRY_TYPE_STRING {
		t.Fatalf("entry types = %v, %v; want cbor, str", e[0].Type, e[1].Type)
	}
	if string(e[1].Msg) != "hello" || e[0].Timestamp != 1000 {
		t.Fatalf("unexpected entries %+v", e)
	}

	// Encoding the decoded response and decoding it again yields the same
	// entry types.
	body2, err := BodyBytes(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rsp2, err := DecodeRspBody(hdr, body2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e2 := rsp2.(*LogShowRsp).Logs[0].Entries
	if e2[0].Type != LOG_ENTRY_TYPE_CBOR || e2[1].Type != LOG_ENTRY_TYPE_STRING {
		t.Fatalf("round trip entry types = %v, %v", e2[0].Type, e2[1].Type)
	}
}

func TestLogNameHelpers(t *testing.T) {
	tests := []struct {
		fn   func(int) string
		in   int
		want string
	}{
		{LogModuleToString, MODULE_NEWTMGR, "NEWTMGR"},
		{LogModuleToString, MODULE_TEST, "TEST"},
		{LogModuleToString, 7, "CUSTOM"},
		{LogLevelToString, LEVEL_WARN, "WARN"},
		{LogLevelToString, 100, "CUSTOM"},
		{LogTypeToString, STORAGE_LOG, "STORAGE"},
		{LogTypeToString, 9, "UNDEFINED"},
	}

	for _, tt := range tests {
		if got := tt.fn(tt.in); got != tt.want {
			t.Errorf("%d: got %q, want %q", tt.in, got, tt.want)
		}
	}
}
