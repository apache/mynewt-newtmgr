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

package bledefs

import (
	"encoding/json"
	"testing"
)

func TestParseBleAddr(t *testing.T) {
	tests := []struct {
		in   string
		want [6]byte
		str  string
	}{
		{"00:00:00:00:00:00", [6]byte{}, "00:00:00:00:00:00"},
		{"01:23:45:67:89:ab", [6]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab},
			"01:23:45:67:89:ab"},
		{"FF:EE:DD:CC:BB:AA", [6]byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa},
			"ff:ee:dd:cc:bb:aa"},
		{"1:2:3:4:5:6", [6]byte{1, 2, 3, 4, 5, 6}, "01:02:03:04:05:06"},
	}

	for _, tt := range tests {
		ba, err := ParseBleAddr(tt.in)
		if err != nil {
			t.Errorf("ParseBleAddr(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if ba.Bytes != tt.want {
			t.Errorf("ParseBleAddr(%q) = % x, want % x", tt.in, ba.Bytes,
				tt.want)
		}
		if s := ba.String(); s != tt.str {
			t.Errorf("String() = %q, want %q", s, tt.str)
		}

		// String output parses back to the same address.
		ba2, err := ParseBleAddr(ba.String())
		if err != nil || ba2 != ba {
			t.Errorf("round trip of %q: %v, %v", tt.in, ba2, err)
		}
	}
}

func TestParseBleAddrInvalid(t *testing.T) {
	bad := []string{
		"",
		"01:23:45:67:89",
		"01:23:45:67:89:ab:cd",
		"01-23-45-67-89-ab",
		"01:23:45:67:89:xy",
		"01:23:45:67:89:100",
		"01:23:45:67::ab",
		"0x1:23:45:67:89:ab",
	}

	for _, s := range bad {
		if ba, err := ParseBleAddr(s); err == nil {
			t.Errorf("ParseBleAddr(%q) = %v, expected error", s, ba)
		}
	}
}

func TestBleAddrJSON(t *testing.T) {
	ba, _ := ParseBleAddr("01:02:03:04:05:06")

	b, err := json.Marshal(&ba)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != `"01:02:03:04:05:06"` {
		t.Fatalf("Marshal = %s", b)
	}

	var got BleAddr
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != ba {
		t.Fatalf("Unmarshal = %v, want %v", got, ba)
	}

	for _, bad := range []string{`"zz"`, `5`, `null`} {
		if err := json.Unmarshal([]byte(bad), &got); err == nil {
			t.Errorf("Unmarshal(%s): expected error", bad)
		}
	}
}

func TestBleDevString(t *testing.T) {
	ba, _ := ParseBleAddr("aa:bb:cc:dd:ee:ff")
	d := BleDev{AddrType: BLE_ADDR_TYPE_RANDOM, Addr: ba}
	if s := d.String(); s != "random,aa:bb:cc:dd:ee:ff" {
		t.Fatalf("String() = %q", s)
	}
}

func TestParseUuid16(t *testing.T) {
	tests := []struct {
		in   string
		want BleUuid16
		str  string
	}{
		{"0x1800", 0x1800, "0x1800"},
		{"0xfe18", 0xfe18, "0xfe18"},
		{"0XFFFF", 0xffff, "0xffff"},
		// Base is inferred from the prefix; no prefix means decimal.
		{"10", 10, "0x000a"},
	}

	for _, tt := range tests {
		got, err := ParseUuid16(tt.in)
		if err != nil {
			t.Errorf("ParseUuid16(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseUuid16(%q) = %#x, want %#x", tt.in, got, tt.want)
		}
		if s := got.String(); s != tt.str {
			t.Errorf("String() = %q, want %q", s, tt.str)
		}
	}

	for _, bad := range []string{"", "0x10000", "-1", "0xzz",
		"ade3d529-c784-4f63-a987-eb69f70ee816"} {

		if _, err := ParseUuid16(bad); err == nil {
			t.Errorf("ParseUuid16(%q): expected error", bad)
		}
	}
}

