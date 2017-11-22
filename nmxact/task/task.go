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
	"fmt"
	"sync"
)

// A single action that runs in the main loop.
type action struct {
	fn func() error
	ch chan error
}

// A queue for running jobs serially.
type TaskQueue struct {
	actCh  chan action
	stopCh chan struct{}
	active bool
	name   string
	mtx    sync.Mutex
	wg     sync.WaitGroup
}

func NewTaskQueue(name string) TaskQueue {
	return TaskQueue{
		name: name,
	}
}

var InactiveError = fmt.Errorf("inactive task queue")

// Pushes the specified function onto the task queue.  When the job completes,
// the result is sent over the returned channel
func (q *TaskQueue) Enqueue(fn func() error) chan error {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	act := action{
		fn: fn,
		ch: make(chan error, 1),
	}

	if !q.active {
		act.ch <- InactiveError
	} else {
		q.actCh <- act
	}

	return act.ch
}

// Enqueues the specified function and waits for it to complete.
func (q *TaskQueue) Run(fn func() error) error {
	return <-q.Enqueue(fn)
}

// Starts the task queue.  A task queue must be started before jobs can be
// enqueued to it.
func (q *TaskQueue) Start(depth int) error {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	if q.active {
		return fmt.Errorf("Task queue started twice \"%s\"", q.name)
	}
	q.active = true

	actCh := make(chan action, depth)
	q.actCh = actCh

	stopCh := make(chan struct{})
	q.stopCh = stopCh

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()

		for {
			select {
			case act, ok := <-actCh:
				if ok {
					err := act.fn()
					act.ch <- err
					close(act.ch)
				}

			case <-stopCh:
				return
			}
		}
	}()

	return nil
}

// Stops the task queue.  If there are any queued jobs, this causes them to
// fail with the specified error.  The task queue must be started again before
// it can be reused.  This function blocks until the task loop returns, so
// alling this from within a job results in deadlock.  If a job needs to stop
// the task queue, it should use StopNoWait instead.
func (q *TaskQueue) Stop(cause error) error {
	if err := q.StopNoWait(cause); err != nil {
		return err
	}

	// Wait for task loop to terminate.
	q.wg.Wait()
	return nil
}

// Stops the task queue.  If there are any queued jobs, this causes them to
// fail with the specified error.  The task queue must be started again before
// it can be reused.  If this function returns success, the stop procedure has
// successfully initiated, but not necessarily completed.
func (q *TaskQueue) StopNoWait(cause error) error {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	if !q.active {
		return fmt.Errorf("Task queue stopped twice \"%s\"", q.name)
	}

	// Stop the task loop.
	close(q.stopCh)

	// Drain unprocessed actions from the action channel.
	close(q.actCh)
	for {
		next, ok := <-q.actCh
		if !ok {
			break
		}

		next.ch <- cause
		close(next.ch)
	}

	q.active = false

	return nil
}

func (q *TaskQueue) Active() bool {
	q.mtx.Lock()
	defer q.mtx.Unlock()

	return q.active
}
