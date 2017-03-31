package nmble

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

const DFLT_ATT_MTU = 23

type BleSesnState int32

const (
	SESN_STATE_UNCONNECTED     BleSesnState = 0
	SESN_STATE_SCANNING                     = 1
	SESN_STATE_CONNECTING                   = 2
	SESN_STATE_CONNECTED                    = 3
	SESN_STATE_EXCHANGING_MTU               = 4
	SESN_STATE_EXCHANGED_MTU                = 5
	SESN_STATE_DISCOVERING_SVC              = 6
	SESN_STATE_DISCOVERED_SVC               = 7
	SESN_STATE_DISCOVERING_CHR              = 8
	SESN_STATE_DISCOVERED_CHR               = 9
	SESN_STATE_TERMINATING                  = 10
	SESN_STATE_CONN_CANCELLING              = 11
)

type BleFsmDisconnectType int

const (
	FSM_DISCONNECT_TYPE_UNOPENED BleFsmDisconnectType = iota
	FSM_DISCONNECT_TYPE_IMMEDIATE_TIMEOUT
	FSM_DISCONNECT_TYPE_OPENED
	FSM_DISCONNECT_TYPE_REQUESTED
)

type BleRxNmpFn func(data []byte)
type BleDisconnectFn func(dt BleFsmDisconnectType, peer BleDev, err error)

type BleFsmParams struct {
	Bx           *BleXport
	OwnAddrType  BleAddrType
	PeerSpec     sesn.BlePeerSpec
	SvcUuid      BleUuid
	ReqChrUuid   BleUuid
	RspChrUuid   BleUuid
	RxNmpCb      BleRxNmpFn
	DisconnectCb BleDisconnectFn
}

type BleFsm struct {
	params BleFsmParams

	peerDev    *BleDev
	connHandle int
	nmpSvc     *BleSvc
	nmpReqChr  *BleChr
	nmpRspChr  *BleChr
	attMtu     int
	connChan   chan error

	mtx             sync.Mutex
	lastStateChange time.Time

	// These variables must be protected by the mutex.
	bls   map[*BleListener]struct{}
	state BleSesnState
}

func NewBleFsm(p BleFsmParams) *BleFsm {
	bf := &BleFsm{
		params: p,

		bls:    map[*BleListener]struct{}{},
		attMtu: DFLT_ATT_MTU,
	}

	return bf
}

func (bf *BleFsm) disconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) peer=%s handle=%d",
		ErrCodeToString(reason), reason, bf.peerDev.String(), bf.connHandle)
	return nmxutil.NewBleSesnDisconnectError(reason, str)
}

func (bf *BleFsm) closedError(msg string) error {
	return nmxutil.NewSesnClosedError(fmt.Sprintf(
		"%s; state=%d last-state-change=%s",
		msg, bf.getState(), bf.lastStateChange))
}

func (bf *BleFsm) getState() BleSesnState {
	bf.mtx.Lock()
	defer bf.mtx.Unlock()

	return bf.state
}

func (bf *BleFsm) setState(toState BleSesnState) {
	bf.mtx.Lock()
	defer bf.mtx.Unlock()

	bf.state = toState
	bf.lastStateChange = time.Now()
}

func (bf *BleFsm) transitionState(fromState BleSesnState,
	toState BleSesnState) error {

	bf.mtx.Lock()
	defer bf.mtx.Unlock()

	if bf.state != fromState {
		return fmt.Errorf(
			"Can't set BleFsm state to %d; current state != required "+
				"value: %d",
			toState, fromState)
	}

	bf.state = toState
	return nil
}

func (bf *BleFsm) addBleListener(base BleMsgBase) (*BleListener, error) {
	bl := NewBleListener()

	bf.mtx.Lock()
	bf.bls[bl] = struct{}{}
	bf.mtx.Unlock()

	if err := bf.params.Bx.Bd.AddListener(base, bl); err != nil {
		delete(bf.bls, bl)
		return nil, err
	}

	return bl, nil
}

