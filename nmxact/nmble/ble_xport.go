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
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/task"
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

	// How long to allow for the host and controller to sync at startup.
	// Default: 2 seconds.
	SyncTimeout time.Duration

	// The static random address to use.  Set to nil if one should be
	// generated.
	// Default: nil (auto-generate).
	RandAddr *BleAddr

	// The value to specify during ATT MTU exchange.
	// Default: 264.
	PreferredMtu uint16

	// Additional args to blehostd
	BlehostdArgs []string

	// Whether to restart automatically when an error is detected.
	// Default: true.
	Restart bool
}

// Implements xport.Xport.
type BleXport struct {
	// Whether the transport should restart on failure.
	enabled bool

	shuttingDown bool

	advertiser *Advertiser
	cfg        XportCfg
	client     *unixchild.Client
	cm         ChrMgr
	d          *Dispatcher
	master     Master
	slave      nmxutil.SingleResource
	stopChan   chan struct{}
	syncer     Syncer
	tq         task.TaskQueue
	wg         sync.WaitGroup

	// Map of open sessions (key: connection handle).
	sesns map[uint16]*NakedSesn

	// Protects `enabled`.
	mtx sync.Mutex
}

func (bx *BleXport) runTask(fn func() error) error {
	err := bx.tq.Run(fn)
	if err == task.InactiveError {
		return nmxutil.NewXportError("attempt to use inactive BLE transport")
	}
	return err
}

func (bx *BleXport) enqueueShutdown(cause error) chan error {
	return bx.tq.Enqueue(func() error { return bx.shutdown(cause) })
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

	config.ChildArgs = append(config.ChildArgs, bx.cfg.BlehostdArgs...)
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

func (bx *BleXport) addAccessListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_ACCESS_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "access")
	return bx.AddListener(key)
}

