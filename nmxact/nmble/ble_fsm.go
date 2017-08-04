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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

var nextId uint32

func getNextId() uint32 {
	return atomic.AddUint32(&nextId, 1) - 1
}

const DFLT_ATT_MTU = 23

type BleSesnState int32

const (
	SESN_STATE_UNCONNECTED     BleSesnState = 0
	SESN_STATE_CONNECTING                   = 1
	SESN_STATE_EXCHANGE_MTU                 = 2
	SESN_STATE_DISCOVER_SVC                 = 3
	SESN_STATE_DISCOVER_CHR                 = 4
	SESN_STATE_SECURITY                     = 5
	SESN_STATE_SUBSCRIBE                    = 6
	SESN_STATE_GET_INFO                     = 7
	SESN_STATE_DONE                         = 8
	SESN_STATE_TERMINATING                  = 9
	SESN_STATE_CONN_CANCELLING              = 10
)

type BleFsmDisconnectType int

const (
	FSM_DISCONNECT_TYPE_UNOPENED BleFsmDisconnectType = iota
	FSM_DISCONNECT_TYPE_IMMEDIATE_TIMEOUT
	FSM_DISCONNECT_TYPE_OPENED
	FSM_DISCONNECT_TYPE_REQUESTED
)

type BleDisconnectEntry struct {
	Dt   BleFsmDisconnectType
	Peer BleDev
	Err  error
}

type BleFsmParamsCentral struct {
	PeerDev     BleDev
	ConnTries   int
	ConnTimeout time.Duration
}

type BleFsmParams struct {
	Bx          *BleXport
	OwnAddrType BleAddrType
	EncryptWhen BleEncryptWhen
	Central     BleFsmParamsCentral
	SvcUuids    []BleUuid
	ReqChrUuid  BleUuid
	RspChrUuid  BleUuid
}

type BleFsm struct {
	params BleFsmParams

	connHandle     uint16
	connDesc       BleConnDesc
	peerDev        BleDev
	nmpSvc         *BleDiscSvc
	nmpReqChr      *BleDiscChr
	nmpRspChr      *BleDiscChr
	prevDisconnect BleDisconnectEntry
	attMtu         int
	state          BleSesnState
	rxer           *Receiver
	errFunnel      nmxutil.ErrFunnel
	id             uint32
	wg             sync.WaitGroup

	encBcast       nmxutil.Bcaster
	disconnectChan chan struct{}
	rxNmpChan      chan []byte
}

func NewBleFsm(p BleFsmParams) *BleFsm {
	bf := &BleFsm{
		params: p,

		attMtu: DFLT_ATT_MTU,
		id:     getNextId(),
	}

	bf.rxer = NewReceiver(bf.id, p.Bx, 1)

	bf.errFunnel.AccumDelay = 250 * time.Millisecond
	bf.errFunnel.LessCb = fsmErrorLess

	return bf
}

func (bf *BleFsm) disconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) peer=%s handle=%d",
		ErrCodeToString(reason), reason, bf.peerDev.String(),
		bf.connHandle)
	return nmxutil.NewBleSesnDisconnectError(reason, str)
}

func (bf *BleFsm) closedError(msg string) error {
	return nmxutil.NewSesnClosedError(
		fmt.Sprintf("%s; state=%d", msg, bf.state))
}

func (bf *BleFsm) connInfo() (BleConnDesc, error) {
	return ConnFindXact(bf.params.Bx, bf.connHandle)
}

func fsmErrorLess(a error, b error) bool {
	aIsXport := nmxutil.IsXport(a)
	bIsXport := nmxutil.IsXport(b)
	aIsTerm := nmxutil.IsBleSesnDisconnect(a)
	bIsTerm := nmxutil.IsBleSesnDisconnect(b)

	if aIsXport {
		return false
	}

	if aIsTerm {
		return !bIsXport && !bIsTerm
	}

	return false
}

func calcDisconnectType(state BleSesnState) BleFsmDisconnectType {
	switch state {
	case SESN_STATE_EXCHANGE_MTU:
		return FSM_DISCONNECT_TYPE_IMMEDIATE_TIMEOUT

	case SESN_STATE_DONE:
		return FSM_DISCONNECT_TYPE_OPENED

	case SESN_STATE_TERMINATING, SESN_STATE_CONN_CANCELLING:
		return FSM_DISCONNECT_TYPE_REQUESTED

	default:
		return FSM_DISCONNECT_TYPE_UNOPENED
	}
}

