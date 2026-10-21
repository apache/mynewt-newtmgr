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

package nmble

import (
	"errors"
	"testing"
	"time"
)

// fakePreemptable records preemption requests.
type fakePreemptable struct {
	preempted chan struct{}
}

func newFakePreemptable() *fakePreemptable {
	return &fakePreemptable{preempted: make(chan struct{}, 16)}
}

func (p *fakePreemptable) Preempt() error {
	p.preempted <- struct{}{}
	return nil
}

func (p *fakePreemptable) waitPreempt(t *testing.T) {
	t.Helper()

	select {
	case <-p.preempted:
	case <-time.After(testTimeout):
		t.Fatalf("secondary was not preempted")
	}
}

func newTestMaster(t *testing.T) (*Master, *fakePreemptable) {
	t.Helper()

	m := NewMaster(nil)
	p := newFakePreemptable()
	if err := m.SetSecondary(p); err != nil {
		t.Fatalf("SetSecondary failed: %v", err)
	}
	return &m, p
}

func getMasterState(m *Master) (masterState, int) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.state, len(m.primaries)
}

func checkMasterState(t *testing.T, m *Master, want masterState,
	wantPrimaries int) {

	t.Helper()

	s, n := getMasterState(m)
	if s != want || n != wantPrimaries {
		t.Fatalf("state=%s waiting primaries=%d; want %s, %d",
			s, n, want, wantPrimaries)
	}
}

// Waits until the master reaches the given state with the given number of
// queued primaries; used to know that a goroutine has queued itself.
func waitMasterState(t *testing.T, m *Master, want masterState,
	wantPrimaries int) {

	t.Helper()

	deadline := time.Now().Add(testTimeout)
	for {
		s, n := getMasterState(m)
		if s == want && n == wantPrimaries {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("state=%s waiting primaries=%d; want %s, %d",
				s, n, want, wantPrimaries)
		}
		time.Sleep(time.Millisecond)
	}
}

type acqResult struct {
	id  int
	err error
}

func acquirePrimaryAsync(m *Master, id int, res chan<- acqResult) {
	go func() {
		res <- acqResult{id, m.AcquirePrimary(id)}
	}()
}

func acquireSecondaryAsync(m *Master, id int, res chan<- acqResult) {
	go func() {
		res <- acqResult{id, m.AcquireSecondary()}
	}()
}

func recvAcq(t *testing.T, res <-chan acqResult) acqResult {
	t.Helper()

	select {
	case r := <-res:
		return r
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for master acquisition")
	}
	return acqResult{}
}

func assertNoAcq(t *testing.T, res <-chan acqResult) {
	t.Helper()

	select {
	case r := <-res:
		t.Fatalf("unexpected acquisition: %+v", r)
	default:
	}
}

