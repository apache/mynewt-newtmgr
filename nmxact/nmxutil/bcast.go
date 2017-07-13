package nmxutil

import (
	"sync"
)

type Bcaster struct {
	chs [](chan interface{})
	mtx sync.Mutex
}

func (b *Bcaster) Listen() chan interface{} {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	ch := make(chan interface{})
	b.chs = append(b.chs, ch)

	return ch
}

func (b *Bcaster) Send(val interface{}) {
	b.mtx.Lock()
	chs := b.chs
	b.mtx.Unlock()

	for _, ch := range chs {
		ch <- val
		close(ch)
	}
}

func (b *Bcaster) Clear() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.chs = nil
}

func (b *Bcaster) SendAndClear(val interface{}) {
	b.Send(val)
	b.Clear()
}