func (bf *BleFsm) shutdown(err error) {
	bf.params.Bx.StopWaitingForMaster(bf, err)
	bf.rxer.RemoveAll("shutdown")

	bf.state = SESN_STATE_UNCONNECTED

	// Wait for all listeners to get removed.
	bf.rxer.WaitUntilNoListeners()
	bf.wg.Wait()

	close(bf.rxNmpChan)
	close(bf.disconnectChan)
}

// Listens for an error in the state machine.  On error, the session is
// considered disconnected and the error is reported to the client.
func (bf *BleFsm) listenForError() {
	bf.wg.Add(1)

	go func() {
		err := <-bf.errFunnel.Wait()

		dt := calcDisconnectType(bf.state)
		bf.prevDisconnect = BleDisconnectEntry{dt, bf.peerDev, err}

		bf.wg.Done()
		bf.shutdown(err)
	}()
}

// Listens for events in the background.
func (bf *BleFsm) eventListen(bl *Listener) error {
	bf.wg.Add(1)

	go func() {
		defer bf.wg.Done()
		defer bf.rxer.RemoveListener("connect", bl)

		for {
			select {
			case err := <-bl.ErrChan:
				bf.errFunnel.Insert(err)
				return

			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleMtuChangeEvt:
					if msg.Status != 0 {
						err := StatusError(MSG_OP_EVT,
							MSG_TYPE_MTU_CHANGE_EVT,
							msg.Status)
						log.Debugf(err.Error())
					} else {
						log.Debugf("BLE ATT MTU updated; from=%d to=%d",
							bf.attMtu, msg.Mtu)
						bf.attMtu = int(msg.Mtu)
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
					}

					// Notify any listeners of the encryption change event.
					bf.encBcast.SendAndClear(err)

				case *BleDisconnectEvt:
					err := bf.disconnectError(msg.Reason)
					bf.errFunnel.Insert(err)
					return

				default:
				}
			}
		}
	}()
	return nil
}

func (bf *BleFsm) nmpRspListen() error {
	key := TchKey(MSG_TYPE_NOTIFY_RX_EVT, int(bf.connHandle))
	bl, err := bf.rxer.AddListener("nmp-rsp", key)
	if err != nil {
		return err
	}

	bf.wg.Add(1)

	go func() {
		defer bf.wg.Done()
		defer bf.rxer.RemoveListener("nmp-rsp", bl)

		for {
			select {
			case err, ok := <-bl.ErrChan:
				if ok {
					bf.errFunnel.Insert(err)
				}
				return
			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleNotifyRxEvt:
					if bf.nmpRspChr != nil &&
						msg.AttrHandle == bf.nmpRspChr.ValHandle {

						bf.rxNmpChan <- msg.Data.Bytes
					}
				}
			}
		}
	}()
	return nil
}

func (bf *BleFsm) connect() error {
	bf.peerDev = bf.params.Central.PeerDev

	r := NewBleConnectReq()
	r.OwnAddrType = bf.params.OwnAddrType
	r.PeerAddrType = bf.peerDev.AddrType
	r.PeerAddr = bf.peerDev.Addr

	// Initiating a connection requires dedicated master privileges.
	if err := bf.params.Bx.AcquireMaster(bf); err != nil {
		return err
	}
	defer bf.params.Bx.ReleaseMaster()

	bl, err := bf.rxer.AddListener("connect", SeqKey(r.Seq))
	if err != nil {
		return err
	}

	// Connection operation now in progress.
	bf.state = SESN_STATE_CONNECTING

	// Tell blehostd to initiate connection.
	if bf.connHandle, err = connect(bf.params.Bx, bl, r,
		bf.params.Central.ConnTimeout); err != nil {

		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EDONE {
			// Already connected.
			bf.rxer.RemoveListener("connect", bl)
			return nil
		} else if !nmxutil.IsXport(err) {
			// The transport did not restart; always attempt to cancel the
			// connect operation.  In most cases, the host has already stopped
			// connecting and will respond with an "ealready" error that can be
			// ignored.
			if err := bf.connCancel(); err != nil {
				log.Errorf("Failed to cancel connect in progress: %s",
					err.Error())
			}
		}

		bf.rxer.RemoveListener("connect", bl)
		return err
	}

	// Listen for events in the background.
	if err := bf.eventListen(bl); err != nil {
		return err
	}

	// Listen for NMP responses in the background.
	if err := bf.nmpRspListen(); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) terminateSetState() error {
	switch bf.state {
	case SESN_STATE_UNCONNECTED,
		SESN_STATE_CONNECTING,
		SESN_STATE_CONN_CANCELLING:
		return fmt.Errorf("BLE terminate failed; not connected")
	case SESN_STATE_TERMINATING:
		return fmt.Errorf(
			"BLE terminate failed; session already being closed")
	default:
		bf.state = SESN_STATE_TERMINATING
	}

	return nil
}

