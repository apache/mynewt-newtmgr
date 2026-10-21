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
	"strings"
	"testing"
	"time"
)

const syncTestTimeout = 5 * time.Second

func recvSrErr(t *testing.T, ch <-chan error) error {
	t.Helper()

	select {
	case err := <-ch:
		return err
	case <-time.After(syncTestTimeout):
		t.Fatalf("timeout waiting on SingleResource channel")
	}
	return nil
}

func assertSrPending(t *testing.T, ch <-chan error) {
	t.Helper()

	select {
	case err := <-ch:
		t.Fatalf("waiter unexpectedly woken: %v", err)
	default:
	}
}

// Runs fn in a goroutine and returns a channel delivering its result.
func goBool(fn func() bool) <-chan bool {
	c := make(chan bool, 1)
	go func() { c <- fn() }()
	return c
}

func recvBool(t *testing.T, c <-chan bool) bool {
	t.Helper()

	select {
	case v := <-c:
		return v
	case <-time.After(syncTestTimeout):
		t.Fatalf("timeout waiting for result")
	}
	return false
}

func TestSingleResourceImmediate(t *testing.T) {
	s := NewSingleResource()

	if s.Acquired() {
		t.Fatalf("new resource is acquired")
	}
	if err := recvSrErr(t, s.Acquire("a")); err != nil {
		t.Fatalf("Acquire returned %v", err)
	}
	if !s.Acquired() {
		t.Fatalf("resource not acquired")
	}

	if s.Release() {
		t.Errorf("Release reported a hand-off with no waiters")
	}
	if s.Acquired() {
		t.Errorf("resource still acquired after Release")
	}
}

func TestSingleResourceFifo(t *testing.T) {
	s := NewSingleResource()
	recvSrErr(t, s.Acquire("a"))

	chB := s.Acquire("b")
	chC := s.Acquire("c")
	assertSrPending(t, chB)
	assertSrPending(t, chC)

	// Release blocks until the next waiter takes over.
	rel := goBool(s.Release)
	if err := recvSrErr(t, chB); err != nil {
		t.Fatalf("b got %v", err)
	}
	if !recvBool(t, rel) {
		t.Errorf("Release did not report a hand-off")
	}
	assertSrPending(t, chC)
	if !s.Acquired() {
		t.Errorf("resource not acquired by b")
	}

	rel = goBool(s.Release)
	if err := recvSrErr(t, chC); err != nil {
		t.Fatalf("c got %v", err)
	}
	if !recvBool(t, rel) {
		t.Errorf("Release did not report a hand-off")
	}

	if s.Release() {
		t.Errorf("Release reported a hand-off with no waiters")
	}
	if s.Acquired() {
		t.Errorf("resource still acquired")
	}
}

func TestSingleResourceReleaseWithoutAcquire(t *testing.T) {
	s := NewSingleResource()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Release without Acquire did not panic")
		}
	}()
	s.Release()
}

func TestSingleResourceStopWaiting(t *testing.T) {
	s := NewSingleResource()
	recvSrErr(t, s.Acquire("a"))

	chB := s.Acquire("b")
	chC := s.Acquire("c")

	want := errors.New("gave up")
	go s.StopWaiting("b", want)
	if err := recvSrErr(t, chB); err != want {
		t.Fatalf("b got %v, want %v", err, want)
	}

	// Unknown token: no-op.
	s.StopWaiting("zzz", want)

	// c is next in line.
	rel := goBool(s.Release)
	if err := recvSrErr(t, chC); err != nil {
		t.Fatalf("c got %v", err)
	}
	recvBool(t, rel)

	if s.Release() {
		t.Errorf("stopped waiter was handed the resource")
	}
}

func TestSingleResourceAbort(t *testing.T) {
	s := NewSingleResource()
	recvSrErr(t, s.Acquire("a"))

	want := errors.New("abort")
	chs := []<-chan error{s.Acquire("b"), s.Acquire("c")}

	// Abort blocks until every waiter has been notified.
	results := make(chan error, len(chs))
	for _, ch := range chs {
		ch := ch
		go func() {
			select {
			case err := <-ch:
				results <- err
			case <-time.After(syncTestTimeout):
				results <- errors.New("timeout")
			}
		}()
	}

	s.Abort(want)
	for range chs {
		if err := <-results; err != want {
			t.Errorf("waiter got %v, want %v", err, want)
		}
	}

	// The owner still holds the resource; nobody is waiting.
	if !s.Acquired() {
		t.Errorf("Abort released the resource")
	}
	if s.Release() {
		t.Errorf("Release handed off to an aborted waiter")
	}
}

func recvBcast(t *testing.T, ch chan interface{}) (interface{}, bool) {
	t.Helper()

	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(syncTestTimeout):
		t.Fatalf("timeout waiting for broadcast")
	}
	return nil, false
}

func TestBcasterSend(t *testing.T) {
	var b Bcaster

	ch1 := b.Listen(1)
	ch2 := b.Listen(1)

	b.Send("x")
	for i, ch := range []chan interface{}{ch1, ch2} {
		if v, ok := recvBcast(t, ch); !ok || v != "x" {
			t.Errorf("listener %d got %v, %v", i, v, ok)
		}
	}

	b.StopListening(ch1)
	if _, ok := recvBcast(t, ch1); ok {
		t.Errorf("StopListening did not close the channel")
	}

	// Stopped listeners no longer receive.
	b.Send("y")
	if v, ok := recvBcast(t, ch2); !ok || v != "y" {
		t.Errorf("listener 2 got %v, %v", v, ok)
	}

	// Unknown channel: no-op.
	b.StopListening(make(chan interface{}))

	b.Clear()
	if _, ok := recvBcast(t, ch2); ok {
		t.Errorf("Clear did not close the channel")
	}

	// No listeners: Send does nothing.
	b.Send("z")
}

