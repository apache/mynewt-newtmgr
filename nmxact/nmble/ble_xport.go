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
		PreferredMtu:          512,
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
	advertiser      *Advertiser
	cfg             XportCfg
	client          *unixchild.Client
	cm              ChrMgr
	d               *Dispatcher
	master          Master
	mtx             sync.Mutex
	readyBcast      nmxutil.Bcaster
	scanner         *BleScanner
	sesns           map[uint16]*BleSesn
	shutdownBlocker nmxutil.Blocker
	slave           nmxutil.SingleResource
	state           BleXportState
	stopChan        chan struct{}
	syncer          Syncer
	wg              sync.WaitGroup
}

func NewBleXport(cfg XportCfg) (*BleXport, error) {
	bx := &BleXport{
		cfg:        cfg,
		d:          NewDispatcher(),
		readyBcast: nmxutil.Bcaster{},
		slave:      nmxutil.NewSingleResource(),
		sesns:      map[uint16]*BleSesn{},
	}

	bx.advertiser = NewAdvertiser(bx)
	bx.scanner = NewBleScanner(bx)
	bx.master = NewMaster(bx, bx.scanner)

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
	return bx.scanner, nil
}

func (bx *BleXport) BuildAdvertiser() (adv.Advertiser, error) {
	// The transport only allows a single advertiser.  This is because the
	// slave privileges need to managed among all the advertise operations.
	return bx.advertiser, nil
}

func (bx *BleXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return NewBleSesn(bx, cfg, MASTER_PRIO_CONNECT)
}

func (bx *BleXport) addAccessListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_ACCESS_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "access")
	return bx.AddListener(key)
}

func (bx *BleXport) shutdown(restart bool, err error) {
	nmxutil.Assert(nmxutil.IsXport(err))

	// Prevents repeated shutdowns without keeping the mutex locked throughout
	// the duration of the shutdown.
	//
	// @return bool             true if a shutdown was successfully initiated.
	initiate := func() bool {
		bx.mtx.Lock()
		defer bx.mtx.Unlock()

		if bx.state == BLE_XPORT_STATE_STARTED ||
			bx.state == BLE_XPORT_STATE_STARTING {

			bx.state = BLE_XPORT_STATE_STOPPING
			return true
		} else {
			return false
		}
	}

	go func() {
		log.Debugf("Shutting down BLE transport")

		success := initiate()
		if !success {
			// Shutdown already in progress.
			return
		}

		bx.sesns = map[uint16]*BleSesn{}

		// Indicate error to all clients who are waiting for the master
		// resource.
		log.Debugf("Aborting BLE master")
		bx.master.Abort(err)

		// Indicate an error to all of this transport's listeners.  This
		// prevents them from blocking endlessly while awaiting a BLE message.
		log.Debugf("Stopping BLE dispatcher")
		bx.d.ErrorAll(err)

		// Reset controller so that all outstanding connections terminate.
		if bx.syncer.Synced() {
			ResetXact(bx)
		}

		bx.syncer.Stop()

		// Stop all of this transport's go routines.
		close(bx.stopChan)
		bx.wg.Wait()

		// Stop the unixchild instance (blehostd + socket).
		if bx.client != nil {
			log.Debugf("Stopping unixchild")
			bx.client.Stop()
		}

		bx.setStateFrom(BLE_XPORT_STATE_STOPPING, BLE_XPORT_STATE_STOPPED)

		// Indicate that the shutdown is complete.  If restarts are enabled on
		// this transport, this signals that the transport should be started
		// again.
		bx.shutdownBlocker.Unblock(restart)
	}()
}

func (bx *BleXport) waitForShutdown() bool {
	itf, _ := bx.shutdownBlocker.Wait(nmxutil.DURATION_FOREVER, nil)
	return itf.(bool)
}

func (bx *BleXport) blockingShutdown(restart bool, err error) {
	bx.shutdown(restart, err)
	bx.waitForShutdown()
}

func (bx *BleXport) blockUntilReady() error {
	var ch chan interface{}

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
		ch = bx.readyBcast.Listen()
	}
	bx.mtx.Unlock()

	itf := <-ch
	if itf == nil {
		return nil
	} else {
		return itf.(error)
	}
}

func (bx *BleXport) getState() BleXportState {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	return bx.state
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
		bx.readyBcast.SendAndClear(nil)
	case BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_DORMANT:
		bx.readyBcast.SendAndClear(
			nmxutil.NewXportError("BLE transport stopped"))
	default:
	}

	return true
}

func (bx *BleXport) Stop() error {
	bx.blockingShutdown(false, nmxutil.NewXportError("xport stopped"))
	return nil
}

