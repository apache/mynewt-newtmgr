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

	"mynewt.apache.org/newtmgr/nmxact/lora"
	"mynewt.apache.org/newtmgr/nmxact/mtech_lora"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func TestParseMtechLoraConnString(t *testing.T) {
	def := mtech_lora.LoraConfig{Port: lora.COAP_LORA_PORT}

	tests := []struct {
		name string
		cs   string
		want mtech_lora.LoraConfig
	}{
		{"empty", "", def},
		{"addr", "addr=01:02:03:04:05:06:07:08",
			mtech_lora.LoraConfig{
				Addr: "01:02:03:04:05:06:07:08",
				Port: lora.COAP_LORA_PORT,
			}},
		{"all keys", "addr=a=b,segsz=64,confirmedtx=true,port=10",
			mtech_lora.LoraConfig{
				Addr:        "a=b",
				SegSz:       64,
				ConfirmedTx: true,
				Port:        10,
			}},
		{"confirmedtx numeric", "confirmedtx=1",
			mtech_lora.LoraConfig{
				ConfirmedTx: true,
				Port:        lora.COAP_LORA_PORT,
			}},
		{"port bounds", "port=255",
			mtech_lora.LoraConfig{Port: 255}},
		{"port zero", "port=0",
			mtech_lora.LoraConfig{Port: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc, err := ParseMtechLoraConnString(tt.cs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *mc != tt.want {
				t.Errorf("got %+v, want %+v", *mc, tt.want)
			}
		})
	}
}

func TestParseMtechLoraConnStringErrors(t *testing.T) {
	tests := []struct {
		cs  string
		err string
	}{
		{"addr",
			"expected comma-separated key=value pairs; no '=' in: addr"},
		{"addr=x,",
			"expected comma-separated key=value pairs; no '=' in: "},
		{"segsz=big", "Invalid SegSz: big"},
		{"confirmedtx=maybe", "Invalid confirmedtx: maybe"},
		{"port=256", "Invalid port number: 256"},
		{"port=-1", "Invalid port number: -1"},
		{"port=x", "Invalid port number: x"},
		{"foo=1", "Unrecognized key: foo"},
	}

	for _, tt := range tests {
		t.Run(tt.cs, func(t *testing.T) {
			_, err := ParseMtechLoraConnString(tt.cs)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err.Error() != tt.err {
				t.Errorf("error = %q, want %q", err.Error(), tt.err)
			}
		})
	}
}

func TestFillMtechLoraSesnCfg(t *testing.T) {
	mc := &mtech_lora.LoraConfig{
		Addr:        "aa",
		SegSz:       32,
		ConfirmedTx: true,
		Port:        7,
	}

	t.Run("from config", func(t *testing.T) {
		setDeviceName(t, "")
		var sc sesn.SesnCfg
		if err := FillMtechLoraSesnCfg(mc, &sc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := sesn.SesnCfgLora{
			Addr: "aa", SegSz: 32, ConfirmedTx: true, Port: 7,
		}
		if sc.Lora != want {
			t.Errorf("got %+v, want %+v", sc.Lora, want)
		}
	})

	t.Run("device name overrides addr", func(t *testing.T) {
		setDeviceName(t, "bb")
		var sc sesn.SesnCfg
		if err := FillMtechLoraSesnCfg(mc, &sc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.Lora.Addr != "bb" {
			t.Errorf("Addr = %q, want \"bb\"", sc.Lora.Addr)
		}
		if mc.Addr != "aa" {
			t.Errorf("config modified: Addr = %q", mc.Addr)
		}
	})
}
