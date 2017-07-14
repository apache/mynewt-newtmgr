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
	logDepth int
	bls      map[*Listener]MsgBase
	mtx      sync.Mutex
	wg       sync.WaitGroup
}

func NewReceiver(id uint32, bx *BleXport, logDepth int) *Receiver {
	return &Receiver{
		id:       id,
		bx:       bx,
		logDepth: logDepth + 3,
		bls:      map[*Listener]MsgBase{},
	}
}

func (r *Receiver) addListener(name string, base MsgBase) (
	*Listener, error) {

	nmxutil.LogAddListener(r.logDepth, base, r.id, name)

	bl := NewListener()

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if err := r.bx.AddListener(base, bl); err != nil {
		return nil, err
	}

	r.bls[bl] = base
	r.wg.Add(1)

	return bl, nil
}

func (r *Receiver) AddBaseListener(name string, base MsgBase) (
	*Listener, error) {

	return r.addListener(name, base)
}

func (r *Receiver) AddSeqListener(name string, seq BleSeq) (
	*Listener, error) {

	base := MsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}
	return r.addListener(name, base)
}

func (r *Receiver) removeListener(name string, base MsgBase) *Listener {
	nmxutil.LogRemoveListener(r.logDepth, base, r.id, name)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	bl := r.bx.RemoveListener(base)
	delete(r.bls, bl)

	if bl != nil {
		r.wg.Done()
	}

	return bl
}

func (r *Receiver) RemoveBaseListener(name string, base MsgBase) {
	r.removeListener(name, base)
}

func (r *Receiver) RemoveSeqListener(name string, seq BleSeq) {
	base := MsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}

	r.removeListener(name, base)
}

func (r *Receiver) RemoveAll(name string) {
	r.mtx.Lock()
	bls := r.bls
	r.bls = map[*Listener]MsgBase{}
	r.mtx.Unlock()

	for _, base := range bls {
		r.removeListener(name, base)
	}
}

func (r *Receiver) WaitUntilNoListeners() {
	r.wg.Wait()
}
