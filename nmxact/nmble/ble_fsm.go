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

type BleFsmListener struct {
	bl        *BleListener
	abortChan chan struct{}
}

type BleRxDataFn func(data []byte)
type BleDisconnectFn func(dt BleFsmDisconnectType, peer BleDev, err error)

type BleFsmParams struct {
	Bx           *BleXport
	OwnAddrType  BleAddrType
	PeerDev      BleDev
	ConnTries    int
	SvcUuids     []BleUuid
	ReqChrUuid   BleUuid
	RspChrUuid   BleUuid
	RxDataCb     BleRxDataFn
	DisconnectCb BleDisconnectFn
	EncryptWhen  BleEncryptWhen
}

type BleFsm struct {
	params BleFsmParams

	connHandle uint16
	nmpSvc     *BleSvc
	nmpReqChr  *BleChr
	nmpRspChr  *BleChr
	attMtu     int
	connChan   chan error
	encChan    chan error
	bls        map[*BleListener]struct{}
	state      BleSesnState
	errFunnel  nmxutil.ErrFunnel
	id         uint32
	wg         sync.WaitGroup

	// Protects all accesses to the FSM state variable.
	stateMtx sync.Mutex

	// Protects all accesses to the bls map.
	blsMtx sync.Mutex
}

func NewBleFsm(p BleFsmParams) *BleFsm {
	bf := &BleFsm{
		params: p,

		bls:    map[*BleListener]struct{}{},
		attMtu: DFLT_ATT_MTU,
		id:     getNextId(),
	}

	bf.errFunnel.AccumDelay = time.Second
	bf.errFunnel.LessCb = fsmErrorLess
	bf.errFunnel.ProcCb = func(err error) { bf.processErr(err) }

	return bf
}

func (bf *BleFsm) disconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) peer=%s handle=%d",
		ErrCodeToString(reason), reason, bf.params.PeerDev.String(),
		bf.connHandle)
	return nmxutil.NewBleSesnDisconnectError(reason, str)
}

func (bf *BleFsm) closedError(msg string) error {
	return nmxutil.NewSesnClosedError(
		fmt.Sprintf("%s; state=%d", msg, bf.getState()))
}

func (bf *BleFsm) getState() BleSesnState {
	bf.stateMtx.Lock()
	defer bf.stateMtx.Unlock()

	return bf.state
}

func (bf *BleFsm) setState(toState BleSesnState) {
	bf.stateMtx.Lock()
	defer bf.stateMtx.Unlock()

	bf.state = toState
}

func (bf *BleFsm) addBleListener(name string, base BleMsgBase) (
	*BleListener, error) {

	nmxutil.LogAddBleListener(2, base, bf.id, name)

	bl := NewBleListener()

	bf.blsMtx.Lock()
	defer bf.blsMtx.Unlock()

	if err := bf.params.Bx.Bd.AddListener(base, bl); err != nil {
		return nil, err
	}

	bf.wg.Add(1)
	bf.bls[bl] = struct{}{}
	return bl, nil
}

func (bf *BleFsm) addBleBaseListener(name string, base BleMsgBase) (
	*BleListener, error) {

	return bf.addBleListener(name, base)
}

func (bf *BleFsm) addBleSeqListener(name string, seq BleSeq) (
	*BleListener, error) {

	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}
	return bf.addBleListener(name, base)
}

func (bf *BleFsm) removeBleListener(name string, base BleMsgBase) {
	nmxutil.LogAddBleListener(2, base, bf.id, name)

	bf.blsMtx.Lock()
	defer bf.blsMtx.Unlock()

	bl := bf.params.Bx.Bd.RemoveListener(base)
	delete(bf.bls, bl)

	bf.wg.Done()
}

func (bf *BleFsm) removeBleBaseListener(name string, base BleMsgBase) {
	bf.removeBleListener(name, base)
}

