package nmble

import (
	"encoding/hex"
	"fmt"
	"strings"
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
	SESN_STATE_DONE                         = 7
	SESN_STATE_TERMINATING                  = 8
	SESN_STATE_CONN_CANCELLING              = 9
)

type BleFsmDisconnectType int

const (
	FSM_DISCONNECT_TYPE_UNOPENED BleFsmDisconnectType = iota
	FSM_DISCONNECT_TYPE_IMMEDIATE_TIMEOUT
	FSM_DISCONNECT_TYPE_OPENED
	FSM_DISCONNECT_TYPE_REQUESTED
)

type BleRxDataFn func(data []byte)
type BleDisconnectFn func(dt BleFsmDisconnectType, peer BleDev, err error)

type BleFsmParamsCentral struct {
	PeerDev   BleDev
	ConnTries int
}

type BleFsmParams struct {
	Bx           *BleXport
	OwnAddrType  BleAddrType
	EncryptWhen  BleEncryptWhen
	Central      BleFsmParamsCentral
	SvcUuids     []BleUuid
	ReqChrUuid   BleUuid
	RspChrUuid   BleUuid
	RxDataCb     BleRxDataFn
	DisconnectCb BleDisconnectFn
}

type BleFsm struct {
	params BleFsmParams

	connHandle uint16
	peerDev    BleDev
	nmpSvc     *BleSvc
	nmpReqChr  *BleChr
	nmpRspChr  *BleChr
	attMtu     int
	state      BleSesnState
	rxer       *Receiver
	errFunnel  nmxutil.ErrFunnel
	id         uint32

	// Conveys changes in encrypted state.
	encChan chan error
}

