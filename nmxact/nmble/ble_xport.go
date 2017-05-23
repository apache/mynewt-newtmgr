package nmble

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/util/unixchild"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type XportCfg struct {
	// ***********************
	// *** Required fields ***
	// ***********************

	// Path of Unix domain socket to create and listen on.
	SockPath string

	// Path of the blehostd executable.
	BlehostdPath string

	// Path of the BLE controller device (e.g., /dev/ttyUSB0).
	DevPath string

	// ***********************
	// *** Optional fields ***
	// ***********************

	// How long to wait for the blehostd process to connect to the Unix domain
	// socket.
	// Default: 1 second.
	BlehostdAcceptTimeout time.Duration

	// How long to wait for a JSON response from the blehostd process.
	// Default: 10 seconds.
	BlehostdRspTimeout time.Duration

	// Whether to restart the transport if it goes down or fails to start in
	// the first place.
	// Default: true.
	Restart bool

	// How long to allow for the host and controller to sync at startup.
	// Default: 10 seconds.
	SyncTimeout time.Duration

	// The static random address to use.  Set to nil if one should be
	// generated.
	// Default: nil (auto-generate).
	RandAddr *BleAddr

	// The value to specify during ATT MTU exchange.
	// Default: 264.
	PreferredMtu uint16
}

func NewXportCfg() XportCfg {
	return XportCfg{
		BlehostdAcceptTimeout: time.Second,
		BlehostdRspTimeout:    10 * time.Second,
		Restart:               true,
		SyncTimeout:           10 * time.Second,
		PreferredMtu:          264,
	}
}

type BleXportState int

const (
	BLE_XPORT_STATE_DORMANT BleXportState = iota
	BLE_XPORT_STATE_STOPPING
	BLE_XPORT_STATE_STOPPED
	BLE_XPORT_STATE_STARTING
	BLE_XPORT_STATE_STARTED
)

// Implements xport.Xport.
type BleXport struct {
	Bd                *BleDispatcher
	client            *unixchild.Client
	state             BleXportState
	stopChan          chan struct{}
	numStopListeners  int
	shutdownChan      chan bool
	readyChan         chan error
	numReadyListeners int
	master            nmxutil.SingleResource
	randAddr          *BleAddr
	mtx               sync.Mutex
	scanner           *BleScanner

	cfg XportCfg
}

func NewBleXport(cfg XportCfg) (*BleXport, error) {
	bx := &BleXport{
		Bd:           NewBleDispatcher(),
		shutdownChan: make(chan bool),
		readyChan:    make(chan error),
		master:       nmxutil.NewSingleResource(),
		cfg:          cfg,
	}

	return bx, nil
}

func (bx *BleXport) createUnixChild() {
	config := unixchild.Config{
		SockPath:      bx.cfg.SockPath,
		ChildPath:     bx.cfg.BlehostdPath,
		ChildArgs:     []string{bx.cfg.DevPath, bx.cfg.SockPath},
		Depth:         10,
		MaxMsgSz:      10240,
		AcceptTimeout: bx.cfg.BlehostdAcceptTimeout,
	}

	bx.client = unixchild.New(config)
}

func (bx *BleXport) BuildScanner() (scan.Scanner, error) {
	// The transport only allows a single scanner.  This is because the
	// master privileges need to managed among the scanner and the
	// sessions.
	if bx.scanner == nil {
		bx.scanner = NewBleScanner(bx)
	}

	return bx.scanner, nil
}

func (bx *BleXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	switch cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return NewBlePlainSesn(bx, cfg), nil
	case sesn.MGMT_PROTO_OMP:
		return NewBleOicSesn(bx, cfg), nil
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
		Seq:        BLE_SEQ_NONE,
		ConnHandle: -1,
	}
	if err := bx.Bd.AddListener(base, bl); err != nil {
		return nil, err
	}

	return bl, nil
}

func (bx *BleXport) removeSyncListener() {
	base := BleMsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_SYNC_EVT,
		Seq:        BLE_SEQ_NONE,
		ConnHandle: -1,
	}
	bx.Bd.RemoveListener(base)
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

	if err := bx.txNoSync(j); err != nil {
		return false, err
	}
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

