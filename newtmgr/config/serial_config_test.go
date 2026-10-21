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
)

func TestParseSerialConnString(t *testing.T) {
	setTimeout(t, 3.5)

	tests := []struct {
		name string
		cs   string
		dev  string
		baud int
		mtu  int
	}{
		// Defaults: baud 115200 (set by the parser), mtu 128 (from
		// nmserial.NewXportCfg).
		{"empty", "", "", 115200, 128},
		{"old style dev only", "/dev/ttyUSB0", "/dev/ttyUSB0", 115200, 128},
		{"dev key", "dev=/dev/ttyACM0", "/dev/ttyACM0", 115200, 128},
		{"windows dev", "COM3", "COM3", 115200, 128},
		{"all keys", "dev=/dev/x,baud=9600,mtu=256", "/dev/x", 9600, 256},
		{"order independent", "mtu=128,baud=1000000,dev=/dev/y",
			"/dev/y", 1000000, 128},
		{"value with equals", "dev=/dev/a=b", "/dev/a=b", 115200, 128},
		{"last key wins", "baud=1,baud=2", "", 2, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := ParseSerialConnString(tt.cs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc.DevPath != tt.dev {
				t.Errorf("DevPath = %q, want %q", sc.DevPath, tt.dev)
			}
			if sc.Baud != tt.baud {
				t.Errorf("Baud = %d, want %d", sc.Baud, tt.baud)
			}
			if sc.Mtu != tt.mtu {
				t.Errorf("Mtu = %d, want %d", sc.Mtu, tt.mtu)
			}
			if sc.ReadTimeout != 3500*time.Millisecond {
				t.Errorf("ReadTimeout = %v, want 3.5s", sc.ReadTimeout)
			}
		})
	}
}

func TestParseSerialConnStringErrors(t *testing.T) {
	tests := []struct {
		cs  string
		err string
	}{
		{"baud=fast",
			"Invalid serial connstring; Invalid baud: fast"},
		{"baud=",
			"Invalid serial connstring; Invalid baud: "},
		{"mtu=big",
			"Invalid serial connstring; Invalid mtu: big"},
		{"dev=/dev/x,speed=9600",
			"Invalid serial connstring; Unrecognized key: speed"},
		{"=foo",
			"Invalid serial connstring; Unrecognized key: "},
	}

	for _, tt := range tests {
		t.Run(tt.cs, func(t *testing.T) {
			_, err := ParseSerialConnString(tt.cs)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err.Error() != tt.err {
				t.Errorf("error = %q, want %q", err.Error(), tt.err)
			}
		})
	}
}