func NewBleFsm(p BleFsmParams) *BleFsm {
	bf := &BleFsm{
		params: p,

		attMtu: DFLT_ATT_MTU,
		id:     getNextId(),
	}

	bf.rxer = NewReceiver(bf.id, p.Bx, 1)

	bf.errFunnel.AccumDelay = time.Second
	bf.errFunnel.LessCb = fsmErrorLess
	bf.errFunnel.ProcCb = func(err error) { bf.processErr(err) }

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

func (bf *BleFsm) logConnection() {
	desc, err := bf.connInfo()
	if err != nil {
		return
	}

	log.Debugf("BLE connection attempt succeeded; %s", desc.String())
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

func (bf *BleFsm) processErr(err error) {
	// Remember some fields before we clear them.
	dt := calcDisconnectType(bf.state)

	bf.params.Bx.StopWaitingForMaster(bf, err)
	bf.rxer.ErrorAll(err)

	bf.state = SESN_STATE_UNCONNECTED

	// Wait for all listeners to get removed.
	bf.rxer.WaitUntilNoListeners()

	bf.errFunnel.Reset()
	bf.params.DisconnectCb(dt, bf.peerDev, err)
}

// Listens for events in the background.
func (bf *BleFsm) eventListen(bl *Listener, seq BleSeq) error {
	go func() {
		defer bf.rxer.RemoveSeqListener("connect", seq)
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
					if bf.encChan != nil {
						bf.encChan <- err
					}

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
	base := MsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_NOTIFY_RX_EVT,
		Seq:        BLE_SEQ_NONE,
		ConnHandle: int(bf.connHandle),
	}

	bl, err := bf.rxer.AddBaseListener("nmp-rsp", base)
	if err != nil {
		return err
	}

	go func() {
		defer bf.rxer.RemoveBaseListener("nmp-rsp", base)
		for {
			select {
			case <-bl.ErrChan:
				if err != nil {
					bf.errFunnel.Insert(err)
				}
				return
			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleNotifyRxEvt:
					if bf.nmpRspChr != nil &&
						msg.AttrHandle == bf.nmpRspChr.ValHandle {

						bf.params.RxDataCb(msg.Data.Bytes)
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

	bl, err := bf.rxer.AddSeqListener("connect", r.Seq)
	if err != nil {
		return err
	}

	// Connection operation now in progress.
	bf.state = SESN_STATE_CONNECTING

	// Tell blehostd to initiate connection.
	if bf.connHandle, err = connect(bf.params.Bx, bl, r); err != nil {
		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EDONE {
			// Already connected.
			bf.rxer.RemoveSeqListener("connect", r.Seq)
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
		} else {
			bf.rxer.RemoveSeqListener("connect", r.Seq)
			return err
		}
	}

	// Listen for events in the background.
	if err := bf.eventListen(bl, r.Seq); err != nil {
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

	bl, err := bf.rxer.AddSeqListener("terminate", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("terminate", r.Seq)

	if err := terminate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) connCancel() error {
	r := NewBleConnCancelReq()

	bl, err := bf.rxer.AddSeqListener("conn-cancel", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("conn-cancel", r.Seq)

	if err := connCancel(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discSvcUuidOnce(uuid BleUuid) (*BleSvc, error) {
	r := NewBleDiscSvcUuidReq()
	r.ConnHandle = bf.connHandle
	r.Uuid = uuid

	bl, err := bf.rxer.AddSeqListener("disc-svc-uuid", r.Seq)
	if err != nil {
		return nil, err
	}
	defer bf.rxer.RemoveSeqListener("disc-svc-uuid", r.Seq)

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

	bl, err := bf.rxer.AddSeqListener("enc-initiate", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("enc-initiate", r.Seq)

	bf.encChan = make(chan error)
	defer func() { bf.encChan = nil }()

	// Initiate the encryption procedure.
	if err := encInitiate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	// Block until the procedure completes.
	return <-bf.encChan
}

func (bf *BleFsm) discAllChrs() error {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = bf.connHandle
	r.StartHandle = bf.nmpSvc.StartHandle
	r.EndHandle = bf.nmpSvc.EndHandle

	bl, err := bf.rxer.AddSeqListener("disc-all-chrs", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("disc-all-chrs", r.Seq)

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

	bl, err := bf.rxer.AddSeqListener("exchange-mtu", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("exchange-mtu", r.Seq)

	mtu, err := exchangeMtu(bf.params.Bx, bl, r)
	if err != nil {
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

	bl, err := bf.rxer.AddSeqListener("write-cmd", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("write-cmd", r.Seq)

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

	bl, err := bf.rxer.AddSeqListener("subscribe", r.Seq)
	if err != nil {
		return err
	}
	defer bf.rxer.RemoveSeqListener("subscribe", r.Seq)

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
		if err := bf.discSvcUuid(); err != nil {
			return false, err
		}
		bf.state = SESN_STATE_DISCOVER_CHR

	case SESN_STATE_DISCOVER_CHR:
		if err := bf.discAllChrs(); err != nil {
			return false, err
		}
		if bf.shouldEncrypt() {
			bf.state = SESN_STATE_SECURITY
		} else {
			bf.state = SESN_STATE_SUBSCRIBE
		}

	case SESN_STATE_SECURITY:
		if err := bf.encInitiate(); err != nil {
			return false, err
		}
		bf.state = SESN_STATE_SUBSCRIBE

	case SESN_STATE_SUBSCRIBE:
		if err := bf.subscribe(); err != nil {
			return false, err
		}
		bf.state = SESN_STATE_DONE

	case SESN_STATE_DONE:
		/* Open complete. */
		return false, fmt.Errorf("BleFsm already done being opened")

	default:
		return false, fmt.Errorf("BleFsm already being opened")
	}

	return false, nil
}

func (bf *BleFsm) startOnce() (bool, error) {
	if !bf.IsClosed() {
		return false, nmxutil.NewSesnAlreadyOpenError(fmt.Sprintf(
			"Attempt to open an already-open BLE session (state=%d)",
			bf.state))
	}

	bf.errFunnel.Start()

	for {
		retry, err := bf.executeState()
		if err != nil {
			bf.errFunnel.Insert(err)
			err = bf.errFunnel.Wait()
			return retry, err
		} else if bf.state == SESN_STATE_DONE {
			return false, nil
		}
	}
}

// @return bool                 Whether another start attempt should be made;
//         error                The error that caused the start attempt to
//                                  fail; nil on success.
func (bf *BleFsm) Start() error {
	var err error

	for i := 0; i < bf.params.Central.ConnTries; i++ {
		var retry bool
		retry, err = bf.startOnce()
		if !retry {
			break
		}
	}

	return err
}

// @return bool                 true if stop complete;
//                              false if disconnect is now pending.
func (bf *BleFsm) Stop() (bool, error) {
	state := bf.state

	switch state {
	case SESN_STATE_UNCONNECTED,
		SESN_STATE_TERMINATING,
		SESN_STATE_CONN_CANCELLING:

		return false,
			bf.closedError("Attempt to close an unopened BLE session")

	case SESN_STATE_CONNECTING:
		bf.connCancel()
		bf.errFunnel.Insert(fmt.Errorf("Connection attempt cancelled"))
		return false, nil

	default:
		if err := bf.terminate(); err != nil {
			return false, err
		}
		return false, nil
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