func (bf *BleFsm) addBleSeqListener(seq int) (*BleListener, error) {
	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}
	bl, err := bf.addBleListener(base)
	if err != nil {
		return nil, err
	}

	return bl, nil
}

func (bf *BleFsm) removeBleListener(base BleMsgBase) {
	bl := bf.params.Bx.Bd.RemoveListener(base)
	if bl != nil {
		bf.mtx.Lock()
		delete(bf.bls, bl)
		bf.mtx.Unlock()
	}
}

func (bf *BleFsm) removeBleSeqListener(seq int) {
	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        seq,
		ConnHandle: -1,
	}

	bf.removeBleListener(base)
}

func (bf *BleFsm) action(
	preState BleSesnState,
	inState BleSesnState,
	postState BleSesnState,
	cb func() error) error {

	if err := bf.transitionState(preState, inState); err != nil {
		return err
	}

	if err := cb(); err != nil {
		bf.setState(preState)
		return err
	}

	bf.setState(postState)
	return nil
}

func (bf *BleFsm) calcDisconnectType() BleFsmDisconnectType {
	switch bf.getState() {
	case SESN_STATE_EXCHANGING_MTU:
		return FSM_DISCONNECT_TYPE_IMMEDIATE_TIMEOUT

	case SESN_STATE_DISCOVERED_CHR:
		return FSM_DISCONNECT_TYPE_OPENED

	case SESN_STATE_TERMINATING, SESN_STATE_CONN_CANCELLING:
		return FSM_DISCONNECT_TYPE_REQUESTED

	default:
		return FSM_DISCONNECT_TYPE_UNOPENED
	}
}

func (bf *BleFsm) onDisconnect(err error) {
	log.Debugf(err.Error())

	bf.mtx.Lock()
	bls := make([]*BleListener, 0, len(bf.bls))
	for bl, _ := range bf.bls {
		bls = append(bls, bl)
	}
	bf.mtx.Unlock()

	// Remember some fields before we clear them.
	dt := bf.calcDisconnectType()
	peer := *bf.peerDev

	bf.setState(SESN_STATE_UNCONNECTED)
	bf.peerDev = nil

	for _, bl := range bls {
		bl.ErrChan <- err
	}

	bf.params.DisconnectCb(dt, peer, err)
}

func (bf *BleFsm) connectListen(seq int) error {
	bf.connChan = make(chan error, 1)

	bl, err := bf.addBleSeqListener(seq)
	if err != nil {
		return err
	}

	go func() {
		defer bf.removeBleSeqListener(seq)
		for {
			select {
			case err := <-bl.ErrChan:
				// Transport reported error.  Assume all connections have
				// dropped.
				bf.onDisconnect(err)
				return

			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleConnectRsp:
					bl.Acked = true
					if msg.Status != 0 {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%s (%d) peer=%s",
							ErrCodeToString(msg.Status), msg.Status,
							bf.peerDev.String())
						log.Debugf(str)
						bf.connChan <- nmxutil.NewBleHostError(msg.Status, str)
						return
					}

				case *BleConnectEvt:
					if msg.Status == 0 {
						bl.Acked = true
						log.Debugf("BLE connection attempt succeeded; "+
							"peer=%d handle=%d", bf.peerDev.String(),
							msg.ConnHandle)
						bf.connHandle = msg.ConnHandle
						if err := bf.nmpRspListen(); err != nil {
							bf.connChan <- err
							return
						}
						bf.connChan <- nil
					} else {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%s (%d) peer=%s",
							ErrCodeToString(msg.Status), msg.Status,
							bf.peerDev.String())
						log.Debugf(str)
						bf.connChan <- nmxutil.NewBleHostError(msg.Status, str)
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
						bf.attMtu = msg.Mtu
					}

				case *BleDisconnectEvt:
					err := bf.disconnectError(msg.Reason)
					bf.onDisconnect(err)
					return

				default:
				}

			case <-bl.AfterTimeout(bf.params.Bx.RspTimeout()):
				bf.connChan <- BhdTimeoutError(MSG_TYPE_CONNECT)
			}
		}
	}()
	return nil
}

