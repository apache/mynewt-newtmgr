package nmxutil

import (
	"math/rand"
	"sync"
)

var nextNmpSeq uint8
var beenRead bool
var seqMutex sync.Mutex

func NextNmpSeq() uint8 {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !beenRead {
		nextNmpSeq = uint8(rand.Uint32())
		beenRead = true
	}

	val := nextNmpSeq
	nextNmpSeq++

	return val
}

type SingleResource struct {
	acquired  bool
	waitQueue [](chan error)
	mtx       sync.Mutex
}

func NewSingleResource() SingleResource {
	return SingleResource{
		waitQueue: [](chan error){},
	}
}

func (s *SingleResource) removeWaiter(waiter chan error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for i, w := range s.waitQueue {
		if w == waiter {
			s.waitQueue = append(s.waitQueue[:i], s.waitQueue[i+1:]...)
		}
	}
}

func (s *SingleResource) Acquire() error {
	s.mtx.Lock()

	if !s.acquired {
		s.acquired = true
		s.mtx.Unlock()
		return nil
	}

	w := make(chan error)
	s.waitQueue = append(s.waitQueue, w)

	s.mtx.Unlock()

	err := <-w
	if err != nil {
		s.removeWaiter(w)
		return err
	}

	return nil
}

func (s *SingleResource) Release() {
	s.mtx.Lock()

	if !s.acquired {
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

	w <- nil
}

func (s *SingleResource) Abort(err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, w := range s.waitQueue {
		w <- err
	}
}