func (bx *BleXport) startSyncer() error {
	syncCh, resetCh, err := bx.syncer.Start(bx)
	if err != nil {
		return err
	}

	initialSyncCh := make(chan struct{})

	// Listen for events in the background:
	//     * sync loss
	//     * stack reset
	//     * GATT access
	bx.wg.Add(1)
	go func() {
		defer bx.wg.Done()

		accessl, err := bx.addAccessListener()
		if err != nil {
			bx.enqueueShutdown(err)
			return
		}
		defer bx.RemoveListener(accessl)

		for {
			select {
			case reason, ok := <-resetCh:
				if ok {
					// Ignore resets prior to initial sync.
					if initialSyncCh == nil {
						bx.enqueueShutdown(nmxutil.NewXportError(fmt.Sprintf(
							"The BLE controller has been reset by the host; "+
								"reason=%s (%d)",
							ErrCodeToString(reason), reason)))
					}
				}

			case synced, ok := <-syncCh:
				if ok {
					if !synced {
						bx.enqueueShutdown(nmxutil.NewXportError(
							"BLE host <-> controller sync lost"))
					} else if initialSyncCh != nil {
						close(initialSyncCh)
						initialSyncCh = nil
					}
				}

			case err, ok := <-accessl.ErrChan:
				if ok {
					bx.enqueueShutdown(err)
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

	bx.syncer.Refresh()

	// Block until host and controller are synced.
	select {
	case <-initialSyncCh:

	case <-time.After(bx.cfg.SyncTimeout):
		return nmxutil.NewXportError(fmt.Sprintf(
			"Error waiting for host <-> controller sync: timeout (%s)",
			bx.cfg.SyncTimeout.String()))

	case <-bx.stopChan:
		return nmxutil.NewXportError("stopped")
	}

	return nil
}

func (bx *BleXport) setAddr() error {
	// Generate a new random address if none was specified.
	var addr BleAddr
	if bx.cfg.RandAddr != nil {
		addr = *bx.cfg.RandAddr
	} else {
		var err error
		addr, err = GenRandAddrXact(bx)
		if err != nil {
			return err
		}
	}

	// Set the random address on the controller.
	if err := SetRandAddrXact(bx, addr); err != nil {
		return err
	}

	return nil
}

func (bx *BleXport) shutdown(cause error) error {
	nmxutil.Assert(nmxutil.IsXport(cause))

	initiate := func() error {
		bx.mtx.Lock()
		defer bx.mtx.Unlock()

		if bx.shuttingDown {
			return nmxutil.NewXportError("BLE xport stopped more than once")
		}
		bx.shuttingDown = true
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}
	defer func() {
		bx.mtx.Lock()
		defer bx.mtx.Unlock()

		bx.shuttingDown = false
	}()

	log.Debugf("Shutting down BLE transport - %s", cause.Error())

	bx.sesns = map[uint16]*NakedSesn{}

	// Stop monitoring host-controller sync.
	synced := bx.syncer.Synced()
	log.Debugf("Stopping BLE syncer")
	bx.syncer.Stop()

	if synced {
		// Reset controller so that all outstanding connections terminate.
		log.Debugf("Resetting host")
		ResetXact(bx)
	}

	if err := bx.tq.StopNoWait(cause); err != nil {
		// Already shut down.
		return err
	}

	// Indicate error to all clients who are waiting for the master
	// resource.
	log.Debugf("Aborting BLE master")
	bx.master.Abort(cause)

	// Indicate an error to all of this transport's listeners.  This
	// prevents them from blocking endlessly while awaiting a BLE message.
	log.Debugf("Stopping BLE dispatcher")
	bx.d.ErrorAll(cause)

	// Stop all of this transport's go routines.
	close(bx.stopChan)

	// Stop the unixchild instance (blehostd + socket).
	if bx.client != nil {
		log.Debugf("Stopping unixchild")
		bx.client.Stop()
	}

	bx.wg.Wait()

	return nil
}

// Transmit data to blehostd; host-controller sync not required.
func (bx *BleXport) txNoSync(data []byte) error {
	log.Debugf("Tx to blehostd:\n%s", hex.Dump(data))
	return bx.client.TxToChild(data)
}

func (bx *BleXport) startEvent() error {
	fail := func(err error) error {
		bx.shutdown(nmxutil.NewXportError(err.Error()))
		return err
	}

	// Make sure we don't think we are still in sync with the controller.  If
	// we fail early, we don't want to try sending a reset command.
	bx.syncer.Stop()

	if err := bx.startUnixChild(); err != nil {
		return fail(err)
	}

	// Listen for errors and data from the blehostd process.
	bx.wg.Add(1)
	go func() {
		defer bx.wg.Done()

		for {
			select {
			case err, ok := <-bx.client.ErrChild:
				if ok {
					bx.enqueueShutdown(nmxutil.NewXportError(
						"BLE transport error: " + err.Error()))
				}

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

	// Listen for sync and reset; blocks until initial sync.
	if err := bx.startSyncer(); err != nil {
		return fail(err)
	}

	// Set the random address.
	if err := bx.setAddr(); err != nil {
		return fail(err)
	}

	// Set the preferred ATT MTU in the host.
	if err := SetPreferredMtuXact(bx, bx.cfg.PreferredMtu); err != nil {
		return fail(err)
	}

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// API                                                                       //
///////////////////////////////////////////////////////////////////////////////

func (bx *BleXport) Advertiser() *Advertiser {
	return bx.advertiser
}

func (bx *BleXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return NewBleSesn(bx, cfg)
}

func (bx *BleXport) Start() error {
	initialize := func() error {
		bx.mtx.Lock()
		defer bx.mtx.Unlock()

		if bx.enabled {
			return nmxutil.NewXportError("BLE xport double start")
		}

		bx.enabled = true
		return nil
	}

	if err := initialize(); err != nil {
		return err
	}

	startTask := func() chan error {
		if err := bx.tq.Start(10); err != nil {
			nmxutil.Assert(false)
		}
		bx.stopChan = make(chan struct{})
		return bx.tq.Enqueue(bx.startEvent)
	}

	// Enqueue start event and block until it completes.  If this first attempt
	// fails, abort the start procedure completely (don't enter the retry
	// loop).
	if err := <-startTask(); err != nil {
		bx.mtx.Lock()
		bx.enabled = false
		bx.mtx.Unlock()
		return err
	}

	// Run and restart task queue in the background.
	go func() {
		isEnabled := func() bool {
			bx.mtx.Lock()
			defer bx.mtx.Unlock()

			return bx.enabled
		}

		for {
			<-bx.stopChan
			bx.wg.Wait()

			if !bx.cfg.Restart || !isEnabled() {
				break
			}

			startTask()
		}
	}()

	return nil
}

func (bx *BleXport) Stop() error {
	fn := func() error {
		initialize := func() error {
			bx.mtx.Lock()
			defer bx.mtx.Unlock()

			if !bx.enabled {
				return fmt.Errorf("BLE xport double stop")
			}
			bx.enabled = false
			return nil
		}

		if err := initialize(); err != nil {
			return err
		}

		cause := nmxutil.NewXportError("BLE xport manually stopped")

		if err := bx.shutdown(cause); err != nil {
			return err
		}

		return nil
	}

	return bx.runTask(fn)
}

func (bx *BleXport) Restart(reason string) {
	cause := nmxutil.NewXportError("Restarting BLE transport; " + reason)
	bx.enqueueShutdown(cause)
}

// Transmit data to blehostd.  If the host and controller are not synced, this
// function blocks until they are (or until the sync fails).
func (bx *BleXport) Tx(data []byte) error {
	fn := func() error {
		return bx.txNoSync(data)
	}

	return bx.runTask(fn)
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

func (bx *BleXport) GetMasterSecondary() Preemptable {
	return bx.master.GetSecondary()
}

func (bx *BleXport) SetMasterSecondary(s Preemptable) error {
	return bx.master.SetSecondary(s)
}

func (bx *BleXport) AcquireMasterPrimary(token interface{}) error {
	return bx.master.AcquirePrimary(token)
}

func (bx *BleXport) AcquireMasterSecondary() error {
	return bx.master.AcquireSecondary()
}

func (bx *BleXport) ReleaseMaster() {
	bx.master.Release()
}

func (bx *BleXport) StopWaitingForMasterPrimary(token interface{}, err error) {
	bx.master.StopWaitingPrimary(token, err)
}

func (bx *BleXport) StopWaitingForMasterSecondary(err error) {
	bx.master.StopWaitingSecondary(err)
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

func (bx *BleXport) AddSesn(connHandle uint16, s *NakedSesn) {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	bx.sesns[connHandle] = s
}

func (bx *BleXport) RemoveSesn(connHandle uint16) *NakedSesn {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	s := bx.sesns[connHandle]
	if s != nil {
		delete(bx.sesns, connHandle)
	}
	return s
}

func (bx *BleXport) FindSesn(connHandle uint16) *NakedSesn {
	bx.mtx.Lock()
	defer bx.mtx.Unlock()

	return bx.sesns[connHandle]
}

func NewXportCfg() XportCfg {
	return XportCfg{
		BlehostdAcceptTimeout: time.Second,
		BlehostdRspTimeout:    10 * time.Second,
		SyncTimeout:           2 * time.Second,
		PreferredMtu:          512,
		Restart:               true,
	}
}

func NewBleXport(cfg XportCfg) (*BleXport, error) {
	bx := &BleXport{
		cfg:   cfg,
		d:     NewDispatcher(),
		slave: nmxutil.NewSingleResource(),
		sesns: map[uint16]*NakedSesn{},
	}

	bx.tq = task.NewTaskQueue("ble_xport")

	bx.advertiser = NewAdvertiser(bx)
	bx.master = NewMaster(bx)

	return bx, nil
}