func TestParseUuid128(t *testing.T) {
	s := "ade3d529-c784-4f63-a987-eb69f70ee816"
	want := BleUuid128{
		0xad, 0xe3, 0xd5, 0x29, 0xc7, 0x84, 0x4f, 0x63,
		0xa9, 0x87, 0xeb, 0x69, 0xf7, 0x0e, 0xe8, 0x16,
	}

	got, err := ParseUuid128(s)
	if err != nil {
		t.Fatalf("ParseUuid128: %v", err)
	}
	if got != want {
		t.Fatalf("ParseUuid128 = % x, want % x", got, want)
	}
	if got.String() != s {
		t.Fatalf("String() = %q, want %q", got.String(), s)
	}

	// Upper case input is accepted; output is lower case.
	got, err = ParseUuid128("ADE3D529-C784-4F63-A987-EB69F70EE816")
	if err != nil || got != want {
		t.Fatalf("upper case: % x, %v", got, err)
	}

	bad := []string{
		"",
		"ade3d529c7844f63a987eb69f70ee816",
		"ade3d529-c784-4f63-a987-eb69f70ee81",
		"ade3d529-c784-4f63-a987-eb69f70ee8166",
		"ade3d529_c784-4f63-a987-eb69f70ee816",
		"ade3d529-c784-4f63-a987-eb69f70ee8zz",
		"ade3d52-9c784-4f63-a987-eb69f70ee816",
	}
	for _, b := range bad {
		if _, err := ParseUuid128(b); err == nil {
			t.Errorf("ParseUuid128(%q): expected error", b)
		}
	}
}

func TestBleUuid128JSON(t *testing.T) {
	u, _ := ParseUuid128("0c08c213-98ed-4e43-a499-7e1137c39567")

	b, err := json.Marshal(&u)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != `"0c08c213-98ed-4e43-a499-7e1137c39567"` {
		t.Fatalf("Marshal = %s", b)
	}

	var got BleUuid128
	if err := json.Unmarshal(b, &got); err != nil || got != u {
		t.Fatalf("Unmarshal = % x, %v", got, err)
	}
	if err := json.Unmarshal([]byte(`"bad"`), &got); err == nil {
		t.Fatalf("Unmarshal(bad): expected error")
	}
}

func TestParseUuid(t *testing.T) {
	u, err := ParseUuid("0x2902")
	if err != nil {
		t.Fatalf("ParseUuid: %v", err)
	}
	if u.U16 != CccdUuid || u.U128 != (BleUuid128{}) {
		t.Fatalf("ParseUuid(0x2902) = %+v", u)
	}
	if u.String() != "0x2902" {
		t.Fatalf("String() = %q", u.String())
	}

	u, err = ParseUuid(NmpPlainSvcUuid)
	if err != nil {
		t.Fatalf("ParseUuid: %v", err)
	}
	if u.U16 != 0 || u.String() != NmpPlainSvcUuid {
		t.Fatalf("ParseUuid(%s) = %+v", NmpPlainSvcUuid, u)
	}

	if _, err := ParseUuid("not-a-uuid"); err == nil {
		t.Fatalf("expected error")
	}

	if n := NewBleUuid16(0x1801); n.U16 != 0x1801 || n.String() != "0x1801" {
		t.Fatalf("NewBleUuid16 = %+v", n)
	}
}

