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

package cli

import (
	"strings"
	"testing"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

func TestLogShowParseArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want logShowCfg
	}{
		{"none", nil, logShowCfg{}},
		{"name", []string{"reboot_log"}, logShowCfg{Name: "reboot_log"}},
		{"index", []string{"log", "10"}, logShowCfg{Name: "log", Index: 10}},
		{"hex index", []string{"log", "0x10"},
			logShowCfg{Name: "log", Index: 16}},
		{"max index", []string{"log", "4294967295"},
			logShowCfg{Name: "log", Index: 0xffffffff}},
		{"last", []string{"log", "last"},
			logShowCfg{Name: "log", Timestamp: -1}},
		{"last ignores timestamp", []string{"log", "last", "junk"},
			logShowCfg{Name: "log", Timestamp: -1}},
		{"index and timestamp", []string{"log", "3", "123456"},
			logShowCfg{Name: "log", Index: 3, Timestamp: 123456}},
		{"negative timestamp", []string{"log", "3", "-7"},
			logShowCfg{Name: "log", Index: 3, Timestamp: -7}},
		{"extra args ignored", []string{"log", "1", "2", "3"},
			logShowCfg{Name: "log", Index: 1, Timestamp: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := logShowParseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *cfg != tt.want {
				t.Errorf("got %+v, want %+v", *cfg, tt.want)
			}
		})
	}
}

func TestLogShowParseArgsErrors(t *testing.T) {
	tests := []struct {
		args []string
		err  string
	}{
		{[]string{"log", "4294967296"}, "index out of range"},
		{[]string{"log", "-1"}, ""},
		{[]string{"log", "abc"}, ""},
		{[]string{"log", "1", "ts"}, ""},
	}

	for _, tt := range tests {
		cfg, err := logShowParseArgs(tt.args)
		if err == nil {
			t.Errorf("%v: expected error, got %+v", tt.args, cfg)
			continue
		}
		if tt.err != "" && err.Error() != tt.err {
			t.Errorf("%v: error = %q, want %q", tt.args, err.Error(), tt.err)
		}
	}
}

func TestLogCborMsgText(t *testing.T) {
	b, err := nmxutil.EncodeCbor(map[string]interface{}{
		"msg": "hello",
		"n":   7,
		"neg": -1,
		"l":   []interface{}{1, "x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := logCborMsgText(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// encoding/json sorts map keys.
	want := `{"l":[1,"x"],"msg":"hello","n":7,"neg":-1}`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestLogCborMsgTextInvalid(t *testing.T) {
	// Not a map.
	if _, err := logCborMsgText([]byte{0x01}); err == nil {
		t.Errorf("expected error for non-map CBOR")
	}

	// Truncated map.
	_, err := logCborMsgText([]byte{0xa1, 0x61})
	if err == nil || !strings.HasPrefix(err.Error(), "failure decoding cbor") {
		t.Errorf("unexpected error: %v", err)
	}
}
