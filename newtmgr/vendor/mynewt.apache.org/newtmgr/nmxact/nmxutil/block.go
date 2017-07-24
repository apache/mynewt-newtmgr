package nmxutil

import (
	"sync"
)

// Blocks a variable number of waiters until Unblock() is called.  Subsequent
// waiters are unblocked until the next call to Block().
type Blocker struct {
	ch  chan struct{}
	mtx sync.Mutex
}

func (b *Blocker) Wait() {
	b.mtx.Lock()
	ch := b.ch
	b.mtx.Unlock()

	if ch != nil {
		<-ch
	}
}

func (b *Blocker) Block() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.ch == nil {
		b.ch = make(chan struct{})
	}
}

func (b *Blocker) Unblock() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.ch != nil {
		close(b.ch)
		b.ch = nil
	}
}
