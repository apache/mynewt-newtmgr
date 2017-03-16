package nmble

import (
	"fmt"
	"sync"
	"time"

	"mynewt.apache.org/newt/nmxact/bledefs"
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/omp"
	"mynewt.apache.org/newt/nmxact/sesn"
	"mynewt.apache.org/newt/util"
)

type BleOicSesn struct {
	bf           *BleFsm
	nls          map[*nmp.NmpListener]struct{}
	od           *omp.OmpDispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn

	closeChan chan error
	mx        sync.Mutex
}

func NewBleOicSesn(bx *BleXport, cfg sesn.SesnCfg) *BleOicSesn {
	bos := &BleOicSesn{
		nls:          map[*nmp.NmpListener]struct{}{},
		od:           omp.NewOmpDispatcher(),
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
	}

	svcUuid, err := ParseUuid(NmpOicSvcUuid)
	if err != nil {
		panic(err.Error())
	}

	reqChrUuid, err := ParseUuid(NmpOicReqChrUuid)
	if err != nil {
		panic(err.Error())
	}

	rspChrUuid, err := ParseUuid(NmpOicRspChrUuid)
	if err != nil {
		panic(err.Error())
	}

	bos.bf = NewBleFsm(BleFsmParams{
		Bx:           bx,
		OwnAddrType:  cfg.Ble.OwnAddrType,
		Peer:         cfg.Ble.Peer,
		SvcUuid:      svcUuid,
		ReqChrUuid:   reqChrUuid,
		RspChrUuid:   rspChrUuid,
		RxNmpCb:      func(d []byte) { bos.onRxNmp(d) },
		DisconnectCb: func(e error) { bos.onDisconnect(e) },
	})

	return bos
}

func (bos *BleOicSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	nl := nmp.NewNmpListener()
	bos.nls[nl] = struct{}{}

	if err := bos.od.AddListener(seq, nl); err != nil {
		delete(bos.nls, nl)
		return nil, err
	}

	return nl, nil
}

func (bos *BleOicSesn) removeNmpListener(seq uint8) {
	listener := bos.od.RemoveListener(seq)
	if listener != nil {
		delete(bos.nls, listener)
	}
}

// Returns true if a new channel was assigned.
func (bos *BleOicSesn) setCloseChan() bool {
	bos.mx.Lock()
	defer bos.mx.Unlock()

	if bos.closeChan != nil {
		return false
	}

	bos.closeChan = make(chan error, 1)
	return true
}

func (bos *BleOicSesn) clearCloseChan() {
	bos.mx.Lock()
	defer bos.mx.Unlock()

	bos.closeChan = nil
}

func (bos *BleOicSesn) AbortRx(seq uint8) error {
	return bos.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bos *BleOicSesn) Open() error {
	return bos.bf.Start()
}

func (bos *BleOicSesn) Close() error {
	if !bos.setCloseChan() {
		return bos.bf.closedError(
			"Attempt to close an unopened BLE session")
	}
	defer bos.clearCloseChan()

	done, err := bos.bf.Stop()
	if err != nil {
		return err
	}

	if done {
		// Close complete.
		return nil
	}

	// Block until close completes or timeout.
	select {
	case <-bos.closeChan:
	case <-time.After(bos.closeTimeout):
	}

	return nil
}

func (bos *BleOicSesn) IsOpen() bool {
	return bos.bf.IsOpen()
}

func (bos *BleOicSesn) onRxNmp(data []byte) {
	bos.od.Dispatch(data)
}

func (bos *BleOicSesn) onDisconnect(err error) {
	for nl, _ := range bos.nls {
		nl.ErrChan <- err
	}

	// If the session is being closed, unblock the close() call.
	if bos.closeChan != nil {
		bos.closeChan <- err
	}
	if bos.onCloseCb != nil {
		bos.onCloseCb(bos, err)
	}
}

func (bos *BleOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpTcp(m)
}

// Blocking.
func (bos *BleOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bos.IsOpen() {
		return nil, bos.bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	nl, err := bos.addNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bos.removeNmpListener(m.Hdr.Seq)

	b, err := bos.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	return bos.bf.TxNmp(b, nl, opt.Timeout)
}

func (bos *BleOicSesn) MtuIn() int {
	return bos.bf.attMtu -
		NOTIFY_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (bos *BleOicSesn) MtuOut() int {
	mtu := bos.bf.attMtu -
		WRITE_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
	return util.IntMin(mtu, bledefs.BLE_ATT_ATTR_MAX_LEN)
}
