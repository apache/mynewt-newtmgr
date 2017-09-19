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
	"fmt"
	"sync"
	"time"
)

// Blocks a variable number of waiters until Unblock() is called.  Subsequent
// waiters are unblocked until the next call to Block().
type Blocker struct {
	ch  chan struct{}
	mtx sync.Mutex
	val interface{}
}

func (b *Blocker) unblockNoLock(val interface{}) {
	if b.ch != nil {
		b.val = val
		close(b.ch)
		b.ch = nil
	}
}

func (b *Blocker) startNoLock() {
	if b.ch == nil {
		b.ch = make(chan struct{})
	}
}

func (b *Blocker) Started() bool {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	return b.ch != nil
}

func (b *Blocker) Wait(timeout time.Duration, stopChan <-chan struct{}) (
	interface{}, error) {

	b.mtx.Lock()
	ch := b.ch
	b.mtx.Unlock()

	if ch == nil {
		return b.val, nil
	}

	if stopChan == nil {
		stopChan = make(chan struct{})
	}

	timer := time.NewTimer(timeout)
	select {
	case <-ch:
		StopAndDrainTimer(timer)
		return b.val, nil
	case <-timer.C:
		return nil, fmt.Errorf("timeout after %s", timeout.String())
	case <-stopChan:
		return nil, fmt.Errorf("aborted")
	}
}

func (b *Blocker) Start() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.startNoLock()
}

func (b *Blocker) Unblock(val interface{}) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.unblockNoLock(val)
}

func (b *Blocker) UnblockAndRestart(val interface{}) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.unblockNoLock(val)
	b.startNoLock()
}
