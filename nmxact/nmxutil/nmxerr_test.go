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

package nmxutil

import (
	"errors"
	"testing"
)

func TestErrorPredicates(t *testing.T) {
	type pred struct {
		name string
		fn   func(error) bool
	}

	preds := []pred{
		{"IsRspTimeout", IsRspTimeout},
		{"IsBleSesnDisconnect", IsBleSesnDisconnect},
		{"IsSesnAlreadyOpen", IsSesnAlreadyOpen},
		{"IsSesnClosed", IsSesnClosed},
		{"IsScanTmo", IsScanTmo},
		{"IsXport", IsXport},
		{"IsBleHost", IsBleHost},
		{"IsAlready", IsAlready},
		{"IsBleSecurity", IsBleSecurity},
	}

	// errs[i] must satisfy exactly preds[i].
	errs := []error{
		FmtRspTimeoutError("tmo %d", 1),
		NewBleSesnDisconnectError(0x13, "disc"),
		NewSesnAlreadyOpenError("open"),
		NewSesnClosedError("closed"),
		NewScanTmoError("scan"),
		NewXportError("xport"),
		FmtBleHostError(5, "host %s", "err"),
		NewAlreadyError("already"),
		NewBleSecurityError("sec"),
	}

	for i, err := range errs {
		for j, p := range preds {
			want := i == j
			if got := p.fn(err); got != want {
				t.Errorf("%s(%T) = %v, want %v", p.name, err, got, want)
			}
		}
	}

	for _, p := range preds {
		if p.fn(nil) {
			t.Errorf("%s(nil) = true", p.name)
		}
		if p.fn(errors.New("plain")) {
			t.Errorf("%s(plain error) = true", p.name)
		}
	}
}

func TestErrorText(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{FmtRspTimeoutError("a=%d b=%s", 1, "x"), "a=1 b=x"},
		{NewBleSesnDisconnectError(1, "disc"), "disc"},
		{NewSesnAlreadyOpenError("open"), "open"},
		{NewSesnClosedError("closed"), "closed"},
		{NewScanTmoError("scan"), "scan"},
		{NewXportError("xport"), "xport"},
		{FmtBleHostError(1, "%s-%d", "h", 2), "h-2"},
		{NewAlreadyError("already"), "already"},
		{NewBleSecurityError("sec"), "sec"},
	}

	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("%T.Error() = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestToBleHost(t *testing.T) {
	e := NewBleHostError(0x205, "x")
	if got := ToBleHost(e); got != e {
		t.Fatalf("ToBleHost() = %v, want %v", got, e)
	}
	if got := ToBleHost(e); got.Status != 0x205 {
		t.Fatalf("Status = %#x, want 0x205", got.Status)
	}
	if got := ToBleHost(NewXportError("x")); got != nil {
		t.Fatalf("ToBleHost(xport) = %v, want nil", got)
	}
	if got := ToBleHost(nil); got != nil {
		t.Fatalf("ToBleHost(nil) = %v, want nil", got)
	}
}

func TestToBleSecurity(t *testing.T) {
	e := NewBleSecurityError("x")
	if got := ToBleSecurity(e); got != e {
		t.Fatalf("ToBleSecurity() = %v, want %v", got, e)
	}
	if got := ToBleSecurity(NewBleHostError(1, "x")); got != nil {
		t.Fatalf("ToBleSecurity(host) = %v, want nil", got)
	}
	if got := ToBleSecurity(nil); got != nil {
		t.Fatalf("ToBleSecurity(nil) = %v, want nil", got)
	}
}

func TestBleSesnDisconnectReason(t *testing.T) {
	e := NewBleSesnDisconnectError(0x13, "disc")
	if e.Reason != 0x13 {
		t.Fatalf("Reason = %#x, want 0x13", e.Reason)
	}
}
