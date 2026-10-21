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
	"bytes"
	"encoding/json"
	"testing"
)

func TestMsgOpStrings(t *testing.T) {
	for op, name := range MsgOpStringMap {
		if got := MsgOpToString(op); got != name {
			t.Errorf("MsgOpToString(%d) = %q, want %q", int(op), got, name)
		}

		got, err := MsgOpFromString(name)
		if err != nil || got != op {
			t.Errorf("MsgOpFromString(%q) = %d, %v", name, int(got), err)
		}

		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(b) != `"`+name+`"` {
			t.Errorf("Marshal(%d) = %s", int(op), b)
		}

		var u MsgOp
		if err := json.Unmarshal(b, &u); err != nil || u != op {
			t.Errorf("Unmarshal(%s) = %d, %v", b, int(u), err)
		}
	}

	if got := MsgOpToString(MsgOp(99)); got != "???" {
		t.Errorf("MsgOpToString(99) = %q", got)
	}
	if _, err := MsgOpFromString("bogus"); err == nil {
		t.Errorf("MsgOpFromString(bogus): expected error")
	}
	var u MsgOp
	for _, bad := range []string{`"bogus"`, `1`} {
		if err := json.Unmarshal([]byte(bad), &u); err == nil {
			t.Errorf("Unmarshal(%s): expected error", bad)
		}
	}
}

func TestMsgTypeStrings(t *testing.T) {
	seen := map[string]bool{}
	for typ, name := range MsgTypeStringMap {
		if seen[name] {
			t.Errorf("duplicate MsgType name %q", name)
		}
		seen[name] = true

		if got := MsgTypeToString(typ); got != name {
			t.Errorf("MsgTypeToString(%d) = %q, want %q",
				int(typ), got, name)
		}

		got, err := MsgTypeFromString(name)
		if err != nil || got != typ {
			t.Errorf("MsgTypeFromString(%q) = %d, %v", name, int(got), err)
		}

		b, err := json.Marshal(typ)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(b) != `"`+name+`"` {
			t.Errorf("Marshal(%d) = %s", int(typ), b)
		}

		var u MsgType
		if err := json.Unmarshal(b, &u); err != nil || u != typ {
			t.Errorf("Unmarshal(%s) = %d, %v", b, int(u), err)
		}
	}

	if got := MsgTypeToString(MsgType(0)); got != "???" {
		t.Errorf("MsgTypeToString(0) = %q", got)
	}
	if _, err := MsgTypeFromString("bogus"); err == nil {
		t.Errorf("MsgTypeFromString(bogus): expected error")
	}
	var u MsgType
	for _, bad := range []string{`"bogus"`, `3`} {
		if err := json.Unmarshal([]byte(bad), &u); err == nil {
			t.Errorf("Unmarshal(%s): expected error", bad)
		}
	}
}

func TestBleHdrJSON(t *testing.T) {
	hdr := BleHdr{Op: MSG_OP_RSP, Type: MSG_TYPE_CONNECT, Seq: 42}

	b, err := json.Marshal(&hdr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"op":"response","type":"connect","seq":42}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}

	var got BleHdr
	if err := json.Unmarshal(b, &got); err != nil || got != hdr {
		t.Fatalf("Unmarshal = %+v, %v", got, err)
	}
}

func TestBleBytesMarshal(t *testing.T) {
	tests := []struct {
		in   []byte
		want string
	}{
		{nil, `""`},
		{[]byte{}, `""`},
		{[]byte{0x01}, `"0x01"`},
		{[]byte{0x01, 0xab, 0x00, 0xff}, `"0x01:0xab:0x00:0xff"`},
	}

	for _, tt := range tests {
		bb := BleBytes{tt.in}
		b, err := json.Marshal(&bb)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(b) != tt.want {
			t.Errorf("Marshal(% x) = %s, want %s", tt.in, b, tt.want)
		}
	}
}

func TestBleBytesUnmarshal(t *testing.T) {
	tests := []struct {
		in   string
		want []byte
	}{
		{`""`, nil},
		{`"0x01"`, []byte{0x01}},
		{`"0x01:0xab"`, []byte{0x01, 0xab}},
		{`"0x01:0xAB:0XFF"`, []byte{0x01, 0xab, 0xff}},
		{`"0x1:0x2"`, []byte{0x01, 0x02}},
	}

	for _, tt := range tests {
		var bb BleBytes
		if err := json.Unmarshal([]byte(tt.in), &bb); err != nil {
			t.Errorf("Unmarshal(%s): %v", tt.in, err)
			continue
		}
		if !bytes.Equal(bb.Bytes, tt.want) {
			t.Errorf("Unmarshal(%s) = % x, want % x", tt.in, bb.Bytes,
				tt.want)
		}
	}

	bad := []string{
		`"01:ab"`,
		`"0x01:ab"`,
		`"0x100"`,
		`"0xzz"`,
		`"0x01:"`,
		`"0x01,0x02"`,
		`5`,
		`[1,2]`,
	}
	for _, s := range bad {
		var bb BleBytes
		if err := json.Unmarshal([]byte(s), &bb); err == nil {
			t.Errorf("Unmarshal(%s): expected error, got % x", s, bb.Bytes)
		}
	}
}