func TestBcasterSendAndClear(t *testing.T) {
	var b Bcaster

	chs := []chan interface{}{b.Listen(1), b.Listen(1), b.Listen(1)}
	b.SendAndClear(42)

	for i, ch := range chs {
		if v, ok := recvBcast(t, ch); !ok || v != 42 {
			t.Errorf("listener %d got %v, %v", i, v, ok)
		}
		if _, ok := recvBcast(t, ch); ok {
			t.Errorf("listener %d not closed", i)
		}
	}

	// Listening works again after a clear.
	ch := b.Listen(1)
	b.Send(1)
	if v, ok := recvBcast(t, ch); !ok || v != 1 {
		t.Errorf("got %v, %v", v, ok)
	}
}

type waitResult struct {
	val interface{}
	err error
}

func goWait(b *Blocker, tmo time.Duration,
	stop <-chan struct{}) <-chan waitResult {

	c := make(chan waitResult, 1)
	go func() {
		v, err := b.Wait(tmo, stop)
		c <- waitResult{v, err}
	}()
	return c
}

func recvWait(t *testing.T, c <-chan waitResult) waitResult {
	t.Helper()

	select {
	case r := <-c:
		return r
	case <-time.After(syncTestTimeout):
		t.Fatalf("timeout waiting for Blocker.Wait")
	}
	return waitResult{}
}

func TestBlockerNotStarted(t *testing.T) {
	var b Blocker

	if b.Started() {
		t.Fatalf("new blocker is started")
	}
	v, err := b.Wait(time.Hour, nil)
	if v != nil || err != nil {
		t.Errorf("Wait on unstarted blocker = %v, %v", v, err)
	}
}

func TestBlockerUnblock(t *testing.T) {
	var b Blocker
	b.Start()
	b.Start() // Idempotent.

	if !b.Started() {
		t.Fatalf("blocker not started")
	}

	w1 := goWait(&b, syncTestTimeout*2, nil)
	w2 := goWait(&b, syncTestTimeout*2, nil)

	b.Unblock("done")
	for _, w := range []<-chan waitResult{w1, w2} {
		r := recvWait(t, w)
		if r.err != nil || r.val != "done" {
			t.Errorf("Wait = %v, %v", r.val, r.err)
		}
	}

	if b.Started() {
		t.Errorf("blocker still started after Unblock")
	}

	// Subsequent waiters return immediately with the last value.
	if r := recvWait(t, goWait(&b, time.Hour, nil)); r.val != "done" {
		t.Errorf("Wait after Unblock = %v, %v", r.val, r.err)
	}

	// Unblock when not started is a no-op.
	b.Unblock("other")
}

func TestBlockerUnblockAndRestart(t *testing.T) {
	var b Blocker
	b.Start()
	b.UnblockAndRestart(1)

	if !b.Started() {
		t.Fatalf("blocker not restarted")
	}

	// A new waiter blocks again until the next Unblock.
	r := recvWait(t, goWait(&b, time.Millisecond, nil))
	if r.err == nil {
		t.Errorf("Wait after restart returned %v without blocking", r.val)
	}

	b.Unblock(2)
	if r := recvWait(t, goWait(&b, time.Hour, nil)); r.err != nil ||
		r.val != 2 {

		t.Errorf("Wait = %v, %v", r.val, r.err)
	}
}

func TestBlockerTimeout(t *testing.T) {
	var b Blocker
	b.Start()

	r := recvWait(t, goWait(&b, time.Millisecond, nil))
	if r.err == nil || !strings.Contains(r.err.Error(), "timeout") {
		t.Errorf("got %v, %v; want timeout error", r.val, r.err)
	}
}

func TestBlockerStopChan(t *testing.T) {
	var b Blocker
	b.Start()

	stop := make(chan struct{})
	w := goWait(&b, time.Hour, stop)
	close(stop)

	r := recvWait(t, w)
	if r.err == nil || !strings.Contains(r.err.Error(), "aborted") {
		t.Errorf("got %v, %v; want aborted error", r.val, r.err)
	}
}

type sevError int

func (e sevError) Error() string { return "severity" }

func sevLess(a error, b error) bool {
	return a.(sevError) < b.(sevError)
}

func recvFunnel(t *testing.T, c chan error) error {
	t.Helper()

	select {
	case err := <-c:
		return err
	case <-time.After(syncTestTimeout):
		t.Fatalf("timeout waiting for ErrFunnel")
	}
	return nil
}

func TestErrFunnelMostSevere(t *testing.T) {
	f := &ErrFunnel{
		LessCb:     sevLess,
		AccumDelay: 100 * time.Millisecond,
	}

	w1 := f.Wait()
	w2 := f.Wait()

	f.Insert(sevError(1))
	f.Insert(sevError(3))
	f.Insert(sevError(2))

	for i, w := range []chan error{w1, w2} {
		if err := recvFunnel(t, w); err != sevError(3) {
			t.Errorf("waiter %d got %v, want severity 3", i, err)
		}
		if _, ok := <-w; ok {
			t.Errorf("waiter %d channel not closed", i)
		}
	}

	// The funnel is reusable.
	w := f.Wait()
	f.Insert(sevError(5))
	if err := recvFunnel(t, w); err != sevError(5) {
		t.Errorf("got %v, want severity 5", err)
	}
}

func TestErrFunnelNilInsert(t *testing.T) {
	f := &ErrFunnel{LessCb: sevLess, AccumDelay: time.Millisecond}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("nil Insert did not panic")
		}
	}()
	f.Insert(nil)
}
