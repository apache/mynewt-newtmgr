//go:build !windows

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

package bll

import (
	"errors"
	"runtime"
	"testing"

	"github.com/JuulLabs-OSS/ble"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

// fakeClient is a ble.Client whose ExchangeMTU reports a fixed MTU.  Any
// other method panics (nil embedded interface).
type fakeClient struct {
	ble.Client
	mtu   int
	err   error
	calls []int
}

func (c *fakeClient) ExchangeMTU(rxMTU int) (int, error) {
	c.calls = append(c.calls, rxMTU)
	return c.mtu, c.err
}

func setMtuOverride(t *testing.T, mtu int) {
	t.Helper()
	old := nmutil.MtuOverride
	nmutil.MtuOverride = mtu
	t.Cleanup(func() { nmutil.MtuOverride = old })
}

func TestExchangeMtu(t *testing.T) {
	tests := []struct {
		name     string
		override int
		reported int
		want     uint16
		wantErr  string
	}{
		{"reported", 0, 247, 247, ""},
		{"reported default", 0, 23, 23, ""},
		{"too small clamped", 0, 17, 185, ""},
		{"override", 100, 247, 100, ""},
		{"override small reported", 100, 17, 100, ""},
		{"override minimum", 23, 247, 23, ""},
		{"override too small", 20, 247, 0, "MTU should be at least 23"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "darwin" &&
				tt.reported <= bledefs.BLE_ATT_MTU_DFLT {
				// macOS retries with one second sleeps.
				t.Skip("skipping macOS MTU retry path")
			}
			setMtuOverride(t, tt.override)

			cln := &fakeClient{mtu: tt.reported}
			mtu, err := exchangeMtu(cln, 512)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mtu != tt.want {
				t.Errorf("mtu = %d, want %d", mtu, tt.want)
			}
			if len(cln.calls) != 1 || cln.calls[0] != 512 {
				t.Errorf("ExchangeMTU calls = %v, want [512]", cln.calls)
			}
		})
	}
}

func TestExchangeMtuError(t *testing.T) {
	setMtuOverride(t, 100)

	want := errors.New("exchange failed")
	cln := &fakeClient{mtu: 247, err: want}
	mtu, err := exchangeMtu(cln, 256)
	if err != want {
		t.Errorf("err = %v, want %v", err, want)
	}
	if mtu != 0 {
		t.Errorf("mtu = %d, want 0", mtu)
	}
	if len(cln.calls) != 1 {
		t.Errorf("ExchangeMTU called %d times, want 1", len(cln.calls))
	}
}

const (
	testSvcUuid = "8d53dc1d-1db7-4cd3-868b-8a527460aa84"
	testChrUuid = "da2e7828-fbce-4e01-ae9e-261174997c48"
)

func mustParseUuid(t *testing.T, s string) bledefs.BleUuid {
	t.Helper()
	u, err := bledefs.ParseUuid(s)
	if err != nil {
		t.Fatalf("ParseUuid(%q): %v", s, err)
	}
	return u
}

func TestUuidFromBllUuid(t *testing.T) {
	u, err := UuidFromBllUuid(ble.UUID16(0x180a))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != (bledefs.BleUuid{U16: 0x180a}) {
		t.Errorf("16-bit: got %+v", u)
	}

	for _, s := range []string{testSvcUuid, testChrUuid} {
		u, err := UuidFromBllUuid(ble.MustParse(s))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", s, err)
		}
		if want := mustParseUuid(t, s); u != want {
			t.Errorf("%s: got %+v, want %+v", s, u, want)
		}
	}

	for _, b := range []ble.UUID{nil, {1}, {1, 2, 3, 4}} {
		if _, err := UuidFromBllUuid(b); err == nil {
			t.Errorf("%#v: expected error", b)
		}
	}
}

func TestFindChr(t *testing.T) {
	smpChr := &ble.Characteristic{UUID: ble.MustParse(testChrUuid)}
	otherChr := &ble.Characteristic{UUID: ble.UUID16(0x2a29)}

	profile := &ble.Profile{
		Services: []*ble.Service{
			{
				UUID:            ble.UUID16(0x180a),
				Characteristics: []*ble.Characteristic{otherChr},
			},
			{
				UUID: ble.MustParse(testSvcUuid),
				Characteristics: []*ble.Characteristic{
					{UUID: ble.UUID16(0x2a00)},
					smpChr,
				},
			},
		},
	}

	svc := mustParseUuid(t, testSvcUuid)
	chr := mustParseUuid(t, testChrUuid)

	tests := []struct {
		name string
		id   bledefs.BleChrId
		want *ble.Characteristic
	}{
		{"128-bit", bledefs.BleChrId{SvcUuid: svc, ChrUuid: chr}, smpChr},
		{"16-bit", bledefs.BleChrId{
			SvcUuid: bledefs.BleUuid{U16: 0x180a},
			ChrUuid: bledefs.BleUuid{U16: 0x2a29},
		}, otherChr},
		{"chr in other service", bledefs.BleChrId{
			SvcUuid: bledefs.BleUuid{U16: 0x180a},
			ChrUuid: chr,
		}, nil},
		{"unknown service", bledefs.BleChrId{
			SvcUuid: bledefs.BleUuid{U16: 0x1800},
			ChrUuid: bledefs.BleUuid{U16: 0x2a00},
		}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.id
			c, err := findChr(profile, &id)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c != tt.want {
				t.Errorf("got %+v, want %+v", c, tt.want)
			}
		})
	}
}

func TestFindChrErrors(t *testing.T) {
	if _, err := findChr(&ble.Profile{}, nil); err == nil {
		t.Errorf("nil chrId: expected error")
	}

	bad := &ble.Profile{
		Services: []*ble.Service{{UUID: ble.UUID{1, 2, 3}}},
	}
	id := &bledefs.BleChrId{SvcUuid: bledefs.BleUuid{U16: 0x180a}}
	if _, err := findChr(bad, id); err == nil {
		t.Errorf("invalid service UUID: expected error")
	}

	badChr := &ble.Profile{
		Services: []*ble.Service{{
			UUID:            ble.UUID16(0x180a),
			Characteristics: []*ble.Characteristic{{UUID: ble.UUID{1}}},
		}},
	}
	if _, err := findChr(badChr, id); err == nil {
		t.Errorf("invalid characteristic UUID: expected error")
	}
}