func TestBleUuidJSON(t *testing.T) {
	u16 := NewBleUuid16(0x2a00)
	b, err := json.Marshal(&u16)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != "10752" {
		t.Fatalf("Marshal(u16) = %s, want 10752", b)
	}

	u128, _ := ParseUuid(OmpUnsecSvcUuid)
	b, err = json.Marshal(&u128)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != `"`+OmpUnsecSvcUuid+`"` {
		t.Fatalf("Marshal(u128) = %s", b)
	}

	tests := []struct {
		in   string
		want BleUuid
	}{
		{"10752", u16},
		{`"0x2a00"`, u16},
		{`"` + OmpUnsecSvcUuid + `"`, u128},
	}
	for _, tt := range tests {
		var got BleUuid
		if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
			t.Errorf("Unmarshal(%s): %v", tt.in, err)
			continue
		}
		if CompareUuids(got, tt.want) != 0 {
			t.Errorf("Unmarshal(%s) = %s, want %s", tt.in, got.String(),
				tt.want.String())
		}
	}

	for _, bad := range []string{`"zzz"`, `70000`, `true`} {
		var got BleUuid
		if err := json.Unmarshal([]byte(bad), &got); err == nil {
			t.Errorf("Unmarshal(%s): expected error", bad)
		}
	}
}

func TestCompareUuids(t *testing.T) {
	a16 := NewBleUuid16(0x1800)
	b16 := NewBleUuid16(0x1801)
	a128, _ := ParseUuid("00000000-0000-0000-0000-000000000001")
	b128, _ := ParseUuid("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		a, b BleUuid
		want int
	}{
		{a16, a16, 0},
		{a16, b16, -1},
		{b16, a16, 1},
		{a128, a128, 0},
		{a128, b128, -1},
		{b128, a128, 1},
		// 16-bit UUIDs sort after 128-bit ones.
		{a16, a128, 1},
		{a128, a16, -1},
	}

	for _, tt := range tests {
		got := CompareUuids(tt.a, tt.b)
		if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
			t.Errorf("CompareUuids(%s, %s) = %d, want sign %d",
				tt.a.String(), tt.b.String(), got, tt.want)
		}
	}
}

func TestCompareChrIds(t *testing.T) {
	s1 := NewBleUuid16(1)
	s2 := NewBleUuid16(2)
	c1 := NewBleUuid16(10)
	c2 := NewBleUuid16(20)

	tests := []struct {
		a, b BleChrId
		want int
	}{
		{BleChrId{s1, c1}, BleChrId{s1, c1}, 0},
		{BleChrId{s1, c1}, BleChrId{s1, c2}, -1},
		{BleChrId{s1, c2}, BleChrId{s1, c1}, 1},
		// Service UUID is the primary key.
		{BleChrId{s1, c2}, BleChrId{s2, c1}, -1},
		{BleChrId{s2, c1}, BleChrId{s1, c2}, 1},
	}

	for _, tt := range tests {
		got := CompareChrIds(tt.a, tt.b)
		if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
			t.Errorf("CompareChrIds(%s, %s) = %d, want sign %d",
				tt.a.String(), tt.b.String(), got, tt.want)
		}
	}

	id := BleChrId{s1, c2}
	if s := id.String(); s != "s=0x0001 c=0x0014" {
		t.Errorf("String() = %q", s)
	}
}

// checkEnum verifies the string and JSON conversions of an enum type
// against its string map.
func checkEnum[T ~int](t *testing.T, m map[T]string,
	toStr func(T) string, fromStr func(string) (T, error), hasJSON bool) {

	t.Helper()

	if len(m) == 0 {
		t.Fatalf("empty string map")
	}

	seen := map[string]bool{}
	for v, name := range m {
		if seen[name] {
			t.Errorf("duplicate name %q", name)
		}
		seen[name] = true

		if got := toStr(v); got != name {
			t.Errorf("ToString(%d) = %q, want %q", int(v), got, name)
		}

		got, err := fromStr(name)
		if err != nil {
			t.Errorf("FromString(%q): %v", name, err)
		} else if got != v {
			t.Errorf("FromString(%q) = %d, want %d", name, int(got), int(v))
		}

		if !hasJSON {
			continue
		}

		b, err := json.Marshal(v)
		if err != nil {
			t.Errorf("Marshal(%d): %v", int(v), err)
			continue
		}
		if want, _ := json.Marshal(name); string(b) != string(want) {
			t.Errorf("Marshal(%d) = %s, want %s", int(v), b, want)
		}

		var u T
		if err := json.Unmarshal(b, &u); err != nil {
			t.Errorf("Unmarshal(%s): %v", b, err)
		} else if u != v {
			t.Errorf("Unmarshal(%s) = %d, want %d", b, int(u), int(v))
		}
	}

	if got := toStr(T(12345)); got != "???" {
		t.Errorf("ToString(unknown) = %q, want ???", got)
	}
	if _, err := fromStr("no-such-name"); err == nil {
		t.Errorf("FromString(unknown): expected error")
	}

	if !hasJSON {
		return
	}

	if b, err := json.Marshal(T(12345)); err != nil ||
		string(b) != `"???"` {

		t.Errorf("Marshal(unknown) = %s, %v", b, err)
	}

	for _, bad := range []string{`"no-such-name"`, `0`, `null`} {
		var u T
		if err := json.Unmarshal([]byte(bad), &u); err == nil {
			t.Errorf("Unmarshal(%s): expected error", bad)
		}
	}
}

