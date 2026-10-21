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

package config

import (
	"testing"
	"time"

	"github.com/JuulLabs-OSS/ble"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type fakeAddr string

func (a fakeAddr) String() string { return string(a) }

// fakeAdv is a stub ble.Advertisement; only the methods used by the
// bll advertisement filters are implemented.
type fakeAdv struct {
	ble.Advertisement
	name string
	addr string
}

func (a *fakeAdv) LocalName() string { return a.name }
func (a *fakeAdv) Addr() ble.Addr    { return fakeAddr(a.addr) }

func TestParseBllConnString(t *testing.T) {
	setTimeout(t, 4)
	setHciIdx(t, 1)

	tests := []struct {
		name string
		cs   string
		want BllConfig
	}{
		{"peer_name", "peer_name=mydev",
			BllConfig{PeerName: "mydev", ConnTimeout: 4, HciIdx: 1}},
		{"all keys",
			"ctlr_name=hci0,own_addr_type=random,peer_id=aa:bb," +
				"peer_name=x=y,conn_timeout=23.54",
			BllConfig{
				CtlrName:    "hci0",
				OwnAddrType: bledefs.BLE_ADDR_TYPE_RANDOM,
				PeerId:      "aa:bb",
				PeerName:    "x=y",
				ConnTimeout: 23.54,
				HciIdx:      1,
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc, err := ParseBllConnString(tt.cs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *bc != tt.want {
				t.Errorf("got %+v, want %+v", *bc, tt.want)
			}
		})
	}
}

func TestParseBllConnStringErrors(t *testing.T) {
	tests := []struct {
		cs  string
		err string
	}{
		{"peer_name",
			"Invalid BLE connstring; expected comma-separated " +
				"key=value pairs; no '=' in: peer_name"},
		{"peer_name=a,",
			"Invalid BLE connstring; expected comma-separated " +
				"key=value pairs; no '=' in: "},
		{"own_addr_type=bad",
			"Invalid BLE connstring; Invalid own_addr_type: bad"},
		{"conn_timeout=soon",
			"Invalid BLE connstring; Invalid conn_timeout: soon"},
		{"peer_addr=01:02:03:04:05:06",
			"Invalid BLE connstring; Unrecognized key: peer_addr"},
	}

	for _, tt := range tests {
		t.Run(tt.cs, func(t *testing.T) {
			bc, err := ParseBllConnString(tt.cs)
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

func TestBuildBllSesnCfg(t *testing.T) {
	type advCase struct {
		adv  fakeAdv
		want bool
	}

	tests := []struct {
		name       string
		devName    string
		bc         BllConfig
		advs       []advCase
		wantPeerNm string
	}{
		{
			name: "peer_name",
			bc:   BllConfig{PeerName: "dev1", PeerId: "11:22", ConnTimeout: 1.5},
			advs: []advCase{
				{fakeAdv{name: "dev1", addr: "00:00"}, true},
				{fakeAdv{name: "dev2", addr: "11:22"}, false},
				{fakeAdv{name: "", addr: ""}, false},
			},
			wantPeerNm: "dev1",
		},
		{
			name: "peer_id",
			bc:   BllConfig{PeerId: "11:22", ConnTimeout: 1.5},
			advs: []advCase{
				{fakeAdv{name: "dev1", addr: "11:22"}, true},
				{fakeAdv{name: "11:22", addr: "33:44"}, false},
			},
		},
		{
			name:    "device name overrides",
			devName: "global",
			bc:      BllConfig{PeerName: "dev1", PeerId: "11:22", ConnTimeout: 1.5},
			advs: []advCase{
				{fakeAdv{name: "global", addr: "00:00"}, true},
				{fakeAdv{name: "dev1", addr: "11:22"}, false},
			},
			wantPeerNm: "global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDeviceName(t, tt.devName)
			setBleWriteRsp(t, true)

			bc := tt.bc
			sc, err := BuildBllSesnCfg(&bc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if bc.PeerName != tt.wantPeerNm {
				t.Errorf("PeerName = %q, want %q", bc.PeerName, tt.wantPeerNm)
			}
			if sc.ConnTimeout != 1500*time.Millisecond {
				t.Errorf("ConnTimeout = %v, want 1.5s", sc.ConnTimeout)
			}
			if !sc.WriteRsp {
				t.Errorf("WriteRsp = false, want true")
			}
			if sc.PreferredMtu != 512 || sc.ConnTries != 3 {
				t.Errorf("defaults not kept: %+v", sc)
			}
			if sc.AdvFilter == nil {
				t.Fatalf("AdvFilter is nil")
			}
			for i, ac := range tt.advs {
				adv := ac.adv
				if got := sc.AdvFilter(&adv); got != ac.want {
					t.Errorf("adv %d (%+v): filter = %v, want %v",
						i, ac.adv, got, ac.want)
				}
			}
		})
	}
}

func TestBuildBllSesnCfgNoPeer(t *testing.T) {
	setDeviceName(t, "")

	_, err := BuildBllSesnCfg(&BllConfig{CtlrName: "hci0"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "bll session lacks a peer specifier" {
		t.Errorf("error = %q", err.Error())
	}
}
