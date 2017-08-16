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
	"sync"
)

type SRWaiter struct {
	c     chan error
	token interface{}
}

type SingleResource struct {
	acquired  bool
	waitQueue []SRWaiter
	mtx       sync.Mutex
}

func NewSingleResource() SingleResource {
	return SingleResource{}
}

func (s *SingleResource) Acquire(token interface{}) error {
	s.mtx.Lock()

	if !s.acquired {
		s.acquired = true
		s.mtx.Unlock()
		return nil
	}

	// XXX: Verify no duplicates.

	w := SRWaiter{
		c:     make(chan error),
		token: token,
	}
	s.waitQueue = append(s.waitQueue, w)

	s.mtx.Unlock()

	err := <-w.c
	if err != nil {
		return err
	}

	return nil
}

// @return                      true if a pending waiter acquired the resource;
//                              false if the resource is now free.
func (s *SingleResource) Release() bool {
	s.mtx.Lock()

	if !s.acquired {
		panic("SingleResource release without acquire")
		s.mtx.Unlock()
		return false
	}

	if len(s.waitQueue) == 0 {
		s.acquired = false
		s.mtx.Unlock()
		return false
	}

	w := s.waitQueue[0]
	s.waitQueue = s.waitQueue[1:]

	s.mtx.Unlock()

	w.c <- nil

	return true
}

func (s *SingleResource) StopWaiting(token interface{}, err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, w := range s.waitQueue {
		if w.token == token {
			w.c <- err
			return
		}
	}
}

func (s *SingleResource) Abort(err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, w := range s.waitQueue {
		w.c <- err
	}
	s.waitQueue = nil
}

func (s *SingleResource) Acquired() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.acquired
}
