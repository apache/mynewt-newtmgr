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
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/task"
)

type Notification struct {
	Chr        *Characteristic
	Data       []byte
	Indication bool
}

type NotifyListener struct {
	NotifyChan chan Notification
	ErrChan    chan error
}

func NewNotifyListener() *NotifyListener {
	return &NotifyListener{
		NotifyChan: make(chan Notification),
		ErrChan:    make(chan error, 1),
	}
}

// Sent up to the parent session to indicate that IO is required to complete a
// pairing procedure.
type SmIoDemand struct {
	// Mandatory.
	Action BleSmAction

	// Only valid when Action==BLE_SM_ACTION_NUMCMP.
	Numcmp uint32
}

// Sent down from the parent session in response to an SM IO demand.
type SmIo struct {
	// Mandatory.
	Action BleSmAction

	// Action dependent.
	Oob          []byte // OOB only.
	Passkey      uint32 // Display and input only.
	NumcmpAccept bool   // Numcmp only.
}

// Implements a low-level BLE connection.  Objets of this type must never be
// reused; after a disconnect, a new object should be created if you wish to
// reconnect to the peer.
//
// Starts Goroutines on:
// * Successful call to Connect().
// * Successful call to Inherit().
//
// Stops Goroutines on:
// * Successful call to Stop().
// * Unsolicited disconnect.
type Conn struct {
	bx         *BleXport
	rxvr       *Receiver
	attMtu     uint16
	profile    Profile
	desc       BleConnDesc
	connHandle uint16
	notifyMap  map[*Characteristic]*NotifyListener
	wg         sync.WaitGroup

	// Indicates a disconnect to the user of this type.
	disconnectChan chan error

	// Closes when the connection drops; used for Goroutine cleanup.
	dropChan chan struct{}

	smIoChan chan SmIoDemand

	// Allows blocking initiate-security procedures.
	encBlocker nmxutil.Blocker

	// The queue of actions that run in the main loop.
	tq task.TaskQueue

	// Protects:
	// * connHandle
	// * notifyMap
	mtx sync.Mutex
}

func NewConn(bx *BleXport) *Conn {
	c := &Conn{
		bx:             bx,
		rxvr:           NewReceiver(nmxutil.GetNextId(), bx, 1),
		connHandle:     BLE_CONN_HANDLE_NONE,
		attMtu:         BLE_ATT_MTU_DFLT,
		disconnectChan: make(chan error, 1),
		dropChan:       make(chan struct{}),
		smIoChan:       make(chan SmIoDemand, 1),
		notifyMap:      map[*Characteristic]*NotifyListener{},
	}

	return c
}

func (c *Conn) DisconnectChan() <-chan error {
	return c.disconnectChan
}

func (c *Conn) SmIoDemandChan() <-chan SmIoDemand {
	return c.smIoChan
}

func (c *Conn) abortNotifyListeners(err error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, nl := range c.notifyMap {
		nl.ErrChan <- err
		close(nl.NotifyChan)
		close(nl.ErrChan)
	}
}

func (c *Conn) initTaskQueue() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.tq.Active() {
		return fmt.Errorf("Attempt to start BLE conn twice")
	}

	c.tq = task.NewTaskQueue("conn")
	if err := c.tq.Start(10); err != nil {
		nmxutil.Assert(false)
		return err
	}

	return nil
}

func (c *Conn) runTask(fn func() error) error {
	err := c.tq.Run(fn)
	if err == task.InactiveError {
		return nmxutil.NewSesnClosedError(
			"attempt to use closed BLE connection")
	}
	return err
}

