package nmble

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newt/nmxact/bledefs"
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/nmxutil"
)

type BleSesnState int32

const DFLT_ATT_MTU = 23

const (
	SESN_STATE_UNCONNECTED     BleSesnState = 0
	SESN_STATE_CONNECTING                   = 1
	SESN_STATE_CONNECTED                    = 2
	SESN_STATE_EXCHANGING_MTU               = 3
	SESN_STATE_EXCHANGED_MTU                = 4
	SESN_STATE_DISCOVERING_SVC              = 5
	SESN_STATE_DISCOVERED_SVC               = 6
	SESN_STATE_DISCOVERING_CHR              = 7
	SESN_STATE_DISCOVERED_CHR               = 8
	SESN_STATE_TERMINATING                  = 9
	SESN_STATE_CONN_CANCELLING              = 10
)

type BleRxNmpFn func(data []byte)
type BleDisconnectFn func(err error)

type BleFsmParams struct {
	Bx           *BleXport
	OwnAddrType  BleAddrType
	Peer         BleDev
	SvcUuid      BleUuid
	ReqChrUuid   BleUuid
	RspChrUuid   BleUuid
	RxNmpCb      BleRxNmpFn
	DisconnectCb BleDisconnectFn
}

type BleFsm struct {
	bx           *BleXport
	ownAddrType  BleAddrType
	peer         BleDev
	svcUuid      BleUuid
	reqChrUuid   BleUuid
	rspChrUuid   BleUuid
	rxNmpCb      BleRxNmpFn
	disconnectCb BleDisconnectFn

	connHandle int
	nmpSvc     *BleSvc
	nmpReqChr  *BleChr
	nmpRspChr  *BleChr
	attMtu     int
	connChan   chan error

	mtx sync.Mutex

	// These variables must be protected by the mutex.
	bls   map[*BleListener]struct{}
	state BleSesnState
}

func NewBleFsm(p BleFsmParams) *BleFsm {
	return &BleFsm{
		bx:           p.Bx,
		ownAddrType:  p.OwnAddrType,
		peer:         p.Peer,
		svcUuid:      p.SvcUuid,
		reqChrUuid:   p.ReqChrUuid,
		rspChrUuid:   p.RspChrUuid,
		rxNmpCb:      p.RxNmpCb,
		disconnectCb: p.DisconnectCb,

		bls:    map[*BleListener]struct{}{},
		attMtu: DFLT_ATT_MTU,
	}
}

