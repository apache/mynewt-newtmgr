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

package nmutil

import (
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestErrorCausedBy(t *testing.T) {
	root := errors.New("root")
	other := errors.New("other")

	tests := []struct {
		name  string
		err   error
		cause error
		want  bool
	}{
		{"same", root, root, true},
		{"nil both", nil, nil, true},
		{"nil err", nil, root, false},
		{"unrelated", other, root, false},
		{"wrapped", errors.Wrap(root, "ctx"), root, true},
		{"wrapped twice", errors.Wrap(errors.WithMessage(root, "a"), "b"),
			root, true},
		{"wrapped other", errors.Wrap(other, "ctx"), root, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorCausedBy(tt.err, tt.cause); got != tt.want {
				t.Errorf("ErrorCausedBy = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTxOptions(t *testing.T) {
	oldTmo, oldTries := Timeout, Tries
	t.Cleanup(func() { Timeout, Tries = oldTmo, oldTries })

	Timeout = 2.5
	Tries = 3
	opt := TxOptions()
	if opt.Timeout != 2500*time.Millisecond {
		t.Errorf("Timeout = %v, want 2.5s", opt.Timeout)
	}
	if opt.Tries != 3 {
		t.Errorf("Tries = %d, want 3", opt.Tries)
	}
}