func TestEnumStrings(t *testing.T) {
	t.Run("BleAddrType", func(t *testing.T) {
		checkEnum(t, BleAddrTypeStringMap,
			BleAddrTypeToString, BleAddrTypeFromString, true)
	})
	t.Run("BleScanFilterPolicy", func(t *testing.T) {
		checkEnum(t, BleScanFilterPolicyStringMap,
			BleScanFilterPolicyToString, BleScanFilterPolicyFromString, true)
	})
	t.Run("BleAdvEventType", func(t *testing.T) {
		checkEnum(t, BleAdvEventTypeStringMap,
			BleAdvEventTypeToString, BleAdvEventTypeFromString, true)
	})
	t.Run("BleAdvConnMode", func(t *testing.T) {
		checkEnum(t, BleAdvConnModeStringMap,
			BleAdvConnModeToString, BleAdvConnModeFromString, true)
	})
	t.Run("BleAdvDiscMode", func(t *testing.T) {
		checkEnum(t, BleAdvDiscModeStringMap,
			BleAdvDiscModeToString, BleAdvDiscModeFromString, true)
	})
	t.Run("BleAdvFilterPolicy", func(t *testing.T) {
		checkEnum(t, BleAdvFilterPolicyStringMap,
			BleAdvFilterPolicyToString, BleAdvFilterPolicyFromString, true)
	})
	t.Run("BleGattOp", func(t *testing.T) {
		checkEnum(t, BleGattOpStringMap,
			BleGattOpToString, BleGattOpFromString, false)
	})
	t.Run("BleSvcType", func(t *testing.T) {
		checkEnum(t, BleSvcTypeStringMap,
			BleSvcTypeToString, BleSvcTypeFromString, true)
	})
	t.Run("BleSmAction", func(t *testing.T) {
		checkEnum(t, BleSmActionStringMap,
			BleSmActionToString, BleSmActionFromString, true)
	})
	t.Run("BleSmIoCap", func(t *testing.T) {
		checkEnum(t, BleSmIoCapStringMap,
			BleSmIoCapToString, BleSmIoCapFromString, true)
	})
	t.Run("BleSmKeyDist", func(t *testing.T) {
		checkEnum(t, BleSmKeyDistStringMap,
			BleSmKeyDistToString, BleSmKeyDistFromString, true)
	})
	t.Run("BleSmAuthReq", func(t *testing.T) {
		checkEnum(t, BleSmAuthReqStringMap,
			BleSmAuthReqToString, BleSmAuthReqFromString, true)
	})
}

func TestEnumStringMethods(t *testing.T) {
	tests := []struct {
		s    interface{ String() string }
		want string
	}{
		{BLE_SM_ACTION_NUMCMP, "numcmp"},
		{BLE_SM_IO_CAP_NO_IO, "no_io"},
		{BLE_SM_KEY_DIST_ID, "id"},
		{BleSmAction(99), "???"},
	}

	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}

	for v, name := range BleSmAuthReqStringMap {
		if got := v.String(); got != name {
			t.Errorf("BleSmAuthReq(%d).String() = %q, want %q",
				int(v), got, name)
		}
	}
}
