package nmxutil

import (
	"sync"
	"time"
)

type ErrLessFn func(a error, b error) bool
type ErrProcFn func(err error)

// Aggregates errors that occur close in time.  The most severe error gets
// reported.
type ErrFunnel struct {
	LessCb     ErrLessFn
	AccumDelay time.Duration

	mtx      sync.Mutex
	resetMtx sync.Mutex
	curErr   error
	errTimer *time.Timer
	waiters  [](chan error)
}

func (f *ErrFunnel) Insert(err error) {
	if err == nil {
		panic("ErrFunnel nil insert")
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.curErr == nil {
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

func (f *ErrFunnel) timerExp() {
	f.mtx.Lock()

	err := f.curErr
	f.curErr = nil

	waiters := f.waiters
	f.waiters = nil

	f.mtx.Unlock()

	if err == nil {
		panic("ErrFunnel timer expired but no error")
	}

	for _, w := range waiters {
		w <- err
		close(w)
	}
}

func (f *ErrFunnel) Wait() chan error {
	c := make(chan error)

	f.mtx.Lock()
	f.waiters = append(f.waiters, c)
	f.mtx.Unlock()

	return c
}