func (bf *BleFsm) removeBleSeqListener(name string, seq BleSeq) {
	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}

	bf.removeBleListener(name, base)
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
	aIsDisc := nmxutil.IsBleSesnDisconnect(a)
	bIsDisc := nmxutil.IsBleSesnDisconnect(b)

	if aIsXport {
		return false
	}

	if aIsDisc {
		return !bIsXport && !bIsDisc
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

func (bf *BleFsm) errorAll(err error) {
	bf.blsMtx.Lock()
	defer bf.blsMtx.Unlock()

	bls := bf.bls
	bf.bls = map[*BleListener]struct{}{}

	for bl, _ := range bls {
		bl.ErrChan <- err
	}
}

func (bf *BleFsm) processErr(err error) {
	// Remember some fields before we clear them.
	dt := calcDisconnectType(bf.getState())

	bf.params.Bx.StopWaitingForMaster(bf, err)
	bf.errorAll(err)

	bf.setState(SESN_STATE_UNCONNECTED)

	// Wait for all listeners to get removed.
	bf.wg.Wait()

	bf.errFunnel.Reset()
	bf.params.DisconnectCb(dt, bf.params.PeerDev, err)
}

func (bf *BleFsm) connectListen(seq BleSeq) error {
	bf.connChan = make(chan error, 1)

	bl, err := bf.addBleSeqListener("connect", seq)
	if err != nil {
		return err
	}

	go func() {
		defer bf.removeBleSeqListener("connect", seq)
		for {
			select {
			case err := <-bl.ErrChan:
				bf.connChan <- err
				bf.errFunnel.Insert(err)
				return

			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleConnectRsp:
					bl.Acked = true
					if msg.Status != 0 {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%s (%d) peer=%s",
							ErrCodeToString(msg.Status), msg.Status,
							bf.params.PeerDev.String())
						log.Debugf(str)
						err := nmxutil.NewBleHostError(msg.Status, str)
						bf.connChan <- err
						bf.errFunnel.Insert(err)
						return
					} else {
						bf.connChan <- nil
					}

				case *BleConnectEvt:
					if msg.Status == 0 {
						bl.Acked = true
						bf.connHandle = msg.ConnHandle
						bf.logConnection()
						if err := bf.nmpRspListen(); err != nil {
							bf.connChan <- err
							bf.errFunnel.Insert(err)
							return
						}
						bf.connChan <- nil
					} else {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%s (%d) peer=%s",
							ErrCodeToString(msg.Status), msg.Status,
							bf.params.PeerDev.String())
						log.Debugf(str)
						err := nmxutil.NewBleHostError(msg.Status, str)
						bf.connChan <- err
						bf.errFunnel.Insert(err)
						return
					}

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

			case <-bl.AfterTimeout(bf.params.Bx.RspTimeout()):
				err := BhdTimeoutError(MSG_TYPE_CONNECT, seq)
				bf.connChan <- err
				bf.errFunnel.Insert(err)
			}
		}
	}()
	return nil
}

