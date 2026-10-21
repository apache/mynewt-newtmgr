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

package task

import (
	"errors"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func recvResult(t *testing.T, ch chan error) error {
	t.Helper()

	select {
	case err := <-ch:
		return err
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for job result")
	}
	return nil
}

func stopWithTimeout(t *testing.T, q *TaskQueue, cause error) error {
	t.Helper()

	res := make(chan error, 1)
	go func() { res <- q.Stop(cause) }()
	return recvResult(t, res)
}

func TestEnqueueInactive(t *testing.T) {
	q := NewTaskQueue("test")

	ran := false
	ch := q.Enqueue(func() error {
		ran = true
		return nil
	})
	if err := recvResult(t, ch); err != InactiveError {
		t.Fatalf("got %v, want InactiveError", err)
	}
	if err := q.Run(func() error { return nil }); err != InactiveError {
		t.Fatalf("Run got %v, want InactiveError", err)
	}
	if ran {
		t.Errorf("job ran on inactive queue")
	}
	if q.Active() {
		t.Errorf("new queue is active")
	}
}

func TestRunOrderAndResults(t *testing.T) {
	q := NewTaskQueue("test")
	if err := q.Start(8); err != nil {
		t.Fatal(err)
	}
	defer stopWithTimeout(t, &q, errors.New("done"))

	if !q.Active() {
		t.Fatalf("started queue not active")
	}

	// Only touched by the task goroutine; read after all jobs completed.
	var order []int
	var chs []chan error
	jobErr := errors.New("job 3 failed")
	for i := 0; i < 5; i++ {
		i := i
		chs = append(chs, q.Enqueue(func() error {
			order = append(order, i)
			if i == 3 {
				return jobErr
			}
			return nil
		}))
	}

	for i, ch := range chs {
		want := error(nil)
		if i == 3 {
			want = jobErr
		}
		if err := recvResult(t, ch); err != want {
			t.Errorf("job %d returned %v, want %v", i, err, want)
		}
		if _, ok := <-ch; ok {
			t.Errorf("job %d result channel not closed", i)
		}
	}

	for i, v := range order {
		if v != i {
			t.Fatalf("jobs ran in order %v", order)
		}
	}
	if len(order) != 5 {
		t.Fatalf("%d jobs ran, want 5", len(order))
	}
}

func TestStartStopTwice(t *testing.T) {
	q := NewTaskQueue("test")

	if err := q.Stop(errors.New("x")); err == nil {
		t.Errorf("Stop of unstarted queue succeeded")
	}
	if err := q.Start(1); err != nil {
		t.Fatal(err)
	}
	if err := q.Start(1); err == nil {
		t.Errorf("second Start succeeded")
	}
	if err := stopWithTimeout(t, &q, errors.New("x")); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := q.Stop(errors.New("x")); err == nil {
		t.Errorf("second Stop succeeded")
	}
	if err := q.StopNoWait(errors.New("x")); err == nil {
		t.Errorf("StopNoWait of stopped queue succeeded")
	}
}

func TestStopFailsPendingJobs(t *testing.T) {
	q := NewTaskQueue("test")
	if err := q.Start(4); err != nil {
		t.Fatal(err)
	}

	running := make(chan struct{})
	release := make(chan struct{})
	ch1 := q.Enqueue(func() error {
		close(running)
		<-release
		return nil
	})

	select {
	case <-running:
	case <-time.After(testTimeout):
		t.Fatalf("first job did not start")
	}

	ran := make(chan int, 2)
	ch2 := q.Enqueue(func() error { ran <- 2; return nil })
	ch3 := q.Enqueue(func() error { ran <- 3; return nil })

	cause := errors.New("stopping")
	stopped := make(chan error, 1)
	go func() { stopped <- q.Stop(cause) }()

	// Pending jobs fail with the cause without running.
	for i, ch := range []chan error{ch2, ch3} {
		if err := recvResult(t, ch); err != cause {
			t.Errorf("pending job %d got %v, want %v", i+2, err, cause)
		}
	}

	// Stop waits for the running job.
	select {
	case err := <-stopped:
		t.Fatalf("Stop returned (%v) while a job was running", err)
	default:
	}

	close(release)
	if err := recvResult(t, ch1); err != nil {
		t.Errorf("running job got %v", err)
	}
	if err := recvResult(t, stopped); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if len(ran) != 0 {
		t.Errorf("pending job ran after Stop")
	}
	if q.Active() {
		t.Errorf("queue still active after Stop")
	}
	ch := q.Enqueue(func() error { return nil })
	if err := recvResult(t, ch); err != InactiveError {
		t.Errorf("Enqueue after Stop got %v, want InactiveError", err)
	}
}

func TestStopNoWaitFromJob(t *testing.T) {
	q := NewTaskQueue("test")
	if err := q.Start(1); err != nil {
		t.Fatal(err)
	}

	cause := errors.New("job stopped queue")
	err := recvResult(t, q.Enqueue(func() error {
		return q.StopNoWait(cause)
	}))
	if err != nil {
		t.Fatalf("StopNoWait from job failed: %v", err)
	}

	if q.Active() {
		t.Errorf("queue still active")
	}
}

func TestRestartAfterStop(t *testing.T) {
	q := NewTaskQueue("test")

	for i := 0; i < 3; i++ {
		if err := q.Start(1); err != nil {
			t.Fatalf("Start %d failed: %v", i, err)
		}
		want := errors.New("result")
		if err := q.Run(func() error { return want }); err != want {
			t.Fatalf("Run %d got %v, want %v", i, err, want)
		}
		if err := stopWithTimeout(t, &q, errors.New("x")); err != nil {
			t.Fatalf("Stop %d failed: %v", i, err)
		}
	}
}
