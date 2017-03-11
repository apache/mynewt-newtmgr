package nmble

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/xport"
	"mynewt.apache.org/newt/util/unixchild"
)

type XportCfg struct {
	// Path of Unix domain socket to create and listen on.
	SockPath string

	// Path of the blehostd executable.
	BlehostdPath string

	// Path of the BLE controller device (e.g., /dev/ttyUSB0).
	DevPath string
}

// Implements xport.Xport.
type BleXport struct {
	Bd        *BleDispatcher
	client    *unixchild.Client
	synced    bool
	syncMutex sync.Mutex

	syncListeners [](chan error)

	syncTimeoutMs time.Duration
}

type BleHostError struct {
	Text   string
	Status int
}

func NewBleHostError(status int, text string) *BleHostError {
	return &BleHostError{
		Status: status,
		Text:   text,
	}
}

func FmtBleHostError(status int, format string,
	args ...interface{}) *BleHostError {

	return NewBleHostError(status, fmt.Sprintf(format, args...))
}

func (e *BleHostError) Error() string {
	return e.Text
}

func IsBleHost(err error) bool {
	_, ok := err.(*BleHostError)
	return ok
}

func ToBleHost(err error) *BleHostError {
	if berr, ok := err.(*BleHostError); ok {
		return berr
	} else {
		return nil
	}
}

func NewBleXport(cfg XportCfg) (*BleXport, error) {
	config := unixchild.Config{
		SockPath:  cfg.SockPath,
		ChildPath: cfg.BlehostdPath,
		ChildArgs: []string{cfg.DevPath, cfg.SockPath},
		Depth:     10,
		MaxMsgSz:  10240,
	}

	c := unixchild.New(config)

	bx := &BleXport{
		client:        c,
		Bd:            NewBleDispatcher(),
		syncTimeoutMs: 10000,
	}

	return bx, nil
}

func (bx *BleXport) notifySyncListeners(err error) {
	bx.syncMutex.Lock()
	bx.synced = err == nil
	for _, ch := range bx.syncListeners {
		ch <- err
	}
	bx.syncMutex.Unlock()
}

func (bx *BleXport) Start() error {
	if err := bx.client.Start(); err != nil {
		return xport.NewXportError(
			"Failed to start child child process: " + err.Error())
	}

	go func() {
		err := <-bx.client.ErrChild
		bx.Bd.ErrorAll(err)
	}()

	go func() {
		for {
			if _, err := bx.rx(); err != nil {
				// The error should have been reported to everyone interested.
				break
			}
		}
	}()

	bl := NewBleListener()
	base := BleMsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_SYNC_EVT,
		Seq:        -1,
		ConnHandle: -1,
	}
	if err := bx.Bd.AddListener(base, bl); err != nil {
		return err
	}

	go func() {
		defer bx.Bd.RemoveListener(base)

		select {
		case err := <-bl.ErrChan:
			bx.notifySyncListeners(err)
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleSyncEvt:
				if msg.Synced {
					bx.notifySyncListeners(nil)
				}
			}
		}
	}()

	return nil
}

func (bx *BleXport) querySyncStatus() (bool, error) {
	bx.syncMutex.Lock()
	synced := bx.synced
	bx.syncMutex.Unlock()

	if synced {
		return true, nil
	}

	req := &BleSyncReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SYNC,
		Seq:  NextSeq(),
	}

	j, err := json.Marshal(req)
	if err != nil {
		return false, err
	}

	bl := NewBleListener()
	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        req.Seq,
		ConnHandle: -1,
	}
	if err := bx.Bd.AddListener(base, bl); err != nil {
		return false, err
	}
	defer bx.Bd.RemoveListener(base)

	bx.txNoSync(j)
	for {
		select {
		case err := <-bl.ErrChan:
			return false, err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleSyncRsp:
				if msg.Synced {
					// Remember that the host is synced.  This is done so that
					// we don't query the daemon for its sync status on every
					// subsequent command.
					bx.syncMutex.Lock()
					bx.synced = true
					bx.syncMutex.Unlock()
				}
				return msg.Synced, nil
			}
		}
	}
}

func (bx *BleXport) waitUntilSync() error {
	synced, err := bx.querySyncStatus()
	if err != nil {
		return err
	}
	if synced {
		return nil
	}

	ch := make(chan error, 1)
	bx.syncMutex.Lock()
	bx.syncListeners = append(bx.syncListeners, ch)
	bx.syncMutex.Unlock()

	go func() {
		time.Sleep(bx.syncTimeoutMs * time.Millisecond)
		ch <- xport.NewXportError(
			"Timeout waiting for host <-> controller sync")
	}()

	err = <-ch
	return err
}

func (bx *BleXport) txNoSync(data []byte) {
	log.Debugf("Tx to blehostd:\n%s", hex.Dump(data))
	bx.client.ToChild <- data
}

func (bx *BleXport) Tx(data []byte) error {
	if err := bx.waitUntilSync(); err != nil {
		return err
	}

	bx.txNoSync(data)
	return nil
}

func (bx *BleXport) rx() ([]byte, error) {
	select {
	case err := <-bx.client.ErrChild:
		return nil, err
	case buf := <-bx.client.FromChild:
		if len(buf) != 0 {
			log.Debugf("Receive from blehostd:\n%s", hex.Dump(buf))
			bx.Bd.Dispatch(buf)
		}
		return buf, nil
	}
}

func (bx *BleXport) Stop() error {
	if bx.client != nil {
		bx.client.FromChild <- nil
		bx.client.Stop()
	}

	return nil
}