func (c *Conn) writeHandle(handle uint16, payload []byte,
	name string) error {

	r := NewBleWriteReq()
	r.ConnHandle = c.connHandle
	r.AttrHandle = int(handle)
	r.Data.Bytes = payload

	bl, err := c.rxvr.AddListener(name, SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener(name, bl)

	if err := write(c.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (c *Conn) writeHandleNoRsp(handle uint16, payload []byte,
	name string) error {

	r := NewBleWriteCmdReq()
	r.ConnHandle = c.connHandle
	r.AttrHandle = int(handle)
	r.Data.Bytes = payload

	bl, err := c.rxvr.AddListener(name, SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener(name, bl)

	if err := writeCmd(c.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (c *Conn) enqueueShutdown(cause error) chan error {
	return c.tq.Enqueue(func() error { return c.shutdown(cause) })
}

func (c *Conn) runShutdown(cause error) error {
	return <-c.enqueueShutdown(cause)
}

func (c *Conn) shutdown(cause error) error {
	if err := c.tq.StopNoWait(cause); err != nil {
		// Connection already shut down.
		return err
	}

	if c.connHandle != BLE_CONN_HANDLE_NONE {
		c.terminate()
		select {
		case <-c.dropChan:
		case <-time.After(time.Second * 30):
			// This shouldn't happen.  Either blehostd is buggy or an event got
			// dropped.
			c.bx.Restart("No disconnect event received after 30 seconds")
		}
	}

	c.rxvr.RemoveAll("shutdown")
	c.rxvr.WaitUntilNoListeners()

	c.connHandle = BLE_CONN_HANDLE_NONE

	c.abortNotifyListeners(cause)

	c.wg.Wait()

	c.disconnectChan <- cause
	close(c.disconnectChan)

	return nil
}

func (c *Conn) newDisconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) connection=%s",
		ErrCodeToString(reason), reason, c.desc.String())

	return nmxutil.NewBleSesnDisconnectError(reason, str)
}

// Listens for events in the background.
func (c *Conn) eventListen(bl *Listener) error {
	// Terminates on:
	// * BLE listener error (also triggered by txvr being stopped).
	// * Receive of disconnect event.
	//
	// On terminate, this Goroutine shuts the connection object down.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.rxvr.RemoveListener("connect", bl)
		defer close(c.dropChan)

		for {
			select {
			case err, ok := <-bl.ErrChan:
				if ok {
					c.enqueueShutdown(err)
				}
				return

			case bm, ok := <-bl.MsgChan:
				if ok {
					switch msg := bm.(type) {
					case *BleMtuChangeEvt:
						if msg.Status != 0 {
							err := StatusError(MSG_OP_EVT,
								MSG_TYPE_MTU_CHANGE_EVT,
								msg.Status)
							log.Debugf(err.Error())
						} else {
							log.Debugf("BLE ATT MTU updated; from=%d to=%d",
								c.attMtu, msg.Mtu)
							c.attMtu = msg.Mtu
						}

					case *BleEncChangeEvt:
						var err error
						if msg.Status != 0 {
							err = StatusError(MSG_OP_EVT,
								MSG_TYPE_ENC_CHANGE_EVT,
								msg.Status)
							log.Debugf(err.Error())
						} else {
							log.Debugf("Connection encrypted; conn_handle=%d",
								msg.ConnHandle)
							c.updateDescriptor()
						}

						// Unblock any initiate-security procedures.
						c.encBlocker.Unblock(err)

					case *BlePasskeyEvt:
						c.smIoChan <- SmIoDemand{
							Action: msg.Action,
							Numcmp: msg.Numcmp,
						}

					case *BleDisconnectEvt:
						c.enqueueShutdown(c.newDisconnectError(msg.Reason))
						return

					default:
					}
				}
			}
		}
	}()
	return nil
}

func (c *Conn) rxNotify(msg *BleNotifyRxEvt) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	chr := c.profile.FindChrByHandle(uint16(msg.AttrHandle))
	if chr == nil {
		return
	}

	nl := c.notifyMap[chr]
	if nl == nil {
		return
	}

	nl.NotifyChan <- Notification{
		Chr:        chr,
		Data:       msg.Data.Bytes,
		Indication: msg.Indication,
	}
}

// Listens for incoming notifications and indications.
func (c *Conn) notifyListen() error {
	key := TchKey(MSG_TYPE_NOTIFY_RX_EVT, int(c.connHandle))
	bl, err := c.rxvr.AddListener("notifications", key)
	if err != nil {
		return err
	}

	// Terminates on:
	// * BLE listener error (triggered by txvr being stopped)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.rxvr.RemoveListener("notifications", bl)

		for {
			select {
			case <-bl.ErrChan:
				return

			case bm, ok := <-bl.MsgChan:
				if ok {
					switch msg := bm.(type) {
					case *BleNotifyRxEvt:
						c.rxNotify(msg)
					}
				}
			}
		}
	}()

	return nil
}

