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

package mtech_lora

import (
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0011223344556677", "0011223344556677"},
		{"00-11-22-33-44-55-66-77", "0011223344556677"},
		{"00:11:22:33:44:55:66:77", "0011223344556677"},
		{"00:11-22:33-44:55-66:77", "0011223344556677"},
	}

	for _, tt := range tests {
		got, err := NormalizeAddr(tt.in)
		if err != nil {
			t.Errorf("NormalizeAddr(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeAddr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	bad := []string{
		"",
		"00112233445566",
		"001122334455667788",
		"00-11-22-33-44-55-66",
		"00:11:22:33:44:55:66",
	}
	for _, s := range bad {
		if got, err := NormalizeAddr(s); err == nil {
			t.Errorf("NormalizeAddr(%q) = %q, expected error", s, got)
		}
	}
}

func TestDenormalizeAddr(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0011223344556677", "00-11-22-33-44-55-66-77"},
		{"aabbccddeeff0011", "aa-bb-cc-dd-ee-ff-00-11"},
		{"", "<invalid>"},
		{"001122", "<invalid>"},
		{"00-11-22-33-44-55-66-77", "<invalid>"},
	}

	for _, tt := range tests {
		if got := DenormalizeAddr(tt.in); got != tt.want {
			t.Errorf("DenormalizeAddr(%q) = %q, want %q", tt.in, got,
				tt.want)
		}
	}
}

func TestNormalizeDenormalizeRoundTrip(t *testing.T) {
	addr := "0123456789abcdef"
	n, err := NormalizeAddr(DenormalizeAddr(addr))
	if err != nil || n != addr {
		t.Fatalf("round trip = %q, %v; want %q", n, err, addr)
	}
}