func (bx *BleXport) initialSyncCheck() (bool, *BleListener, error) {
	bl, err := bx.addSyncListener()
	if err != nil {
		return false, nil, err
	}

	synced, err := bx.querySyncStatus()
	if err != nil {
		bx.removeSyncListener()
		return false, nil, err
	}

	return synced, bl, nil
}

func (bx *BleXport) shutdown(restart bool, err error) {
	if !nmxutil.IsXport(err) {
		panic(fmt.Sprintf(
			"BleXport.shutdown() received error that isn't an XportError: %+v",
			err))
	}

	bx.mtx.Lock()

	var fullyStarted bool
	var already bool

	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		already = false
		fullyStarted = true
		bx.state = BLE_XPORT_STATE_STOPPING
	case BLE_XPORT_STATE_STARTING:
		already = false
		fullyStarted = false
		bx.state = BLE_XPORT_STATE_STOPPING
	default:
		already = true
	}

	bx.mtx.Unlock()

	if already {
		// Shutdown already in progress.
		return
	}

	if bx.scanner != nil {
		bx.scanner.Stop()
	}

	// Stop the unixchild instance (blehostd + socket).
	if bx.client != nil {
		bx.client.Stop()
	}

	// Indicate error to all clients who are waiting for the master resource.
	bx.master.Abort(err)

	// Indicate an error to all of this transport's listeners.  This prevents
	// them from blocking endlessly while awaiting a BLE message.
	bx.Bd.ErrorAll(err)

	// Stop all of this transport's go routines.
	for i := 0; i < bx.numStopListeners; i++ {
		bx.stopChan <- struct{}{}
	}

	bx.setStateFrom(BLE_XPORT_STATE_STOPPING, BLE_XPORT_STATE_STOPPED)

	// Indicate that the shutdown is complete.  If restarts are enabled on this
	// transport, this signals that the transport should be started again.
	if fullyStarted {
		bx.shutdownChan <- restart
	}
}

func (bx *BleXport) blockUntilReady() error {
	bx.mtx.Lock()
	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		// Already started; don't block.
		bx.mtx.Unlock()
		return nil

	case BLE_XPORT_STATE_DORMANT:
		// Not in the process of starting; the user will be waiting forever.
		bx.mtx.Unlock()
		return fmt.Errorf("Attempt to use BLE transport without starting it")

	default:
	}

	bx.numReadyListeners++
	bx.mtx.Unlock()

	return <-bx.readyChan
}

func (bx *BleXport) notifyReadyListeners(err error) {
	for i := 0; i < bx.numReadyListeners; i++ {
		bx.readyChan <- err
	}
	bx.numReadyListeners = 0
}

func (bx *BleXport) setStateFrom(from BleXportState, to BleXportState) bool {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	if bx.state != from {
		return false
	}

	bx.state = to
	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		bx.notifyReadyListeners(nil)
	case BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_DORMANT:
		bx.notifyReadyListeners(nmxutil.NewXportError("BLE transport stopped"))
	default:
	}

	return true
}

func (bx *BleXport) Stop() error {
	synced, err := bx.querySyncStatus()
	if err == nil && synced {
		// Reset controller so that all outstanding connections terminate.
		ResetXact(bx)
	}

	bx.shutdown(false, nmxutil.NewXportError("xport stopped"))
	return nil
}