func (c *Conn) updateDescriptor() error {
	d, err := ConnFindXact(c.bx, c.connHandle)
	if err != nil {
		return err
	}

	c.desc = d
	return nil
}

func (c *Conn) finalizeConnection(connHandle uint16,
	eventListener *Listener) error {

	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.connHandle = connHandle

	// Listen for events in the background.
	if err := c.eventListen(eventListener); err != nil {
		return err
	}

	// Listen for notifications in the background.
	if err := c.notifyListen(); err != nil {
		return err
	}

	if err := c.updateDescriptor(); err != nil {
		return err
	}

	return nil
}

func (c *Conn) IsConnected() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	return c.connHandle != BLE_CONN_HANDLE_NONE
}

func (c *Conn) ConnInfo() BleConnDesc {
	return c.desc
}

func (c *Conn) AttMtu() uint16 {
	return c.attMtu
}

func (c *Conn) Profile() *Profile {
	return &c.profile
}

func (c *Conn) Connect(ownAddrType BleAddrType, peer BleDev,
	timeout time.Duration) error {

	if err := c.initTaskQueue(); err != nil {
		return err
	}

	fn := func() error {
		r := NewBleConnectReq()
		r.OwnAddrType = ownAddrType
		r.PeerAddrType = peer.AddrType
		r.PeerAddr = peer.Addr
		r.DurationMs = int(timeout / time.Millisecond)

		bl, err := c.rxvr.AddListener("connect", SeqKey(r.Seq))
		if err != nil {
			return err
		}

		// Tell blehostd to initiate connection.
		connHandle, err := connect(c.bx, bl, r)
		if err != nil {
			bhe := nmxutil.ToBleHost(err)
			if bhe != nil && bhe.Status == ERR_CODE_EDONE {
				// Already connected.
				c.rxvr.RemoveListener("connect", bl)
				err := fmt.Errorf("Already connected to peer %s",
					peer.String())
				return err
			} else if !nmxutil.IsXport(err) {
				// The transport did not restart; always attempt to cancel the
				// connect operation.  In most cases, the host has already
				// stopped connecting and will respond with an "ealready" error
				// that can be ignored.
				if err := c.connCancel(); err != nil {
					log.Debugf("Failed to cancel connect in progress: %s",
						err.Error())
				}
			}

			c.rxvr.RemoveListener("connect", bl)
			return err
		}

		return c.finalizeConnection(connHandle, bl)
	}

	if err := c.runTask(fn); err != nil {
		c.runShutdown(err)
		return err
	}

	return nil
}

// Opens the session for an already-established BLE connection.
func (c *Conn) Inherit(connHandle uint16, bl *Listener) error {
	if err := c.initTaskQueue(); err != nil {
		return err
	}

	fn := func() error {
		if err := c.finalizeConnection(connHandle, bl); err != nil {
			return err
		}

		return nil
	}

	if err := c.runTask(fn); err != nil {
		return c.runShutdown(err)
	}

	return nil
}

func (c *Conn) ExchangeMtu() error {
	fn := func() error {
		r := NewBleExchangeMtuReq()
		r.ConnHandle = c.connHandle

		bl, err := c.rxvr.AddListener("exchange-mtu", SeqKey(r.Seq))
		if err != nil {
			return err
		}
		defer c.rxvr.RemoveListener("exchange-mtu", bl)

		mtu, err := exchangeMtu(c.bx, bl, r)
		if isExchangeMtuError(err) {
			return err
		}

		c.attMtu = uint16(mtu)
		return nil
	}

	return c.runTask(fn)
}

