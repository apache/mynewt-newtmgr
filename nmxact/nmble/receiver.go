package nmble

import (
	"sync"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// The receiver never writes to any of its listeners.  It only maintains a set
// of listeners so that their lifetimes can be tracked and to facilitate their
// removal from the BLE transport.

type Receiver struct {
	id       uint32
	bx       *BleXport
	lm       *ListenerMap
	logDepth int
	mtx      sync.Mutex
	wg       sync.WaitGroup
}

func NewReceiver(id uint32, bx *BleXport, logDepth int) *Receiver {
	return &Receiver{
		id:       id,
		bx:       bx,
		logDepth: logDepth + 3,
		lm:       NewListenerMap(),
	}
}

func (r *Receiver) AddListener(name string, key ListenerKey) (
	*Listener, error) {

	nmxutil.LogAddListener(r.logDepth, key, r.id, name)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	bl, err := r.bx.AddListener(key)
	if err != nil {
		return nil, err
	}

	r.lm.AddListener(key, bl)
	r.wg.Add(1)

	return bl, nil
}

func (r *Receiver) RemoveKey(name string, key ListenerKey) *Listener {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.bx.RemoveKey(key)
	bl := r.lm.RemoveKey(key)
	if bl == nil {
		return nil
	}

	nmxutil.LogRemoveListener(r.logDepth, key, r.id, name)
	r.wg.Done()
	return bl
}

func (r *Receiver) RemoveListener(name string, listener *Listener) *ListenerKey {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.bx.RemoveListener(listener)
	key := r.lm.RemoveListener(listener)
	if key == nil {
		return nil
	}

	nmxutil.LogRemoveListener(r.logDepth, key, r.id, name)
	r.wg.Done()
	return key
}

func (r *Receiver) RemoveAll(name string) {
	r.mtx.Lock()
	bls := r.lm.ExtractAll()
	r.mtx.Unlock()

	for _, bl := range bls {
		if key := r.bx.RemoveListener(bl); key != nil {
			nmxutil.LogRemoveListener(r.logDepth, key, r.id, name)
		}
		r.wg.Done()
	}
}

func (r *Receiver) WaitUntilNoListeners() {
	r.wg.Wait()
}