func (bx *BleXport) startOnce() error {
	if !bx.setStateFrom(BLE_XPORT_STATE_STOPPED, BLE_XPORT_STATE_STARTING) {
		return nmxutil.NewXportError("BLE xport started twice")
	}

	bx.shutdownBlocker.Start()
	bx.stopChan = make(chan struct{})

	if err := bx.startUnixChild(); err != nil {
		bx.blockingShutdown(true, err)
		return err
	}

	// Listen for errors and data from the blehostd process.
	bx.wg.Add(1)
	go func() {
		defer bx.wg.Done()

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

	// Listen for events in the background:
	//     * sync loss
	//     * stack reset
	//     * GATT access
	bx.wg.Add(1)
	go func() {
		defer bx.wg.Done()

		accessl, err := bx.addAccessListener()
		if err != nil {
			bx.shutdown(true, err)
			return
		}
		defer bx.RemoveListener(accessl)

		resetCh := bx.syncer.ListenReset()
		syncCh := bx.syncer.ListenSync()

		for {
			select {
			case reasonItf, ok := <-resetCh:
				if ok {
					// Only process the reset event if the transport is not
					// already shutting down.  If in mid-shutdown, the reset
					// event was likely elicited by the shutdown itself.
					state := bx.getState()
					if state == BLE_XPORT_STATE_STARTED {

						reason := reasonItf.(int)
						bx.shutdown(true, nmxutil.NewXportError(fmt.Sprintf(
							"The BLE controller has been reset by the host; "+
								"reason=%s (%d)",
							ErrCodeToString(reason), reason)))
					}
				}

			case syncedItf, ok := <-syncCh:
				if ok {
					synced := syncedItf.(bool)
					if !synced {
						bx.shutdown(true, nmxutil.NewXportError(
							"BLE host <-> controller sync lost"))
					}
				}

			case err, ok := <-accessl.ErrChan:
				if ok {
					bx.shutdown(true, err)
				}

			case bm, ok := <-accessl.MsgChan:
				if ok {
					switch msg := bm.(type) {
					case *BleAccessEvt:
						if err := bx.cm.Access(bx, msg); err != nil {
							log.Debugf("Error sending access status: %s",
								err.Error())
						}
					}
				}

			case <-bx.stopChan:
				return
			}
		}
	}()

	if err := bx.syncer.Start(bx); err != nil {
		bx.blockingShutdown(true, err)
		return err
	}

	// Block until host and controller are synced.
	if err := bx.syncer.BlockUntilSynced(
		bx.cfg.SyncTimeout, bx.stopChan); err != nil {

		err = nmxutil.NewXportError(
			"Error waiting for host <-> controller sync: " + err.Error())
		bx.blockingShutdown(true, err)
		return err
	}

	// Generate a new random address if none was specified.
	var addr BleAddr
	if bx.cfg.RandAddr != nil {
		addr = *bx.cfg.RandAddr
	} else {
		var err error
		addr, err = GenRandAddrXact(bx)
		if err != nil {
			bx.blockingShutdown(true, err)
			return err
		}
	}

	// Set the random address on the controller.
	if err := SetRandAddrXact(bx, addr); err != nil {
		bx.blockingShutdown(true, err)
		return err
	}

	// Set the preferred ATT MTU in the host.
	if err := SetPreferredMtuXact(bx, bx.cfg.PreferredMtu); err != nil {
		bx.blockingShutdown(true, err)
		return err
	}

	if !bx.setStateFrom(BLE_XPORT_STATE_STARTING, BLE_XPORT_STATE_STARTED) {
		bx.blockingShutdown(true, nmxutil.NewXportError(
			"Internal error; BLE transport in unexpected state"))
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
	// the background.  This Go routine does not participate in the wait group
	// because it terminates itself independent of the others.
	go func() {
		// Block until transport shuts down.
		restart := bx.waitForShutdown()

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
				restart = bx.waitForShutdown()
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

func (bx *BleXport) AcquireMasterConnect(token interface{}) error {
	return bx.master.AcquireConnect(token)
}

func (bx *BleXport) AcquireMasterScan(token interface{}) error {
	return bx.master.AcquireScan(token)
}

func (bx *BleXport) ReleaseMaster() {
	bx.master.Release()
}

func (bx *BleXport) StopWaitingForMasterConnect(token interface{}, err error) {
	bx.master.StopWaitingConnect(token, err)
}

func (bx *BleXport) StopWaitingForMasterScan(token interface{}, err error) {
	bx.master.StopWaitingScan(token, err)
}

func (bx *BleXport) AcquireSlave(token interface{}) error {
	return <-bx.slave.Acquire(token)
}

func (bx *BleXport) ReleaseSlave() {
	bx.slave.Release()
}

func (bx *BleXport) StopWaitingForSlave(token interface{}, err error) {
	bx.slave.StopWaiting(token, err)
}

func (bx *BleXport) addSesn(connHandle uint16, s *BleSesn) {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	bx.sesns[connHandle] = s
}

func (bx *BleXport) removeSesn(connHandle uint16) *BleSesn {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	s := bx.sesns[connHandle]
	if s != nil {
		delete(bx.sesns, connHandle)
	}
	return s
}

func (bx *BleXport) findSesn(connHandle uint16) *BleSesn {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	return bx.sesns[connHandle]
}