func (c *Conn) DiscoverSvcs() error {
	fn := func() error {
		svcs, err := c.discAllSvcs()
		if err != nil {
			return err
		}

		if err := c.discAllChrs(svcs); err != nil {
			return err
		}

		c.profile.SetServices(svcs)

		return nil
	}

	return c.runTask(fn)
}

func (c *Conn) WriteChr(chr *Characteristic, payload []byte,
	name string) error {

	fn := func() error {
		return c.writeHandle(chr.ValHandle, payload, name)
	}

	return c.runTask(fn)
}

func (c *Conn) WriteChrNoRsp(chr *Characteristic, payload []byte,
	name string) error {

	fn := func() error {
		return c.writeHandleNoRsp(chr.ValHandle, payload, name)
	}

	return c.runTask(fn)
}

func (c *Conn) Subscribe(chr *Characteristic) error {
	fn := func() error {
		uuid := BleUuid{CccdUuid, [16]byte{}}
		dsc := FindDscByUuid(chr, uuid)
		if dsc == nil {
			return fmt.Errorf("Cannot subscribe to characteristic %s; no CCCD",
				chr.Uuid.String())
		}

		var payload []byte
		switch chr.SubscribeType() {
		case BLE_DISC_CHR_PROP_NOTIFY:
			payload = []byte{1, 0}
		case BLE_DISC_CHR_PROP_INDICATE:
			payload = []byte{2, 0}
		default:
			return fmt.Errorf("Cannot subscribe to characteristic %s; "+
				"properties indicate unsubscribable", chr.Uuid.String())
		}

		return c.writeHandle(dsc.Handle, payload, "subscribe")
	}

	return c.runTask(fn)
}

func (c *Conn) ListenForNotifications(chr *Characteristic) (
	*NotifyListener, error) {

	var nl *NotifyListener

	fn := func() error {
		c.mtx.Lock()
		defer c.mtx.Unlock()

		if _, ok := c.notifyMap[chr]; ok {
			return fmt.Errorf(
				"Already listening for notifications on characteristic %s",
				chr.String())
		}

		nl = NewNotifyListener()
		c.notifyMap[chr] = nl

		return nil
	}

	if err := c.runTask(fn); err != nil {
		return nil, err
	}

	return nl, nil
}

func (c *Conn) InitiateSecurity() error {
	fn := func() error {
		r := NewBleSecurityInitiateReq()
		r.ConnHandle = c.connHandle

		bl, err := c.rxvr.AddListener("security-initiate", SeqKey(r.Seq))
		if err != nil {
			return err
		}
		defer c.rxvr.RemoveListener("security-initiate", bl)

		c.encBlocker.Start()
		if err := securityInitiate(c.bx, bl, r); err != nil {
			return err
		}

		encErr, tmoErr := c.encBlocker.Wait(time.Second*15, c.dropChan)
		if encErr != nil {
			return encErr.(error)
		}
		if tmoErr != nil {
			return fmt.Errorf("Timeout waiting for security to be established")
		}

		return nil
	}

	return c.runTask(fn)
}

