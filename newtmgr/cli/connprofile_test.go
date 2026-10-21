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
	"testing"

	"mynewt.apache.org/newtmgr/newtmgr/config"
)

func TestConnProfileParseVars(t *testing.T) {
	tests := []struct {
		name  string
		vdefs []string
		want  config.ConnProfile
	}{
		{"none", nil, config.ConnProfile{Name: "p"}},
		{"type", []string{"type=serial"},
			config.ConnProfile{Name: "p", Type: config.CONN_TYPE_SERIAL_PLAIN}},
		{"type and connstring",
			[]string{"type=oic_ble", "connstring=peer_name=x,conn_timeout=5"},
			config.ConnProfile{
				Name:       "p",
				Type:       config.CONN_TYPE_BLL_OIC,
				ConnString: "peer_name=x,conn_timeout=5",
			}},
		{"empty connstring", []string{"connstring="},
			config.ConnProfile{Name: "p"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := &config.ConnProfile{Name: "p"}
			if err := connProfileParseVars(cp, tt.vdefs); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *cp != tt.want {
				t.Errorf("got %+v, want %+v", *cp, tt.want)
			}
		})
	}
}

func TestConnProfileParseVarsErrors(t *testing.T) {
	tests := []struct {
		vdefs []string
		err   string
	}{
		// Regression: a token without '=' used to panic.
		{[]string{"type"},
			"Invalid variable definition (expected key=value): type"},
		{[]string{"type=serial", "connstring"},
			"Invalid variable definition (expected key=value): connstring"},
		{[]string{""},
			"Invalid variable definition (expected key=value): "},
		{[]string{"type=bogus"}, "Invalid connection type: bogus"},
		{[]string{"type="}, "Invalid connection type: "},
		{[]string{"foo=bar"}, "Unknown variable foo"},
	}

	for _, tt := range tests {
		cp := config.NewConnProfile()
		err := connProfileParseVars(cp, tt.vdefs)
		if err == nil {
			t.Errorf("%q: expected error", tt.vdefs)
			continue
		}
		if err.Error() != tt.err {
			t.Errorf("%q: error = %q, want %q", tt.vdefs, err.Error(), tt.err)
		}
	}
}
