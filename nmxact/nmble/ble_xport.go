package nmble

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/nmxutil"
	"mynewt.apache.org/newt/nmxact/sesn"
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

type BleXportState uint32

const (
	BLE_XPORT_STATE_STOPPED BleXportState = iota
	BLE_XPORT_STATE_STARTING
	BLE_XPORT_STATE_STARTED
)

// Implements xport.Xport.
type BleXport struct {
	Bd     *BleDispatcher
	client *unixchild.Client
	state  BleXportState

	syncTimeoutMs time.Duration
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

func (bx *BleXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	switch cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return NewBlePlainSesn(bx, cfg.Ble.OwnAddrType, cfg.Ble.Peer), nil
	case sesn.MGMT_PROTO_OMP:
		return NewBleOicSesn(bx, cfg.Ble.OwnAddrType, cfg.Ble.Peer), nil
	default:
		return nil, fmt.Errorf(
			"Invalid management protocol: %d; expected NMP or OMP",
			cfg.MgmtProto)
	}
}

func (bx *BleXport) addSyncListener() (*BleListener, error) {
	bl := NewBleListener()
	base := BleMsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_SYNC_EVT,
		Seq:        -1,
		ConnHandle: -1,
	}
	if err := bx.Bd.AddListener(base, bl); err != nil {
		return nil, err
	}

	return bl, nil
}

func (bx *BleXport) querySyncStatus() (bool, error) {
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
				return msg.Synced, nil
			}
		}
	}
}

func (bx *BleXport) onError(err error) {
	bx.Bd.ErrorAll(err)
	if bx.client != nil {
		bx.client.Stop()
		bx.client.FromChild <- nil
	}
}

func (bx *BleXport) setStateFrom(from BleXportState, to BleXportState) bool {
	return atomic.CompareAndSwapUint32(
		(*uint32)(&bx.state), uint32(from), uint32(to))
}

func (bx *BleXport) getState() BleXportState {
	u32 := atomic.LoadUint32((*uint32)(&bx.state))
	return BleXportState(u32)
}

func (bx *BleXport) Stop() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_STARTED, BLE_XPORT_STATE_STOPPED) &&
		!bx.setStateFrom(BLE_XPORT_STATE_STARTING, BLE_XPORT_STATE_STOPPED) {

		// Stop already in progress.
		return nil
	}

	bx.onError(nil)
	return nil
}

func (bx *BleXport) Start() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_STARTING) {
		return nmxutil.NewXportError("BLE xport started twice")
	}

	if err := bx.client.Start(); err != nil {
		return nmxutil.NewXportError(
			"Failed to start child child process: " + err.Error())
	}

	go func() {
		err := <-bx.client.ErrChild
		bx.onError(err)
		return
	}()

	go func() {
		for {
			if _, err := bx.rx(); err != nil {
				// The error should have been reported to everyone interested.
				break
			}
		}
	}()

	bl, err := bx.addSyncListener()
	if err != nil {
		bx.Stop()
		return err
	}

	synced, err := bx.querySyncStatus()
	if err != nil {
		bx.Stop()
		return err
	}

	if !synced {
		// Not synced yet.  Wait for sync event.

	SyncLoop:
		for {
			select {
			case err := <-bl.ErrChan:
				return err
			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					if msg.Synced {
						break SyncLoop
					}
				}
			case <-time.After(bx.syncTimeoutMs * time.Millisecond):
				bx.Stop()
				return nmxutil.NewXportError(
					"Timeout waiting for host <-> controller sync")
			}
		}
	}

	// Host and controller are synced.  Listen for sync loss in the background.
	go func() {
		for {
			select {
			case err := <-bl.ErrChan:
				bx.onError(err)
				return
			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					if !msg.Synced {
						bx.onError(nmxutil.NewXportError(
							"BLE host <-> controller sync lost"))
						return
					}
				}
			}
		}
	}()

	if !bx.setStateFrom(BLE_XPORT_STATE_STARTING, BLE_XPORT_STATE_STARTED) {
		return nmxutil.NewXportError(
			"Internal error; BLE transport in unexpected state")
	}

	return nil
}

func (bx *BleXport) txNoSync(data []byte) {
	log.Debugf("Tx to blehostd:\n%s", hex.Dump(data))
	bx.client.ToChild <- data
}

func (bx *BleXport) Tx(data []byte) error {
	if bx.getState() != BLE_XPORT_STATE_STARTED {
		return nmxutil.NewXportError("Attempt to transmit before BLE xport " +
			"fully started")
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