func (bf *BleFsm) terminate() error {
	if err := bf.terminateSetState(); err != nil {
		return err
	}

	r := NewBleTerminateReq()
	r.ConnHandle = bf.connHandle
	r.HciReason = ERR_CODE_HCI_REM_USER_CONN_TERM

	bl, err := bf.rxer.AddListener("terminate", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("terminate", bl)

	if err := terminate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) connCancel() error {
	r := NewBleConnCancelReq()

	bl, err := bf.rxer.AddListener("conn-cancel", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("conn-cancel", bl)

	if err := connCancel(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discSvcUuidOnce(uuid BleUuid) (*BleDiscSvc, error) {
	r := NewBleDiscSvcUuidReq()
	r.ConnHandle = bf.connHandle
	r.Uuid = uuid

	bl, err := bf.rxer.AddListener("disc-svc-uuid", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer bf.rxer.RemoveListener("disc-svc-uuid", bl)

	svc, err := discSvcUuid(bf.params.Bx, bl, r)
	if err != nil {
		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EDONE {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return svc, nil
}

func (bf *BleFsm) discSvcUuid() error {
	for _, uuid := range bf.params.SvcUuids {
		svc, err := bf.discSvcUuidOnce(uuid)
		if err != nil {
			return err
		}

		if svc != nil {
			bf.nmpSvc = svc
			return nil
		}
	}

	strs := []string{}
	for _, uuid := range bf.params.SvcUuids {
		strs = append(strs, uuid.String())
	}

	return fmt.Errorf("Peer does not support any required services: %s",
		strings.Join(strs, ", "))
}

func (bf *BleFsm) encInitiate() error {
	r := NewBleSecurityInitiateReq()
	r.ConnHandle = bf.connHandle

	bl, err := bf.rxer.AddListener("enc-initiate", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("enc-initiate", bl)

	// Initiate the encryption procedure.
	if err := encInitiate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	// Block until the procedure completes.
	itf := <-bf.encBcast.Listen()
	return itf.(error)
}

func (bf *BleFsm) discAllChrs() error {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = bf.connHandle
	r.StartHandle = bf.nmpSvc.StartHandle
	r.EndHandle = bf.nmpSvc.EndHandle

	bl, err := bf.rxer.AddListener("disc-all-chrs", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("disc-all-chrs", bl)

	chrs, err := discAllChrs(bf.params.Bx, bl, r)
	if err != nil {
		return err
	}

	for _, c := range chrs {
		if CompareUuids(bf.params.ReqChrUuid, c.Uuid) == 0 {
			bf.nmpReqChr = c
		}
		if CompareUuids(bf.params.RspChrUuid, c.Uuid) == 0 {
			bf.nmpRspChr = c
		}
	}

	if bf.nmpReqChr == nil {
		return fmt.Errorf(
			"Peer doesn't support required characteristic: %s",
			bf.params.ReqChrUuid.String())
	}

	if bf.nmpRspChr == nil {
		return fmt.Errorf(
			"Peer doesn't support required characteristic: %s",
			bf.params.RspChrUuid.String())
	}

	return nil
}

func (bf *BleFsm) exchangeMtu() error {
	r := NewBleExchangeMtuReq()
	r.ConnHandle = bf.connHandle

	bl, err := bf.rxer.AddListener("exchange-mtu", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("exchange-mtu", bl)

	mtu, err := exchangeMtu(bf.params.Bx, bl, r)
	if err != nil {
		// If the operation failed because the peer already initiated the
		// exchange, just pretend it was successful.
		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EALREADY {
			return nil
		}
		return err
	}

	bf.attMtu = mtu
	return nil
}

func (bf *BleFsm) writeCmd(data []byte) error {
	r := NewBleWriteCmdReq()
	r.ConnHandle = bf.connHandle
	r.AttrHandle = bf.nmpReqChr.ValHandle
	r.Data.Bytes = data

	bl, err := bf.rxer.AddListener("write-cmd", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("write-cmd", bl)

	if err := writeCmd(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) subscribe() error {
	r := NewBleWriteCmdReq()
	r.ConnHandle = bf.connHandle
	r.AttrHandle = bf.nmpRspChr.ValHandle + 1
	r.Data.Bytes = []byte{1, 0}

	bl, err := bf.rxer.AddListener("subscribe", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("subscribe", bl)

	if err := writeCmd(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) shouldEncrypt() bool {
	switch bf.params.EncryptWhen {
	case BLE_ENCRYPT_NEVER:
		return false

	case BLE_ENCRYPT_ALWAYS:
		return true

	case BLE_ENCRYPT_PRIV_ONLY:
		return bf.peerDev.AddrType == BLE_ADDR_TYPE_RPA_PUB ||
			bf.peerDev.AddrType == BLE_ADDR_TYPE_RPA_RND

	default:
		log.Errorf("Invalid BleEncryptWhen value: %d",
			bf.params.EncryptWhen)
		return false
	}
}

func (bf *BleFsm) executeState() (bool, error) {
	switch bf.state {
	case SESN_STATE_UNCONNECTED:
		if err := bf.connect(); err != nil {
			return false, err
		}
		bf.state = SESN_STATE_EXCHANGE_MTU

	case SESN_STATE_EXCHANGE_MTU:
		if err := bf.exchangeMtu(); err != nil {
			bhe := nmxutil.ToBleHost(err)
			retry := bhe != nil && bhe.Status == ERR_CODE_ENOTCONN
			return retry, err
		}
		bf.state = SESN_STATE_DISCOVER_SVC

	case SESN_STATE_DISCOVER_SVC:
		if len(bf.params.SvcUuids) > 0 {
			if err := bf.discSvcUuid(); err != nil {
				return false, err
			}
		}
		bf.state = SESN_STATE_DISCOVER_CHR

	case SESN_STATE_DISCOVER_CHR:
		if bf.nmpSvc != nil {
			if err := bf.discAllChrs(); err != nil {
				return false, err
			}
		}
		bf.state = SESN_STATE_SECURITY

	case SESN_STATE_SECURITY:
		if bf.shouldEncrypt() {
			if err := bf.encInitiate(); err != nil {
				return false, err
			}
		}
		bf.state = SESN_STATE_SUBSCRIBE

	case SESN_STATE_SUBSCRIBE:
		if bf.nmpRspChr != nil {
			if err := bf.subscribe(); err != nil {
				return false, err
			}
		}
		bf.state = SESN_STATE_GET_INFO

	case SESN_STATE_GET_INFO:
		desc, err := bf.connInfo()
		if err != nil {
			return false, err
		}
		bf.connDesc = desc
		bf.state = SESN_STATE_DONE

	case SESN_STATE_DONE:
		/* Open complete. */
		return false, fmt.Errorf("BleFsm already done being opened")

	default:
		return false, fmt.Errorf("BleFsm already being opened")
	}

	return false, nil
}

func (bf *BleFsm) DisconnectChan() <-chan struct{} {
	return bf.disconnectChan
}

func (bf *BleFsm) RxNmpChan() <-chan []byte {
	return bf.rxNmpChan
}

func (bf *BleFsm) PrevDisconnect() BleDisconnectEntry {
	return bf.prevDisconnect
}

func (bf *BleFsm) IsCentral() bool {
	return bf.connDesc.Role == BLE_ROLE_MASTER
}

func (bf *BleFsm) startOnce() (bool, error) {
	bf.disconnectChan = make(chan struct{})
	bf.rxNmpChan = make(chan []byte)

	bf.listenForError()

	for {
		retry, err := bf.executeState()
		if err != nil {
			// If stop fails, assume the connection wasn't established and
			// force an error.
			if bf.Stop() != nil {
				bf.errFunnel.Insert(err)
			}
			<-bf.disconnectChan
			return retry, err
		} else if bf.state == SESN_STATE_DONE {
			// We are fully connected.  Listen for errors in the background.
			return false, nil
		}
	}
}

// @return bool                 Whether another start attempt should be made;
//         error                The error that caused the start attempt to
//                                  fail; nil on success.
func (bf *BleFsm) Start() error {
	var err error

	if !bf.IsClosed() {
		return nmxutil.NewSesnAlreadyOpenError(fmt.Sprintf(
			"Attempt to open an already-open BLE session (state=%d)",
			bf.state))
	}

	for i := 0; i < bf.params.Central.ConnTries; i++ {
		var retry bool
		retry, err = bf.startOnce()
		if !retry {
			break
		}
	}

	if err != nil {
		nmxutil.Assert(!bf.IsOpen())
		nmxutil.Assert(bf.IsClosed())
		return err
	}

	return nil
}

func (bf *BleFsm) StartConnected(
	connHandle uint16, eventListener *Listener) error {

	bf.connHandle = connHandle
	if err := bf.eventListen(eventListener); err != nil {
		return err
	}

	bf.state = SESN_STATE_EXCHANGE_MTU
	if _, err := bf.startOnce(); err != nil {
		nmxutil.Assert(!bf.IsOpen())
		nmxutil.Assert(bf.IsClosed())
		return err
	}

	return nil
}

// @return bool                 true if stop complete;
//                              false if disconnect is now pending.
func (bf *BleFsm) Stop() error {
	state := bf.state

	switch state {
	case SESN_STATE_UNCONNECTED,
		SESN_STATE_TERMINATING,
		SESN_STATE_CONN_CANCELLING:

		return bf.closedError("Attempt to close an unopened BLE session")

	case SESN_STATE_CONNECTING:
		bf.connCancel()
		bf.errFunnel.Insert(fmt.Errorf("Connection attempt cancelled"))
		return nil

	default:
		if err := bf.terminate(); err != nil {
			return err
		}
		return nil
	}
}

func (bf *BleFsm) IsOpen() bool {
	return bf.state == SESN_STATE_DONE
}

func (bf *BleFsm) IsClosed() bool {
	return bf.state == SESN_STATE_UNCONNECTED
}

func (bf *BleFsm) TxNmp(payload []byte, nl *nmp.Listener,
	timeout time.Duration) (nmp.NmpRsp, error) {

	log.Debugf("Tx NMP request: %s", hex.Dump(payload))
	if err := bf.writeCmd(payload); err != nil {
		return nil, err
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			// Only accept NMP responses if the session is still open.  This is
			// to help prevent race conditions in client code.
			if bf.IsOpen() {
				return rsp, nil
			}
		case <-nl.AfterTimeout(timeout):
			msg := fmt.Sprintf(
				"NMP timeout; op=%d group=%d id=%d seq=%d peer=%#v",
				payload[0], payload[4]+payload[5]<<8,
				payload[7], payload[6], bf.peerDev)

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bf *BleFsm) TxOic(payload []byte, ol *oic.Listener,
	timeout time.Duration) (*coap.Message, error) {

	log.Debugf("Tx OIC request: %s", hex.Dump(payload))
	if err := bf.writeCmd(payload); err != nil {
		return nil, err
	}

	// Don't wait for a response if no listener was provided.
	if ol == nil {
		return nil, nil
	}

	for {
		select {
		case err := <-ol.ErrChan:
			return nil, err
		case m := <-ol.RspChan:
			// Only accept messages if the session is still open.  This is
			// to help prevent race conditions in client code.
			if bf.IsOpen() {
				return m, nil
			}
		case <-ol.AfterTimeout(timeout):
			return nil, nmxutil.NewRspTimeoutError("OIC timeout")
		}
	}
}