func (c *Conn) SmInjectIo(io SmIo) error {
	r := NewBleSmInjectIoReq()
	r.ConnHandle = c.connHandle
	r.Action = io.Action
	r.OobData.Bytes = io.Oob
	r.Passkey = io.Passkey
	r.NumcmpAccept = io.NumcmpAccept

	bl, err := c.rxvr.AddListener("inject-sm-io", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("inject-sm-io", bl)

	return smInjectIo(c.bx, bl, r)
}

func (c *Conn) Stop() error {
	return c.runShutdown(fmt.Errorf("stopped"))
}

func (c *Conn) discAllDscsOnce(startHandle uint16, endHandle uint16) (
	[]*Descriptor, error) {

	r := NewBleDiscAllDscsReq()
	r.ConnHandle = c.connHandle
	r.StartHandle = int(startHandle)
	r.EndHandle = int(endHandle)

	bl, err := c.rxvr.AddListener("disc-all-dscs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-dscs", bl)

	rawDscs, err := discAllDscs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	dscs := make([]*Descriptor, len(rawDscs))
	for i, rd := range rawDscs {
		dscs[i] = &Descriptor{
			Uuid:   rd.Uuid,
			Handle: uint16(rd.Handle),
		}
	}

	return dscs, nil
}

func (c *Conn) discAllDscs(chrs []*Characteristic, svcEndHandle uint16) error {
	for i, _ := range chrs {
		chr := chrs[i]

		// Only discover descriptors if this characteristic has a CCCD.
		if chr.SubscribeType() != 0 {
			var endHandle uint16
			if i < len(chrs)-1 {
				endHandle = chrs[i+1].DefHandle - 1
			} else {
				endHandle = svcEndHandle
			}

			dscs, err := c.discAllDscsOnce(chrs[i].ValHandle, endHandle)
			if err != nil {
				return err
			}

			chr.Dscs = dscs
		}
	}

	return nil
}

func (c *Conn) discAllChrsOnce(svc Service) ([]*Characteristic, error) {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = c.connHandle
	r.StartHandle = int(svc.StartHandle)
	r.EndHandle = int(svc.EndHandle)

	bl, err := c.rxvr.AddListener("disc-all-chrs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-chrs", bl)

	rawChrs, err := discAllChrs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	chrs := make([]*Characteristic, len(rawChrs))
	for i, rc := range rawChrs {
		chrs[i] = &Characteristic{
			Uuid:       rc.Uuid,
			DefHandle:  uint16(rc.DefHandle),
			ValHandle:  uint16(rc.ValHandle),
			Properties: BleDiscChrProperties(rc.Properties),
		}
	}

	if err := c.discAllDscs(chrs, svc.EndHandle); err != nil {
		return nil, err
	}

	return chrs, nil
}

func (c *Conn) discAllChrs(svcs []Service) error {
	for i, _ := range svcs {
		chrs, err := c.discAllChrsOnce(svcs[i])
		if err != nil {
			return err
		}

		svcs[i].Chrs = chrs
	}

	return nil
}

func (c *Conn) discAllSvcs() ([]Service, error) {
	r := NewBleDiscAllSvcsReq()
	r.ConnHandle = c.connHandle

	bl, err := c.rxvr.AddListener("disc-all-svcs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-svcs", bl)

	rawSvcs, err := discAllSvcs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	svcs := make([]Service, len(rawSvcs))
	for i, rs := range rawSvcs {
		svcs[i] = Service{
			Uuid:        rs.Uuid,
			StartHandle: uint16(rs.StartHandle),
			EndHandle:   uint16(rs.EndHandle),
		}
	}

	return svcs, nil
}

func (c *Conn) connCancel() error {
	r := NewBleConnCancelReq()

	bl, err := c.rxvr.AddListener("conn-cancel", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("conn-cancel", bl)

	if err := connCancel(c.bx, bl, r); err != nil {
		// Ignore ealready errors.
		bhdErr := nmxutil.ToBleHost(err)
		if bhdErr == nil || bhdErr.Status != ERR_CODE_EALREADY {
			return err
		}
	}

	return nil
}

// Indicates whether an error reported during MTU exchange is a "real" error or
// not.  If the error was reported because MTU exchange already took place,
// that isn't considered a real error.
func isExchangeMtuError(err error) bool {
	if err == nil {
		return false
	}

	bhe := nmxutil.ToBleHost(err)
	if bhe == nil {
		return true
	}

	switch bhe.Status {
	case ERR_CODE_EALREADY:
		return false
	case ERR_CODE_ATT_BASE + ERR_CODE_ATT_REQ_NOT_SUPPORTED:
		return false
	default:
		return true
	}
}

func (c *Conn) terminate() error {
	r := NewBleTerminateReq()
	r.ConnHandle = c.connHandle
	r.HciReason = ERR_CODE_HCI_REM_USER_CONN_TERM

	bl, err := c.rxvr.AddListener("terminate", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("terminate", bl)

	if err := terminate(c.bx, bl, r); err != nil {
		// Ignore ealready errors.
		bhdErr := nmxutil.ToBleHost(err)
		if bhdErr == nil || bhdErr.Status != ERR_CODE_EALREADY {
			return err
		}
	}

	return nil
}