func (bf *BleFsm) nmpRspListen() error {
	base := BleMsgBase{
		Op:         MSG_OP_EVT,
		Type:       MSG_TYPE_NOTIFY_RX_EVT,
		Seq:        -1,
		ConnHandle: bf.connHandle,
	}

	bl, err := bf.addBleListener(base)
	if err != nil {
		return err
	}

	go func() {
		defer bf.removeBleListener(base)
		for {
			select {
			case <-bl.ErrChan:
				// The session encountered an error; stop listening.
				return
			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleNotifyRxEvt:
					if bf.nmpRspChr != nil &&
						msg.AttrHandle == bf.nmpRspChr.ValHandle {

						bf.params.RxNmpCb(msg.Data.Bytes)
					}

				default:
				}
			}
		}
	}()
	return nil
}

func (bf *BleFsm) connect() error {
	r := NewBleConnectReq()
	r.OwnAddrType = bf.params.OwnAddrType
	r.PeerAddrType = bf.peerDev.AddrType
	r.PeerAddr = bf.peerDev.Addr

	if err := bf.connectListen(r.Seq); err != nil {
		return err
	}

	if err := connect(bf.params.Bx, bf.connChan, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) scan() error {
	r := NewBleScanReq()
	r.OwnAddrType = bf.params.OwnAddrType
	r.DurationMs = 15000
	r.FilterPolicy = BLE_SCAN_FILT_NO_WL
	r.Limited = false
	r.Passive = false
	r.FilterDuplicates = true

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	abortChan := make(chan struct{}, 1)

	// This function gets called for each incoming advertisement.
	scanCb := func(evt *BleScanEvt) {
		r := BleAdvReport{
			EventType: evt.EventType,
			Sender: BleDev{
				AddrType: evt.AddrType,
				Addr:     evt.Addr,
			},
			Rssi: evt.Rssi,
			Data: evt.Data.Bytes,

			Flags:          evt.DataFlags,
			Name:           evt.DataName,
			NameIsComplete: evt.DataNameIsComplete,
		}

		// Ask client if we should connect to this advertiser.
		if bf.params.PeerSpec.ScanPred(r) {
			bf.peerDev = &r.Sender
			abortChan <- struct{}{}
		}
	}

	if err := scan(bf.params.Bx, bl, r, abortChan, scanCb); err != nil {
		return err
	}

	// Scanning still in progress; cancel the operation.
	return bf.scanCancel()
}

func (bf *BleFsm) scanCancel() error {
	r := NewBleScanCancelReq()

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	if err := scanCancel(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) terminateSetState() error {
	bf.mtx.Lock()
	defer bf.mtx.Unlock()

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

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	if err := terminate(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) connCancel() error {
	if err := bf.transitionState(
		SESN_STATE_CONNECTING,
		SESN_STATE_CONN_CANCELLING); err != nil {

		return fmt.Errorf("BLE connect cancel failed; not connecting")
	}

	r := NewBleConnCancelReq()
	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	if err := connCancel(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discSvcUuid() error {
	r := NewBleDiscSvcUuidReq()
	r.ConnHandle = bf.connHandle
	r.Uuid = bf.params.SvcUuid

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	bf.nmpSvc, err = discSvcUuid(bf.params.Bx, bl, r)
	if err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discAllChrs() error {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = bf.connHandle
	r.StartHandle = bf.nmpSvc.StartHandle
	r.EndHandle = bf.nmpSvc.EndHandle

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

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

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

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

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

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

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	if err := writeCmd(bf.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

// Tries to populate the FSM's peerDev field.  This function succeeds if the
// client specified the address of the peer to connect to.
func (bf *BleFsm) tryFillPeerDev() bool {
	// The peer spec contains one of:
	//     * Peer address;
	//     * Predicate function to call during scanning.
	// If a peer address is specified, fill in the peer field now so the
	// scanning step can be skipped.  Otherwise, the peer field gets populated
	// during scanning.
	if bf.params.PeerSpec.ScanPred == nil {
		bf.peerDev = &bf.params.PeerSpec.Dev
		return true
	}

	return false
}

// @return bool                 Whether another start attempt should be made;
//         error                The error that caused the start attempt to
//                                  fail; nil on success.
func (bf *BleFsm) Start() (bool, error) {
	if bf.getState() != SESN_STATE_UNCONNECTED {
		return false, nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open BLE session")
	}

	for {
		state := bf.getState()
		switch state {
		case SESN_STATE_UNCONNECTED:
			var err error

			// Determine if we can immediately initiate a connection, or if we
			// need to scan for a peer first.  If the client specified a peer
			// address, or if we have already successfully scanned, we initiate
			// a connection now.  Otherwise, we need to scan to determine which
			// peer meets the specified scan criteria.
			bf.tryFillPeerDev()
			if bf.peerDev == nil {
				// Peer not inferred yet.  Initiate scan.
				cb := func() error { return bf.scan() }
				err = bf.action(
					SESN_STATE_UNCONNECTED,
					SESN_STATE_SCANNING,
					SESN_STATE_UNCONNECTED,
					cb)
			} else {
				// We already know the address we want to connect to.  Initiate
				// a connection.
				cb := func() error { return bf.connect() }
				err = bf.action(
					SESN_STATE_UNCONNECTED,
					SESN_STATE_CONNECTING,
					SESN_STATE_CONNECTED,
					cb)
			}

			if err != nil {
				return false, err
			}

		case SESN_STATE_CONNECTED:
			cb := func() error { return bf.exchangeMtu() }
			err := bf.action(
				SESN_STATE_CONNECTED,
				SESN_STATE_EXCHANGING_MTU,
				SESN_STATE_EXCHANGED_MTU,
				cb)
			if err != nil {
				bhe := nmxutil.ToBleHost(err)
				retry := bhe != nil && bhe.Status == ERR_CODE_ENOTCONN
				return retry, err
			}

		case SESN_STATE_EXCHANGED_MTU:
			cb := func() error { return bf.discSvcUuid() }
			err := bf.action(
				SESN_STATE_EXCHANGED_MTU,
				SESN_STATE_DISCOVERING_SVC,
				SESN_STATE_DISCOVERED_SVC,
				cb)
			if err != nil {
				return false, err
			}

		case SESN_STATE_DISCOVERED_SVC:
			cb := func() error {
				return bf.discAllChrs()
			}

			err := bf.action(
				SESN_STATE_DISCOVERED_SVC,
				SESN_STATE_DISCOVERING_CHR,
				SESN_STATE_DISCOVERED_CHR,
				cb)
			if err != nil {
				return false, err
			}

			if err := bf.subscribe(); err != nil {
				return false, err
			}

		case SESN_STATE_DISCOVERED_CHR:
			/* Open complete. */
			return false, nil

		case SESN_STATE_CONNECTING,
			SESN_STATE_DISCOVERING_SVC,
			SESN_STATE_DISCOVERING_CHR,
			SESN_STATE_TERMINATING:
			return false, fmt.Errorf("BleFsm already being opened")
		}
	}
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
		if err := bf.connCancel(); err != nil {
			return false, err
		}
		return true, nil

	default:
		if err := bf.terminate(); err != nil {
			return false, err
		}
		return false, nil
	}
}

func (bf *BleFsm) IsOpen() bool {
	return bf.getState() == SESN_STATE_DISCOVERED_CHR
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
			return nil, nmxutil.NewNmpTimeoutError("NMP timeout")
		}
	}
}