func TestBleBytesRoundTrip(t *testing.T) {
	in := BleBytes{[]byte{0x00, 0x10, 0x7f, 0x80, 0xff}}
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out BleBytes
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(in.Bytes, out.Bytes) {
		t.Fatalf("round trip = % x, want % x", out.Bytes, in.Bytes)
	}
}

// BleBytes fields of a request are encoded in the blehostd format when the
// request is marshalled through a pointer.
func TestBleBytesInMsg(t *testing.T) {
	r := NewBleWriteCmdReq()
	r.Seq = 1
	r.Data = BleBytes{[]byte{0xde, 0xad}}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	m := map[string]interface{}{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["data"] != "0xde:0xad" || m["op"] != "request" ||
		m["type"] != "write_cmd" {

		t.Fatalf("unexpected JSON %s", b)
	}
}

func TestErrCodeTo(t *testing.T) {
	tests := []struct {
		name string
		fn   func(int) int
		base int
	}{
		{"att", ErrCodeToAtt, ERR_CODE_ATT_BASE},
		{"hci", ErrCodeToHci, ERR_CODE_HCI_BASE},
		{"l2c", ErrCodeToL2c, ERR_CODE_L2C_BASE},
		{"sm us", ErrCodeToSmUs, ERR_CODE_SM_US_BASE},
		{"sm peer", ErrCodeToSmPeer, ERR_CODE_SM_PEER_BASE},
	}

	for _, tt := range tests {
		checks := []struct {
			in   int
			want int
		}{
			{tt.base, 0},
			{tt.base + 0x13, 0x13},
			{tt.base + 0xff, 0xff},
			{tt.base - 1, -1},
			{tt.base + 0x100, -1},
			{0, -1},
		}
		for _, c := range checks {
			if got := tt.fn(c.in); got != c.want {
				t.Errorf("%s(%#x) = %d, want %d", tt.name, c.in, got, c.want)
			}
		}
	}
}

func TestErrCodeToString(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{ERR_CODE_EAGAIN, "eagain"},
		{ERR_CODE_ETIMEOUT, "etimeout"},
		{0, "unknown"},
		{0xff, "unknown"},
		{-1, "unknown"},
		{ERR_CODE_HCI_BASE + ERR_CODE_HCI_REM_USER_CONN_TERM,
			HciErrCodeStringMap[ERR_CODE_HCI_REM_USER_CONN_TERM]},
		{ERR_CODE_HCI_BASE + 0xfe, "unknown"},
		{ERR_CODE_L2C_BASE + 1, "unknown"},
		{ERR_CODE_ATT_BASE + ERR_CODE_ATT_INSUFFICIENT_ENC,
			"insufficient encryption"},
		{ERR_CODE_ATT_BASE, "unknown"},
		{ERR_CODE_ATT_BASE + 0xfe, "unknown"},
		{ERR_CODE_SM_US_BASE + ERR_CODE_SM_ERR_PASSKEY, "passkey"},
		{ERR_CODE_SM_PEER_BASE + ERR_CODE_SM_ERR_DHKEY,
			"dhkey check failed"},
		{ERR_CODE_SM_PEER_BASE + 0x100, "unknown"},
	}

	for _, tt := range tests {
		if got := ErrCodeToString(tt.in); got != tt.want {
			t.Errorf("ErrCodeToString(%#x) = %q, want %q", tt.in, got,
				tt.want)
		}
	}

	for code, name := range ErrCodeStringMap {
		if got := ErrCodeToString(code); got != name {
			t.Errorf("ErrCodeToString(%d) = %q, want %q", code, got, name)
		}
	}
	for code, name := range AttErrCodeStringMap {
		if got := ErrCodeToString(ERR_CODE_ATT_BASE + code); got != name {
			t.Errorf("ErrCodeToString(att %d) = %q, want %q", code, got,
				name)
		}
	}
	for code, name := range SmErrCodeStringMap {
		for _, base := range []int{ERR_CODE_SM_US_BASE,
			ERR_CODE_SM_PEER_BASE} {

			if got := ErrCodeToString(base + code); got != name {
				t.Errorf("ErrCodeToString(%#x) = %q, want %q", base+code,
					got, name)
			}
		}
	}
	for code, name := range HciErrCodeStringMap {
		if got := ErrCodeToString(ERR_CODE_HCI_BASE + code); got != name {
			t.Errorf("ErrCodeToString(hci %d) = %q, want %q", code, got,
				name)
		}
	}
}
