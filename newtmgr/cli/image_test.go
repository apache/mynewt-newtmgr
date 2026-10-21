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

	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

func TestImageFlagsStr(t *testing.T) {
	tests := []struct {
		img  nmp.ImageStateEntry
		want string
	}{
		{nmp.ImageStateEntry{}, ""},
		{nmp.ImageStateEntry{Active: true}, "active"},
		{nmp.ImageStateEntry{Active: true, Confirmed: true},
			"active confirmed"},
		{nmp.ImageStateEntry{Pending: true, Permanent: true},
			"pending permanent"},
		{nmp.ImageStateEntry{Permanent: true, Pending: true,
			Confirmed: true, Active: true, Bootable: true},
			"active confirmed pending permanent"},
		{nmp.ImageStateEntry{Bootable: true}, ""},
	}

	for _, tt := range tests {
		if got := imageFlagsStr(tt.img); got != tt.want {
			t.Errorf("imageFlagsStr(%+v) = %q, want %q", tt.img, got,
				tt.want)
		}
	}
}
