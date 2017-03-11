package nmble

import (
	"fmt"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/sesn"
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
)

type BleRxNmpFn func(data []byte)
type BleDisconnectFn func(err error)

type BleFsmParams struct {
	Bx           *BleXport
	OwnAddrType  AddrType
	Peer         BleDev
	SvcUuid      BleUuid
	ReqChrUuid   BleUuid
	RspChrUuid   BleUuid
	RxNmpCb      BleRxNmpFn
	DisconnectCb BleDisconnectFn
}

type BleFsm struct {
	bx           *BleXport
	ownAddrType  AddrType
	peer         BleDev
	svcUuid      BleUuid
	reqChrUuid   BleUuid
	rspChrUuid   BleUuid
	rxNmpCb      BleRxNmpFn
	disconnectCb BleDisconnectFn

	state      BleSesnState
	connHandle int
	bls        map[*BleListener]struct{}
	nmpSvc     *BleSvc
	nmpReqChr  *BleChr
	nmpRspChr  *BleChr
	attMtu     int
	connChan   chan error
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

func (bf *BleFsm) getState() BleSesnState {
	val := atomic.LoadInt32((*int32)(&bf.state))
	return BleSesnState(val)
}

func (bf *BleFsm) setState(toState BleSesnState) {
	atomic.StoreInt32((*int32)(&bf.state), int32(toState))
}

func (bf *BleFsm) transitionState(fromState BleSesnState,
	toState BleSesnState) error {

	swapped := atomic.CompareAndSwapInt32((*int32)(&bf.state),
		int32(fromState), int32(toState))
	if !swapped {
		return fmt.Errorf(
			"Can't set BleFsm state to %d; current state != required "+
				"value: %d",
			toState, fromState)
	}

	return nil
}

func (bf *BleFsm) addBleListener(base BleMsgBase) (*BleListener, error) {
	bl := NewBleListener()
	bf.bls[bl] = struct{}{}

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
	listener := bf.bx.Bd.RemoveListener(base)
	if listener != nil {
		delete(bf.bls, listener)
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
					if msg.Status != 0 {
						str := fmt.Sprintf("BLE connection attempt failed; "+
							"status=%d peer=%s", msg.Status, bf.peer.String())
						log.Debugf(str)
						bf.connChan <- NewBleHostError(msg.Status, str)
						return
					}

				case *BleConnectEvt:
					if msg.Status == 0 {
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
							"status=%d peer=%s", msg.Status, bf.peer.String())
						log.Debugf(str)
						bf.connChan <- NewBleHostError(msg.Status, str)
						return
					}

				case *BleMtuChangeEvt:
					if msg.Status != 0 {
						log.Debugf("BLE ATT MTU update failure; status=%d",
							msg.Status)
					} else {
						log.Debugf("BLE ATT MTU updated; from=%d to=%d",
							bf.attMtu, msg.Mtu)
						bf.attMtu = msg.Mtu
					}

				case *BleDisconnectEvt:
					str := fmt.Sprintf("BLE peer disconnected;"+
						"reason=%d peer=%s handle=%d",
						msg.Reason, bf.peer.String(), bf.connHandle)
					log.Debugf(str)

					err := sesn.NewDisconnectError(str)
					for bl, _ := range bf.bls {
						bl.ErrChan <- err
					}
					bf.disconnectCb(err)
					return

				default:
				}
			}
		}
	}()
	return nil
}

func (bf *BleFsm) nmpRspListen() error {
	bl := NewBleListener()
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
				// XXX: Make sure nothing else needs to happen here.
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
	r.PeerAddr.Bytes = bf.peer.Addr

	if err := bf.connectListen(r.Seq); err != nil {
		return err
	}

	if err := bf.nmpRspListen(); err != nil {
		return err
	}

	if err := connect(bf.bx, bf.connChan, r); err != nil {
		return err
	}

	return nil
}

func (bf *BleFsm) terminate() error {
	if bf.connHandle == 0 {
		// Already disconnected.
		return nil
	}

	r := NewBleTerminateReq()
	r.ConnHandle = bf.connHandle
	r.HciReason = BLE_ERR_REM_USER_CONN_TERM

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

func (bf *BleFsm) Stop() error {
	return bf.terminate()
}
