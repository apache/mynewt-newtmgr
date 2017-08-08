/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package nmble

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/util/unixchild"
	"mynewt.apache.org/newtmgr/nmxact/adv"
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
	cfg          XportCfg
	d            *Dispatcher
	client       *unixchild.Client
	state        BleXportState
	stopChan     chan struct{}
	shutdownChan chan bool
	readyBcast   nmxutil.Bcaster
	master       nmxutil.SingleResource
	slave        nmxutil.SingleResource
	randAddr     *BleAddr
	stateMtx     sync.Mutex
	scanner      *BleScanner
	advertiser   *Advertiser
	cm           ChrMgr
}

func NewBleXport(cfg XportCfg) (*BleXport, error) {
	bx := &BleXport{
		d:            NewDispatcher(),
		shutdownChan: make(chan bool),
		readyBcast:   nmxutil.Bcaster{},
		master:       nmxutil.NewSingleResource(),
		slave:        nmxutil.NewSingleResource(),
		cfg:          cfg,
	}

	return bx, nil
}

func (bx *BleXport) startUnixChild() error {
	config := unixchild.Config{
		SockPath:      bx.cfg.SockPath,
		ChildPath:     bx.cfg.BlehostdPath,
		ChildArgs:     []string{bx.cfg.DevPath, bx.cfg.SockPath},
		Depth:         10,
		MaxMsgSz:      10240,
		AcceptTimeout: bx.cfg.BlehostdAcceptTimeout,
	}

	bx.client = unixchild.New(config)

	if err := bx.client.Start(); err != nil {
		if unixchild.IsUcAcceptError(err) {
			err = nmxutil.NewXportError(
				"blehostd did not connect to socket; " +
					"controller not attached?")
		} else {
			err = nmxutil.NewXportError(
				"Failed to start child process: " + err.Error())
		}
		return err
	}

	return nil
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

func (bx *BleXport) BuildAdvertiser() (adv.Advertiser, error) {
	// The transport only allows a single advertiser.  This is because the
	// slave privileges need to managed among all the advertise operations.
	if bx.advertiser == nil {
		bx.advertiser = NewAdvertiser(bx)
	}

	return bx.advertiser, nil
}

func (bx *BleXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return NewBleSesn(bx, cfg)
}

func (bx *BleXport) addSyncListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_SYNC_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "sync")
	return bx.AddListener(key)
}

func (bx *BleXport) addResetListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_RESET_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "reset")
	return bx.AddListener(key)
}

func (bx *BleXport) addAccessListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_ACCESS_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "access")
	return bx.AddListener(key)
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

	key := SeqKey(req.Seq)
	bl, err := bx.AddListener(key)
	if err != nil {
		return false, err
	}
	defer bx.RemoveListener(bl)

	if err := bx.txNoSync(j); err != nil {
		return false, err
	}
	for {
		select {
		case err := <-bl.ErrChan:
			return false, err
		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSyncRsp:
				return msg.Synced, nil
			}
		}
	}
}

func (bx *BleXport) initialSyncCheck() (bool, *Listener, error) {
	bl, err := bx.addSyncListener()
	if err != nil {
		return false, nil, err
	}

	synced, err := bx.querySyncStatus()
	if err != nil {
		bx.RemoveListener(bl)
		return false, nil, err
	}

	return synced, bl, nil
}

func (bx *BleXport) shutdown(restart bool, err error) {
	nmxutil.Assert(nmxutil.IsXport(err))

	log.Debugf("Shutting down BLE transport")

	bx.stateMtx.Lock()

	var fullyStarted bool
	var already bool

	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		already = false
		fullyStarted = true
	case BLE_XPORT_STATE_STARTING:
		already = false
		fullyStarted = false
	default:
		already = true
	}

	if !already {
		bx.state = BLE_XPORT_STATE_STOPPING
	}

	bx.stateMtx.Unlock()

	if already {
		// Shutdown already in progress.
		return
	}

	// Indicate error to all clients who are waiting for the master resource.
	log.Debugf("Aborting BLE master")
	bx.master.Abort(err)

	// Indicate an error to all of this transport's listeners.  This prevents
	// them from blocking endlessly while awaiting a BLE message.
	log.Debugf("Stopping BLE dispatcher")
	bx.d.ErrorAll(err)

	synced, err := bx.querySyncStatus()
	if err == nil && synced {
		// Reset controller so that all outstanding connections terminate.
		ResetXact(bx)
	}

	// Stop all of this transport's go routines.
	close(bx.stopChan)

	// Stop the unixchild instance (blehostd + socket).
	if bx.client != nil {
		log.Debugf("Stopping unixchild")
		bx.client.Stop()
	}

	bx.setStateFrom(BLE_XPORT_STATE_STOPPING, BLE_XPORT_STATE_STOPPED)

	// Indicate that the shutdown is complete.  If restarts are enabled on this
	// transport, this signals that the transport should be started again.
	if fullyStarted {
		bx.shutdownChan <- restart
	}
}

func (bx *BleXport) blockUntilReady() error {
	var ch chan interface{}

	bx.stateMtx.Lock()
	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		// Already started; don't block.
		bx.stateMtx.Unlock()
		return nil

	case BLE_XPORT_STATE_DORMANT:
		// Not in the process of starting; the user will be waiting forever.
		bx.stateMtx.Unlock()
		return fmt.Errorf("Attempt to use BLE transport without starting it")

	default:
		ch = bx.readyBcast.Listen()
	}
	bx.stateMtx.Unlock()

	itf := <-ch
	if itf == nil {
		return nil
	} else {
		return itf.(error)
	}
}

func (bx *BleXport) getState() BleXportState {
	bx.stateMtx.Lock()
	defer bx.stateMtx.Unlock()

	return bx.state
}