func (bx *BleXport) startOnce() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_STARTING) {
		return nmxutil.NewXportError("BLE xport started twice")
	}

	bx.stopChan = make(chan struct{})
	bx.numStopListeners = 0

	bx.createUnixChild()
	if err := bx.client.Start(); err != nil {
		if unixchild.IsUcAcceptError(err) {
			err = nmxutil.NewXportError(
				"blehostd did not connect to socket; " +
					"controller not attached?")
		} else {
			err = nmxutil.NewXportError(
				"Failed to start child process: " + err.Error())
		}
		bx.shutdown(true, err)
		return err
	}

	go func() {
		bx.numStopListeners++
		for {
			select {
			case err := <-bx.client.ErrChild:
				err = nmxutil.NewXportError("BLE transport error: " +
					err.Error())
				go bx.shutdown(true, err)

			case <-bx.stopChan:
				return
			}
		}
	}()

	go func() {
		bx.numStopListeners++
		for {
			select {
			case buf := <-bx.client.FromChild:
				if len(buf) != 0 {
					log.Debugf("Receive from blehostd:\n%s", hex.Dump(buf))
					bx.Bd.Dispatch(buf)
				}

			case <-bx.stopChan:
				return
			}
		}
	}()

	synced, bl, err := bx.initialSyncCheck()
	if err != nil {
		bx.shutdown(true, err)
		return err
	}

	if !synced {
		// Not synced yet.  Wait for sync event.

	SyncLoop:
		for {
			select {
			case err := <-bl.ErrChan:
				bx.shutdown(true, err)
				return err
			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					if msg.Synced {
						break SyncLoop
					}
				}
			case <-time.After(bx.cfg.SyncTimeout):
				err := nmxutil.NewXportError(
					"Timeout waiting for host <-> controller sync")
				bx.shutdown(true, err)
				return err
			}
		}
	}

	// Host and controller are synced.  Listen for sync loss in the background.
	go func() {
		bx.numStopListeners++
		for {
			select {
			case err := <-bl.ErrChan:
				go bx.shutdown(true, err)
			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					if !msg.Synced {
						go bx.shutdown(true, nmxutil.NewXportError(
							"BLE host <-> controller sync lost"))
					}
				}
			case <-bx.stopChan:
				return
			}
		}
	}()

	if bx.randAddr == nil {
		addr, err := GenRandAddrXact(bx)
		if err != nil {
			bx.shutdown(true, err)
			return err
		}

		bx.randAddr = &addr
	}

	if err := SetRandAddrXact(bx, *bx.randAddr); err != nil {
		bx.shutdown(true, err)
		return err
	}

	if err := SetPreferredMtuXact(bx, bx.cfg.PreferredMtu); err != nil {
		bx.shutdown(true, err)
		return err
	}

	if !bx.setStateFrom(BLE_XPORT_STATE_STARTING, BLE_XPORT_STATE_STARTED) {
		bx.shutdown(true, err)
		return nmxutil.NewXportError(
			"Internal error; BLE transport in unexpected state")
	}

	return nil
}

func (bx *BleXport) Start() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_DORMANT, BLE_XPORT_STATE_STOPPED) {
		return nmxutil.NewXportError("BLE xport started twice")
	}

	// Try to start the transport.  If this first attempt fails, report the
	// error and don't retry.
	if err := bx.startOnce(); err != nil {
		bx.setStateFrom(BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_DORMANT)
		log.Debugf("Error starting BLE transport: %s",
			err.Error())
		return err
	}

	// Now that the first start attempt has succeeded, start a restart loop in
	// the background.
	go func() {
		// Block until transport shuts down.
		restart := <-bx.shutdownChan
		for {
			// If restarts are disabled, or if the shutdown was a result of an
			// explicit stop call (instead of an unexpected error), stop
			// restarting the transport.
			if !bx.cfg.Restart || !restart {
				bx.setStateFrom(BLE_XPORT_STATE_STOPPED,
					BLE_XPORT_STATE_DORMANT)
				break
			}

			// Wait a second before the next restart.  This is necessary to
			// ensure the unix domain socket can be rebound.
			time.Sleep(time.Second)

			// Attempt to start the transport again.
			if err := bx.startOnce(); err != nil {
				// Start attempt failed.
				log.Debugf("Error starting BLE transport: %s",
					err.Error())
			} else {
				// Success.  Block until the transport shuts down.
				restart = <-bx.shutdownChan
			}
		}
	}()

	return nil
}

func (bx *BleXport) txNoSync(data []byte) error {
	log.Debugf("Tx to blehostd:\n%s", hex.Dump(data))
	return bx.client.TxToChild(data)
}

func (bx *BleXport) Tx(data []byte) error {
	if err := bx.blockUntilReady(); err != nil {
		return err
	}

	return bx.txNoSync(data)
}

func (bx *BleXport) RspTimeout() time.Duration {
	return bx.cfg.BlehostdRspTimeout
}

func (bx *BleXport) AcquireMaster(token interface{}) error {
	return bx.master.Acquire(token)
}

func (bx *BleXport) ReleaseMaster() {
	bx.master.Release()
}

func (bx *BleXport) StopWaitingForMaster(token interface{}, err error) {
	bx.master.StopWaiting(token, err)
}
