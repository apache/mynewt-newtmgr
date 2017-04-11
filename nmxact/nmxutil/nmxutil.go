package nmxutil

import (
	"math/rand"
	"sync"
	"time"
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

type ErrLessFn func(a error, b error) bool
type ErrProcFn func(err error)

// Aggregates errors that occur close in time.  The most severe error gets
// reported.
type ErrFunnel struct {
	LessCb     ErrLessFn
	ProcCb     ErrProcFn
	AccumDelay time.Duration

	mtx      sync.Mutex
	resetMtx sync.Mutex
	curErr   error
	errTimer *time.Timer
}

func (f *ErrFunnel) Insert(err error) {
	if err == nil {
		panic("ErrFunnel nil insert")
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.curErr == nil {
		// Subsequent use attempts will block until the funnel is inactive.
		f.resetMtx.Lock()

		f.curErr = err
		f.errTimer = time.AfterFunc(f.AccumDelay, func() {
			f.timerExp()
		})
	} else {
		if f.LessCb(f.curErr, err) {
			if !f.errTimer.Stop() {
				<-f.errTimer.C
			}
			f.curErr = err
			f.errTimer.Reset(f.AccumDelay)
		}
	}
}

func (f *ErrFunnel) resetNoLock() {
	if f.curErr != nil {
		f.curErr = nil
		f.errTimer.Stop()
		f.resetMtx.Unlock()
	}
}

func (f *ErrFunnel) Reset() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.resetNoLock()
}

func (f *ErrFunnel) BlockUntilReset() {
	f.resetMtx.Lock()
	f.resetMtx.Unlock()
}

func (f *ErrFunnel) timerExp() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.curErr == nil {
		panic("ErrFunnel timer expired but no error")
	}

	f.ProcCb(f.curErr)

	f.resetNoLock()
}
