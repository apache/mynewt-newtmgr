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

package config

import (
	"testing"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func mustParseBleAddr(t *testing.T, s string) bledefs.BleAddr {
	t.Helper()
	a, err := bledefs.ParseBleAddr(s)
	if err != nil {
		t.Fatalf("ParseBleAddr(%q): %v", s, err)
	}
	return a
}

func TestParseBleConnStringDefaults(t *testing.T) {
	setTimeout(t, 7.25)
	setHciIdx(t, 2)

	bc, err := ParseBleConnString("peer_name=dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := BleConfig{
		PeerAddrType: bledefs.BLE_ADDR_TYPE_PUBLIC,
		PeerName:     "dev",
		OwnAddrType:  bledefs.BLE_ADDR_TYPE_RANDOM,
		ConnTimeout:  7.25,
		BlehostdPath: "blehostd",
		HciIdx:       2,
	}
	if *bc != want {
		t.Errorf("got %+v, want %+v", *bc, want)
	}
}

func TestParseBleConnStringKeys(t *testing.T) {
	setHciIdx(t, 0)

	cs := "peer_addr_type=random,peer_addr=01:23:45:67:89:AB," +
		"peer_name=a=b,own_addr_type=rpa_pub,own_addr=ff:ee:dd:cc:bb:aa," +
		"bhd_path=/opt/blehostd,ctlr_path=/dev/ttyUSB1"
	bc, err := ParseBleConnString(cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.PeerAddrType != bledefs.BLE_ADDR_TYPE_RANDOM {
		t.Errorf("PeerAddrType = %v", bc.PeerAddrType)
	}
	if bc.PeerAddr != mustParseBleAddr(t, "01:23:45:67:89:ab") {
		t.Errorf("PeerAddr = %v", bc.PeerAddr)
	}
	if bc.PeerName != "a=b" {
		t.Errorf("PeerName = %q, want \"a=b\"", bc.PeerName)
	}
	if bc.OwnAddrType != bledefs.BLE_ADDR_TYPE_RPA_PUB {
		t.Errorf("OwnAddrType = %v", bc.OwnAddrType)
	}
	if bc.OwnAddr != mustParseBleAddr(t, "ff:ee:dd:cc:bb:aa") {
		t.Errorf("OwnAddr = %v", bc.OwnAddr)
	}
	if bc.BlehostdPath != "/opt/blehostd" {
		t.Errorf("BlehostdPath = %q", bc.BlehostdPath)
	}
	if bc.ControllerPath != "/dev/ttyUSB1" {
		t.Errorf("ControllerPath = %q", bc.ControllerPath)
	}
}

func TestParseBleConnStringErrors(t *testing.T) {
	tests := []struct {
		cs  string
		err string
	}{
		{"",
			"Invalid BLE connstring; expected comma-separated " +
				"key=value pairs; no '=' in: "},
		{"peer_name=x,bogus",
			"Invalid BLE connstring; expected comma-separated " +
				"key=value pairs; no '=' in: bogus"},
		{"peer_addr_type=foo",
			"Invalid BLE connstring; Invalid peer_addr_type: foo"},
		{"own_addr_type=",
			"Invalid BLE connstring; Invalid own_addr_type: "},
		{"peer_addr=01:02",
			"Invalid BLE connstring; Invalid peer_addr; " +
				"invalid BLE addr string: 01:02"},
		{"own_addr=1:2:3:4:5",
			"Invalid BLE connstring; Invalid own_addr; " +
				"invalid BLE addr string: 1:2:3:4:5"},
		{"foo=bar",
			"Invalid BLE connstring; Unrecognized key: foo"},
	}

	for _, tt := range tests {
		t.Run(tt.cs, func(t *testing.T) {
			bc, err := ParseBleConnString(tt.cs)
			if err == nil {
				t.Fatalf("expected error, got %+v", bc)
			}
			if bc != nil {
				t.Errorf("expected nil config on error")
			}
			if err.Error() != tt.err {
				t.Errorf("error = %q, want %q", err.Error(), tt.err)
			}
		})
	}
}
