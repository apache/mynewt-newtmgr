package nmxutil

import (
	"fmt"
	"sync"
	"time"
)

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
	started  bool
	waiters  [](chan error)
}

func (f *ErrFunnel) Start() {
	f.resetMtx.Lock()

	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.started = true
}

func (f *ErrFunnel) Insert(err error) {
	if err == nil {
		panic("ErrFunnel nil insert")
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()

	if !f.started {
		panic("ErrFunnel insert without start")
	}

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

func (f *ErrFunnel) Reset() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if f.started {
		f.started = false
		f.curErr = nil
		f.errTimer.Stop()
		f.resetMtx.Unlock()
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

	f.ProcCb(err)

	for _, w := range waiters {
		w <- err
	}
}

func (f *ErrFunnel) Wait() error {
	var err error
	var c chan error

	f.mtx.Lock()

	if !f.started {
		if f.curErr == nil {
			err = fmt.Errorf("Wait on unstarted ErrFunnel")
		} else {
			err = f.curErr
		}
	} else {
		c = make(chan error)
		f.waiters = append(f.waiters, c)
	}

	f.mtx.Unlock()

	if err != nil {
		return err
	} else {
		return <-c
	}
}