func (bx *BleXport) setStateFrom(from BleXportState, to BleXportState) bool {
	bx.stateMtx.Lock()
	defer bx.stateMtx.Unlock()

	if bx.state != from {
		return false
	}

	bx.state = to
	switch bx.state {
	case BLE_XPORT_STATE_STARTED:
		bx.readyBcast.SendAndClear(nil)
	case BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_DORMANT:
		bx.readyBcast.SendAndClear(
			nmxutil.NewXportError("BLE transport stopped"))
	default:
	}

	return true
}

func (bx *BleXport) Stop() error {
	bx.shutdown(false, nmxutil.NewXportError("xport stopped"))
	return nil
}

func (bx *BleXport) startOnce() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_STARTING) {
		return nmxutil.NewXportError("BLE xport started twice")
	}

	bx.stopChan = make(chan struct{})

	if err := bx.startUnixChild(); err != nil {
		bx.shutdown(true, err)
		return err
	}

	// Listen for errors and data from the blehostd process.
	go func() {
		for {
			select {
			case err := <-bx.client.ErrChild:
				err = nmxutil.NewXportError("BLE transport error: " +
					err.Error())
				bx.shutdown(true, err)

			case buf := <-bx.client.FromChild:
				if len(buf) != 0 {
					log.Debugf("Receive from blehostd:\n%s", hex.Dump(buf))
					bx.d.Dispatch(buf)
				}

			case <-bx.stopChan:
				return
			}
		}
	}()

	synced, syncl, err := bx.initialSyncCheck()
	if err != nil {
		bx.shutdown(true, err)
		return err
	}

	// Block until host and controller are synced.
	if !synced {
	SyncLoop:
		for {
			select {
			case err := <-syncl.ErrChan:
				bx.shutdown(true, err)
				return err
			case bm := <-syncl.MsgChan:
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
			case <-bx.stopChan:
				return nmxutil.NewXportError("Transport startup aborted")
			}
		}
	}

	// Host and controller are synced.  Listen for events in the background:
	//     * sync loss
	//     * stack reset
	//     * GATT access
	go func() {
		resetl, err := bx.addResetListener()
		if err != nil {
			bx.shutdown(true, err)
			return
		}
		defer bx.RemoveListener(resetl)

		accessl, err := bx.addAccessListener()
		if err != nil {
			bx.shutdown(true, err)
			return
		}
		defer bx.RemoveListener(accessl)

		for {
			select {
			case err := <-syncl.ErrChan:
				bx.shutdown(true, err)
				return
			case bm := <-syncl.MsgChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					if !msg.Synced {
						bx.shutdown(true, nmxutil.NewXportError(
							"BLE host <-> controller sync lost"))
					}
				}

			case err := <-resetl.ErrChan:
				bx.shutdown(true, err)
				return
			case bm := <-resetl.MsgChan:
				switch msg := bm.(type) {
				case *BleResetEvt:
					// Only process the reset event if the transport is not
					// already shutting down.  If in mid-shutdown, the reset
					// event was likely elicited by the shutdown itself.
					state := bx.getState()
					if state == BLE_XPORT_STATE_STARTING ||
						state == BLE_XPORT_STATE_STARTED {

						bx.shutdown(true, nmxutil.NewXportError(fmt.Sprintf(
							"The BLE controller has been reset by the host; "+
								"reason=%s (%d)",
							ErrCodeToString(msg.Reason), msg.Reason)))
						return
					}
				}

			case err := <-accessl.ErrChan:
				bx.shutdown(true, err)
				return
			case bm := <-accessl.MsgChan:
				switch msg := bm.(type) {
				case *BleAccessEvt:
					if err := bx.cm.Access(bx, msg); err != nil {
						log.Debugf("Error sending access status: %s",
							err.Error())
					}
				}

			case <-bx.stopChan:
				return
			}
		}
	}()

	// Generate a new random address if none was specified.
	if bx.randAddr == nil {
		addr, err := GenRandAddrXact(bx)
		if err != nil {
			bx.shutdown(true, err)
			return err
		}

		bx.randAddr = &addr
	}

	// Set the random address on the controller.
	if err := SetRandAddrXact(bx, *bx.randAddr); err != nil {
		bx.shutdown(true, err)
		return err
	}

	// Set the preferred ATT MTU in the host.
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

// Transmit data to blehostd; host-controller sync not required.
func (bx *BleXport) txNoSync(data []byte) error {
	log.Debugf("Tx to blehostd:\n%s", hex.Dump(data))
	return bx.client.TxToChild(data)
}

// Transmit data to blehostd.  If the host and controller are not synced, this
// function blocks until they are (or until the sync fails).
func (bx *BleXport) Tx(data []byte) error {
	if err := bx.blockUntilReady(); err != nil {
		return err
	}

	return bx.txNoSync(data)
}

func (bx *BleXport) SetServices(svcs []BleSvc) error {
	return bx.cm.SetServices(bx, svcs)
}

func (bx *BleXport) AddListener(key ListenerKey) (*Listener, error) {
	listener := NewListener()
	if err := bx.d.AddListener(key, listener); err != nil {
		return nil, err
	}
	return listener, nil
}

func (bx *BleXport) RemoveListener(listener *Listener) *ListenerKey {
	return bx.d.RemoveListener(listener)
}

func (bx *BleXport) RemoveKey(key ListenerKey) *Listener {
	return bx.d.RemoveKey(key)
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

func (bx *BleXport) AcquireSlave(token interface{}) error {
	return bx.slave.Acquire(token)
}

func (bx *BleXport) ReleaseSlave() {
	bx.slave.Release()
}

func (bx *BleXport) StopWaitingForSlave(token interface{}, err error) {
	bx.slave.StopWaiting(token, err)
}
