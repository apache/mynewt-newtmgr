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

func (s *SingleResource) Release() {
	s.mtx.Lock()

	if !s.acquired {
		panic("SingleResource release without acquire")
		s.mtx.Unlock()
		return
	}

	if len(s.waitQueue) == 0 {
		s.acquired = false
		s.mtx.Unlock()
		return
	}

	w := s.waitQueue[0]
	s.waitQueue = s.waitQueue[1:]

	s.mtx.Unlock()

	w.c <- nil
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
