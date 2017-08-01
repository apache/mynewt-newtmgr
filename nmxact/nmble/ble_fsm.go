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
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
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
	MgmtChrs    BleMgmtChrs
	MgmtProto   sesn.MgmtProto
}

type FsmChr struct {
	Uuid       BleUuid
	DefHandle  uint16
	ValHandle  uint16
	Properties uint8
}

type FsmSvc struct {
	Uuid        BleUuid
	StartHandle uint16
	EndHandle   uint16
	Chrs        []FsmChr
}

type BleFsm struct {
	params BleFsmParams

	connHandle     uint16
	connDesc       BleConnDesc
	peerDev        BleDev
	svcs           []FsmSvc
	chrs           map[BleChrId]FsmChr
	prevDisconnect BleDisconnectEntry
	attMtu         int
	state          BleSesnState
	txvr           *Transceiver
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

	bf.chrs = map[BleChrId]FsmChr{}

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

func (bf *BleFsm) chrHandle(chrId *BleChrId, name string) (uint16, error) {
	if chrId == nil {
		return 0, fmt.Errorf("BLE session not configured with "+
			"characteristic \"%s\"", name)
	}

	chr, ok := bf.chrs[*chrId]
	if !ok {
		return 0, fmt.Errorf("BLE peer does not support characteristic "+
			"\"%s\" (svc=%s chr=%s)",
			name, chrId.SvcUuid.String(), chrId.ChrUuid.String())
	}

	return chr.ValHandle, nil
}

func (bf *BleFsm) nmpReqHandle() (uint16, error) {
	return bf.chrHandle(bf.params.MgmtChrs.NmpReqChr, "NMP request")
}

func (bf *BleFsm) nmpRspHandle() (uint16, error) {
	return bf.chrHandle(bf.params.MgmtChrs.NmpRspChr, "NMP response")
}

func (bf *BleFsm) oicResHandle(resType oic.ResType) (uint16, error) {
	switch resType {
	case oic.RES_TYPE_PUBLIC:
		return bf.chrHandle(bf.params.MgmtChrs.ResPublicChr,
			"Public resource")
	case oic.RES_TYPE_GW:
		return bf.chrHandle(bf.params.MgmtChrs.ResGwChr,
			"Gateway resource")
	case oic.RES_TYPE_PRIVATE:
		return bf.chrHandle(bf.params.MgmtChrs.ResPrivateChr,
			"Private resource")

	default:
		return 0, fmt.Errorf("Invalid resource type: %#v", resType)
	}
}

func (bf *BleFsm) shutdown(err error) {
	bf.params.Bx.StopWaitingForMaster(bf, err)

	bf.rxer.RemoveAll("shutdown")
	if bf.txvr != nil {
		bf.txvr.Stop(err)
	}

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
			case err, ok := <-bl.ErrChan:
				if ok {
					bf.errFunnel.Insert(err)
				}
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

func (bf *BleFsm) discAllSvcs() error {
	r := NewBleDiscAllSvcsReq()
	r.ConnHandle = bf.connHandle

	bl, err := bf.rxer.AddListener("disc-all-svcs", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveListener("disc-all-svcs", bl)

	svcs, err := discAllSvcs(bf.params.Bx, bl, r)
	if err != nil {
		return err
	}

	bf.svcs = nil
	for _, s := range svcs {
		bf.svcs = append(bf.svcs, FsmSvc{
			Uuid:        s.Uuid,
			StartHandle: uint16(s.StartHandle),
			EndHandle:   uint16(s.EndHandle),
		})
	}

	return nil
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

func (bf *BleFsm) discAllChrsOnce(svc *FsmSvc) error {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = bf.connHandle
	r.StartHandle = int(svc.StartHandle)
	r.EndHandle = int(svc.EndHandle)

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
		fc := FsmChr{
			Uuid:       c.Uuid,
			DefHandle:  uint16(c.DefHandle),
			ValHandle:  uint16(c.ValHandle),
			Properties: uint8(c.Properties),
		}
		svc.Chrs = append(svc.Chrs, fc)
		bf.chrs[BleChrId{svc.Uuid, c.Uuid}] = fc
	}

	return nil
}

func (bf *BleFsm) discAllChrs() error {
	for _, s := range bf.svcs {
		if err := bf.discAllChrsOnce(&s); err != nil {
			return err
		}
	}

	// Listen for NMP responses in the background.
	attHandle, err := bf.nmpRspHandle()
	if err != nil {
		return err
	}

	txvr, err := NewTransceiver(bf.id, bf.connHandle, bf.params.MgmtProto,
		bf.params.Bx, bf.rxer, 1)
	if err != nil {
		return err
	}
	bf.txvr = txvr

	errChan, err := bf.txvr.NmpRspListen(attHandle)
	if err != nil {
		return err
	}

	bf.wg.Add(1)
	go func() {
		defer bf.wg.Done()

		if err := <-errChan; err != nil {
			bf.errFunnel.Insert(err)
		}
	}()

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
		if bhe != nil &&
			(bhe.Status == ERR_CODE_EALREADY ||
				bhe.Status == ERR_CODE_ATT_BASE+ERR_CODE_ATT_REQ_NOT_SUPPORTED) {

			return nil
		}
		return err
	}

	bf.attMtu = mtu
	return nil
}

func (bf *BleFsm) subscribe() error {
	rspHandle, err := bf.nmpRspHandle()
	if err != nil {
		return err
	}

	return bf.txvr.WriteCmd(rspHandle+1, []byte{1, 0}, "subscribe")
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
		if err := bf.discAllSvcs(); err != nil {
			return false, err
		}
		bf.state = SESN_STATE_DISCOVER_CHR

	case SESN_STATE_DISCOVER_CHR:
		if err := bf.discAllChrs(); err != nil {
			return false, err
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
		if err := bf.subscribe(); err != nil {
			return false, err
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
	bf.txvr = nil

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

	bf.state = SESN_STATE_GET_INFO

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

func (bf *BleFsm) TxNmp(req *nmp.NmpMsg, timeout time.Duration) (
	nmp.NmpRsp, error) {

	if !bf.IsOpen() {
		return nil, bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	attHandle, err := bf.nmpReqHandle()
	if err != nil {
		return nil, err
	}

	return bf.txvr.TxNmp(attHandle, req, timeout)
}

func (bf *BleFsm) TxOic(req coap.Message, resType oic.ResType,
	timeout time.Duration) (coap.Message, error) {

	if !bf.IsOpen() {
		return nil, bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	attHandle, err := bf.oicResHandle(resType)
	if err != nil {
		return nil, err
	}

	return bf.txvr.TxOic(attHandle, req, timeout)
}

func (bf *BleFsm) AbortNmpRx(seq uint8) error {
	return bf.txvr.AbortNmpRx(seq)
}