func (bf *BleFsm) nmpRspListen() error {
	base := BleMsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_NOTIFY_RX_EVT,
		Seq:        BLE_SEQ_NONE,
		ConnHandle: int(bf.connHandle),
	}

	bl, err := bf.addBleBaseListener("nmp-rsp", base)
	if err != nil {
		return err
	}

	go func() {
		defer bf.removeBleBaseListener("nmp-rsp", base)
		for {
			select {
			case <-bl.ErrChan:
				if err != nil {
					bf.errFunnel.Insert(err)
				}
				return
			case bm := <-bl.BleChan:
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
	r := NewBleConnectReq()
	r.OwnAddrType = bf.params.OwnAddrType
	r.PeerAddrType = bf.params.PeerDev.AddrType
	r.PeerAddr = bf.params.PeerDev.Addr

	// Initiating a connection requires dedicated master privileges.
	if err := bf.params.Bx.AcquireMaster(bf); err != nil {
		return err
	}
	defer bf.params.Bx.ReleaseMaster()

	if err := bf.connectListen(r.Seq); err != nil {
		return err
	}

	// Tell blehostd to initiate connection.
	if err := connect(bf.params.Bx, bf.connChan, r); err != nil {
		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EDONE {
			// Already connected.
			return nil
		}
		return err
	}

	// Connection operation now in progress.
	bf.setState(SESN_STATE_CONNECTING)

	err := <-bf.connChan
	if !nmxutil.IsXport(err) {
		// The transport did not restart; always attempt to cancel the connect
		// operation.  In most cases, the host has already stopped connecting
		// and will respond with an "ealready" error that can be ignored.
		if err := bf.connCancel(); err != nil {
			log.Errorf("Failed to cancel connect in progress: %s",
				err.Error())
		}
	}

	return err
}

func (bf *BleFsm) terminateSetState() error {
	switch bf.getState() {
	case SESN_STATE_UNCONNECTED,
		SESN_STATE_CONNECTING,
		SESN_STATE_CONN_CANCELLING:
		return fmt.Errorf("BLE terminate failed; not connected")
	case SESN_STATE_TERMINATING:
		return fmt.Errorf(
			"BLE terminate failed; session already being closed")
	default:
		bf.setState(SESN_STATE_TERMINATING)
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

	bl, err := bf.addBleSeqListener("terminate", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("terminate", r.Seq)

	if err := terminate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) connCancel() error {
	r := NewBleConnCancelReq()

	bl, err := bf.addBleSeqListener("conn-cancel", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("conn-cancel", r.Seq)

	if err := connCancel(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discSvcUuidOnce(uuid BleUuid) (*BleSvc, error) {
	r := NewBleDiscSvcUuidReq()
	r.ConnHandle = bf.connHandle
	r.Uuid = uuid

	bl, err := bf.addBleSeqListener("disc-svc-uuid", r.Seq)
	if err != nil {
		return nil, err
	}
	defer bf.removeBleSeqListener("disc-svc-uuid", r.Seq)

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

	bl, err := bf.addBleSeqListener("enc-initiate", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("enc-initiate", r.Seq)

	bf.encChan = make(chan error, 1)
	defer func() { bf.encChan = nil }()

	if err := encInitiate(bf.params.Bx, bl, bf.encChan, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discAllChrs() error {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = bf.connHandle
	r.StartHandle = bf.nmpSvc.StartHandle
	r.EndHandle = bf.nmpSvc.EndHandle

	bl, err := bf.addBleSeqListener("disc-all-chrs", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("disc-all-chrs", r.Seq)

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

	bl, err := bf.addBleSeqListener("exchange-mtu", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("exchange-mtu", r.Seq)

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

	bl, err := bf.addBleSeqListener("write-cmd", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("write-cmd", r.Seq)

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

	bl, err := bf.addBleSeqListener("subscribe", r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener("subscribe", r.Seq)

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
		return bf.params.PeerDev.AddrType == BLE_ADDR_TYPE_RPA_PUB ||
			bf.params.PeerDev.AddrType == BLE_ADDR_TYPE_RPA_RND

	default:
		panic(fmt.Sprintf("Invalid BleEncryptWhen value: %d",
			bf.params.EncryptWhen))
	}
}

func (bf *BleFsm) executeState() (bool, error) {
	switch bf.getState() {
	case SESN_STATE_UNCONNECTED:
		if err := bf.connect(); err != nil {
			return false, err
		}
		bf.setState(SESN_STATE_EXCHANGE_MTU)

	case SESN_STATE_EXCHANGE_MTU:
		if err := bf.exchangeMtu(); err != nil {
			bhe := nmxutil.ToBleHost(err)
			retry := bhe != nil && bhe.Status == ERR_CODE_ENOTCONN
			return retry, err
		}
		bf.setState(SESN_STATE_DISCOVER_SVC)

	case SESN_STATE_DISCOVER_SVC:
		if err := bf.discSvcUuid(); err != nil {
			return false, err
		}
		bf.setState(SESN_STATE_DISCOVER_CHR)

	case SESN_STATE_DISCOVER_CHR:
		if err := bf.discAllChrs(); err != nil {
			return false, err
		}
		if bf.shouldEncrypt() {
			bf.setState(SESN_STATE_SECURITY)
		} else {
			bf.setState(SESN_STATE_SUBSCRIBE)
		}

	case SESN_STATE_SECURITY:
		if err := bf.encInitiate(); err != nil {
			return false, err
		}
		bf.setState(SESN_STATE_SUBSCRIBE)

	case SESN_STATE_SUBSCRIBE:
		if err := bf.subscribe(); err != nil {
			return false, err
		}
		bf.setState(SESN_STATE_DONE)

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
			bf.getState()))
	}

	bf.errFunnel.Start()

	for {
		retry, err := bf.executeState()
		if err != nil {
			bf.errFunnel.Insert(err)
			err = bf.errFunnel.Wait()
			return retry, err
		} else if bf.getState() == SESN_STATE_DONE {
			return false, nil
		}
	}
}

// @return bool                 Whether another start attempt should be made;
//         error                The error that caused the start attempt to
//                                  fail; nil on success.
func (bf *BleFsm) Start() error {
	var err error

	for i := 0; i < bf.params.ConnTries; i++ {
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
	state := bf.getState()

	switch state {
	case SESN_STATE_UNCONNECTED,
		SESN_STATE_TERMINATING,
		SESN_STATE_CONN_CANCELLING:

		return false,
			bf.closedError("Attempt to close an unopened BLE session")

	case SESN_STATE_CONNECTING:
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
	return bf.getState() == SESN_STATE_DONE
}

func (bf *BleFsm) IsClosed() bool {
	return bf.getState() == SESN_STATE_UNCONNECTED
}

func (bf *BleFsm) TxNmp(payload []byte, nl *nmp.NmpListener,
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
				payload[7], payload[6], bf.params.PeerDev)

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bf *BleFsm) TxOic(payload []byte, ol *oic.OicListener,
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