func (bf *BleFsm) disconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) peer=%s handle=%d",
		ErrCodeToString(reason), reason, bf.peer.String(), bf.connHandle)
	return nmxutil.NewBleSesnDisconnectError(reason, str)
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

	if err := bf.bx.Bd.AddListener(base, bl); err != nil {
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
	bl := bf.bx.Bd.RemoveListener(base)
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
			case <-bl.ErrChan:
				return

			case bm := <-bl.BleChan:
				switch msg := bm.(type) {
				case *BleConnectRsp:
					bl.Acked = true
					if msg.Status != 0 {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%s (%d) peer=%s",
							ErrCodeToString(msg.Status), msg.Status,
							bf.peer.String())
						log.Debugf(str)
						bf.connChan <- nmxutil.NewBleHostError(msg.Status, str)
						return
					}

				case *BleConnectEvt:
					if msg.Status == 0 {
						bl.Acked = true
						log.Debugf("BLE connection attempt succeeded; "+
							"peer=%d handle=%d", bf.peer.String(),
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
							bf.peer.String())
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
					log.Debugf(err.Error())

					bf.mtx.Lock()
					bls := make([]*BleListener, 0, len(bf.bls))
					for bl, _ := range bf.bls {
						bls = append(bls, bl)
					}
					bf.mtx.Unlock()

					for _, bl := range bls {
						bl.ErrChan <- err
					}

					bf.setState(SESN_STATE_UNCONNECTED)
					bf.disconnectCb(err)
					return

				default:
				}

			case <-bl.AfterTimeout(bf.bx.rspTimeout):
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

						bf.rxNmpCb(msg.Data.Bytes)
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
	r.OwnAddrType = bf.ownAddrType
	r.PeerAddrType = bf.peer.AddrType
	r.PeerAddr = bf.peer.Addr

	if err := bf.connectListen(r.Seq); err != nil {
		return err
	}

	if err := connect(bf.bx, bf.connChan, r); err != nil {
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

	if err := terminate(bf.bx, bl, r); err != nil {
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

	if err := connCancel(bf.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) discSvcUuid() error {
	r := NewBleDiscSvcUuidReq()
	r.ConnHandle = bf.connHandle
	r.Uuid = bf.svcUuid

	bl, err := bf.addBleSeqListener(r.Seq)
	if err != nil {
		return err
	}
	defer bf.removeBleSeqListener(r.Seq)

	bf.nmpSvc, err = discSvcUuid(bf.bx, bl, r)
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

	chrs, err := discAllChrs(bf.bx, bl, r)
	if err != nil {
		return err
	}

	for _, c := range chrs {
		if CompareUuids(bf.reqChrUuid, c.Uuid) == 0 {
			bf.nmpReqChr = c
		}
		if CompareUuids(bf.rspChrUuid, c.Uuid) == 0 {
			bf.nmpRspChr = c
		}
	}

	if bf.nmpReqChr == nil {
		return fmt.Errorf(
			"Peer doesn't support required characteristic: %s",
			bf.reqChrUuid.String())
	}

	if bf.nmpRspChr == nil {
		return fmt.Errorf(
			"Peer doesn't support required characteristic: %s",
			bf.rspChrUuid.String())
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

	mtu, err := exchangeMtu(bf.bx, bl, r)
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

	if err := writeCmd(bf.bx, bl, r); err != nil {
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

	if err := writeCmd(bf.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) Start() error {
	for {
		state := bf.getState()
		switch state {
		case SESN_STATE_UNCONNECTED:
			cb := func() error { return bf.connect() }
			err := bf.action(
				SESN_STATE_UNCONNECTED,
				SESN_STATE_CONNECTING,
				SESN_STATE_CONNECTED,
				cb)

			if err != nil {
				return err
			}

		case SESN_STATE_CONNECTED:
			cb := func() error { return bf.exchangeMtu() }
			err := bf.action(
				SESN_STATE_CONNECTED,
				SESN_STATE_EXCHANGING_MTU,
				SESN_STATE_EXCHANGED_MTU,
				cb)
			if err != nil {
				return err
			}

		case SESN_STATE_EXCHANGED_MTU:
			cb := func() error { return bf.discSvcUuid() }
			err := bf.action(
				SESN_STATE_EXCHANGED_MTU,
				SESN_STATE_DISCOVERING_SVC,
				SESN_STATE_DISCOVERED_SVC,
				cb)
			if err != nil {
				return err
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
				return err
			}

			if err := bf.subscribe(); err != nil {
				return err
			}

		case SESN_STATE_DISCOVERED_CHR:
			/* Open complete. */
			return nil

		case SESN_STATE_CONNECTING,
			SESN_STATE_DISCOVERING_SVC,
			SESN_STATE_DISCOVERING_CHR,
			SESN_STATE_TERMINATING:
			return fmt.Errorf("BleFsm already being opened")
		}
	}
}

// @return bool                 true if stop complete;
//                              false if disconnect is now pending.
func (bf *BleFsm) Stop() (bool, error) {
	state := bf.getState()

	switch state {
	case SESN_STATE_UNCONNECTED:
		return false, fmt.Errorf("BLE session already closed")

	case SESN_STATE_TERMINATING, SESN_STATE_CONN_CANCELLING:
		return false, fmt.Errorf("BLE session already being closed")

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