func TestMasterPrimaryIdle(t *testing.T) {
	m, _ := newTestMaster(t)

	if err := m.AcquirePrimary("a"); err != nil {
		t.Fatalf("AcquirePrimary failed: %v", err)
	}
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 0)

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterSecondaryIdle(t *testing.T) {
	m, _ := newTestMaster(t)

	if err := m.AcquireSecondary(); err != nil {
		t.Fatalf("AcquireSecondary failed: %v", err)
	}
	checkMasterState(t, m, MASTER_STATE_SECONDARY, 0)

	if err := m.SetSecondary(newFakePreemptable()); err == nil {
		t.Errorf("SetSecondary succeeded while secondary active")
	}
	if err := m.AcquireSecondary(); err == nil {
		t.Errorf("second AcquireSecondary succeeded")
	}

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterPrimariesFifo(t *testing.T) {
	m, _ := newTestMaster(t)
	res := make(chan acqResult, 3)

	if err := m.AcquirePrimary(0); err != nil {
		t.Fatal(err)
	}

	for id := 1; id <= 3; id++ {
		acquirePrimaryAsync(m, id, res)
		waitMasterState(t, m, MASTER_STATE_PRIMARY, id)
	}
	assertNoAcq(t, res)

	for id := 1; id <= 3; id++ {
		m.Release()
		r := recvAcq(t, res)
		if r.err != nil || r.id != id {
			t.Fatalf("got %+v, want id %d with nil error", r, id)
		}
		assertNoAcq(t, res)
		checkMasterState(t, m, MASTER_STATE_PRIMARY, 3-id)
	}

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterPrimaryPreemptsSecondary(t *testing.T) {
	m, p := newTestMaster(t)
	res := make(chan acqResult, 1)

	if err := m.AcquireSecondary(); err != nil {
		t.Fatal(err)
	}

	acquirePrimaryAsync(m, 1, res)
	p.waitPreempt(t)
	waitMasterState(t, m, MASTER_STATE_SECONDARY, 1)
	assertNoAcq(t, res)

	// The preempted secondary stops and releases.
	m.Release()
	if r := recvAcq(t, res); r.err != nil || r.id != 1 {
		t.Fatalf("got %+v, want id 1 with nil error", r)
	}
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 0)

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterSecondaryPending(t *testing.T) {
	m, p := newTestMaster(t)
	sec := make(chan acqResult, 1)
	prim := make(chan acqResult, 1)

	if err := m.AcquirePrimary(0); err != nil {
		t.Fatal(err)
	}

	acquireSecondaryAsync(m, 100, sec)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 0)

	// A primary arriving while a secondary is pending goes first.
	acquirePrimaryAsync(m, 1, prim)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 1)

	m.Release()
	if r := recvAcq(t, prim); r.err != nil {
		t.Fatalf("primary got error %v", r.err)
	}
	assertNoAcq(t, sec)
	checkMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 0)

	m.Release()
	if r := recvAcq(t, sec); r.err != nil {
		t.Fatalf("secondary got error %v", r.err)
	}
	checkMasterState(t, m, MASTER_STATE_SECONDARY, 0)

	select {
	case <-p.preempted:
		t.Errorf("secondary preempted although no primary is waiting")
	default:
	}

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterStopWaitingSecondary(t *testing.T) {
	m, _ := newTestMaster(t)
	sec := make(chan acqResult, 1)

	if err := m.AcquirePrimary(0); err != nil {
		t.Fatal(err)
	}

	acquireSecondaryAsync(m, 100, sec)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 0)

	m.StopWaitingSecondary(errors.New("stop"))
	if r := recvAcq(t, sec); r.err == nil {
		t.Fatalf("aborted secondary acquisition succeeded")
	}
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 0)

	// No-op when no secondary is waiting.
	m.StopWaitingSecondary(errors.New("stop"))
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 0)

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterAbort(t *testing.T) {
	m, _ := newTestMaster(t)
	sec := make(chan acqResult, 1)
	prim := make(chan acqResult, 2)

	if err := m.AcquirePrimary(0); err != nil {
		t.Fatal(err)
	}

	acquireSecondaryAsync(m, 100, sec)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 0)
	acquirePrimaryAsync(m, 1, prim)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 1)
	acquirePrimaryAsync(m, 2, prim)
	waitMasterState(t, m, MASTER_STATE_PRIMARY_SECONDARY_PENDING, 2)

	want := errors.New("abort")
	m.Abort(want)

	if r := recvAcq(t, sec); r.err != want {
		t.Errorf("secondary got %v, want %v", r.err, want)
	}
	for i := 0; i < 2; i++ {
		if r := recvAcq(t, prim); r.err != want {
			t.Errorf("primary %d got %v, want %v", r.id, r.err, want)
		}
	}

	// The current owner still holds the master and releases it itself.
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 0)
	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

func TestMasterAbortPreemptsSecondary(t *testing.T) {
	m, p := newTestMaster(t)

	if err := m.AcquireSecondary(); err != nil {
		t.Fatal(err)
	}

	m.Abort(errors.New("abort"))
	p.waitPreempt(t)

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}

// A primary removed from the wait queue must be woken up with the given
// error rather than left blocked in AcquirePrimary forever.
func TestMasterStopWaitingPrimary(t *testing.T) {
	m, _ := newTestMaster(t)
	prim := make(chan acqResult, 2)

	if err := m.AcquirePrimary(0); err != nil {
		t.Fatal(err)
	}

	acquirePrimaryAsync(m, 1, prim)
	waitMasterState(t, m, MASTER_STATE_PRIMARY, 1)
	acquirePrimaryAsync(m, 2, prim)
	waitMasterState(t, m, MASTER_STATE_PRIMARY, 2)

	want := errors.New("stop waiting")
	m.StopWaitingPrimary(1, want)
	if r := recvAcq(t, prim); r.id != 1 || r.err != want {
		t.Fatalf("got %+v, want id 1 with error %v", r, want)
	}
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 1)

	// Unknown token: no-op.
	m.StopWaitingPrimary(42, want)
	checkMasterState(t, m, MASTER_STATE_PRIMARY, 1)

	// The remaining waiter is still served.
	m.Release()
	if r := recvAcq(t, prim); r.id != 2 || r.err != nil {
		t.Fatalf("got %+v, want id 2 with nil error", r)
	}

	m.Release()
	checkMasterState(t, m, MASTER_STATE_IDLE, 0)
}
