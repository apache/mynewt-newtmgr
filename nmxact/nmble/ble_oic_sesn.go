package nmble

import (
	"fmt"
	"path"
	"runtime"
	"sync"
	"time"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BleOicSesn struct {
	bf           *BleFsm
	nls          map[*nmp.NmpListener]struct{}
	od           *omp.OmpDispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.BleOnCloseFn

	closeChan chan error
	mtx       sync.Mutex
}

func NewBleOicSesn(bx *BleXport, cfg sesn.SesnCfg) *BleOicSesn {
	bos := &BleOicSesn{
		nls:          map[*nmp.NmpListener]struct{}{},
		od:           omp.NewOmpDispatcher(),
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.Ble.OnCloseCb,
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
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		PeerSpec:    cfg.Ble.PeerSpec,
		ConnTries:   cfg.Ble.ConnTries,
		SvcUuid:     svcUuid,
		ReqChrUuid:  reqChrUuid,
		RspChrUuid:  rspChrUuid,
		RxNmpCb:     func(d []byte) { bos.onRxNmp(d) },
		DisconnectCb: func(dt BleFsmDisconnectType, p BleDev, e error) {
			bos.onDisconnect(dt, p, e)
		},
	})

	return bos
}

func (bos *BleOicSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	bos.mtx.Lock()
	defer bos.mtx.Unlock()

	_, file, line, _ := runtime.Caller(1)
	file = path.Base(file)
	nmxutil.ListenLog.Debugf("{add-nmp-listener} [%s:%d] seq=%+v",
		file, line, seq)

	nl := nmp.NewNmpListener()
	if err := bos.od.AddListener(seq, nl); err != nil {
		return nil, err
	}

	bos.nls[nl] = struct{}{}
	return nl, nil
}

func (bos *BleOicSesn) removeNmpListener(seq uint8) {
	bos.mtx.Lock()
	defer bos.mtx.Unlock()

	_, file, line, _ := runtime.Caller(1)
	file = path.Base(file)
	nmxutil.ListenLog.Debugf("{remove-nmp-listener} [%s:%d] seq=%+v",
		file, line, seq)

	listener := bos.od.RemoveListener(seq)
	if listener != nil {
		delete(bos.nls, listener)
	}
}

// Returns true if a new channel was assigned.
func (bos *BleOicSesn) setCloseChan() error {
	bos.mtx.Lock()
	defer bos.mtx.Unlock()

	if bos.closeChan != nil {
		return fmt.Errorf("Multiple listeners waiting for session to close")
	}

	bos.closeChan = make(chan error, 1)
	return nil
}

func (bos *BleOicSesn) clearCloseChan() {
	bos.mtx.Lock()
	defer bos.mtx.Unlock()

	bos.closeChan = nil
}

func (bos *BleOicSesn) listenForClose(timeout time.Duration) error {
	select {
	case <-bos.closeChan:
		return nil
	case <-time.After(timeout):
		// Session never closed.
		return fmt.Errorf("Timeout while waiting for session to close")
	}
}

func (bos *BleOicSesn) blockUntilClosed(timeout time.Duration) error {
	if err := bos.setCloseChan(); err != nil {
		return err
	}
	defer bos.clearCloseChan()

	// If the session is already closed, we're done.
	if bos.bf.IsClosed() {
		return nil
	}

	// Block until close completes or times out.
	return bos.listenForClose(timeout)
}

func (bos *BleOicSesn) AbortRx(seq uint8) error {
	return bos.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bos *BleOicSesn) Open() error {
	return bos.bf.Start()
}

func (bos *BleOicSesn) Close() error {
	if err := bos.setCloseChan(); err != nil {
		return err
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

	// Block until close completes or times out.
	return bos.listenForClose(bos.closeTimeout)
}

func (bos *BleOicSesn) IsOpen() bool {
	return bos.bf.IsOpen()
}

func (bos *BleOicSesn) onRxNmp(data []byte) {
	bos.od.Dispatch(data)
}

// Called by the FSM when a blehostd disconnect event is received.
func (bos *BleOicSesn) onDisconnect(dt BleFsmDisconnectType, peer BleDev,
	err error) {

	bos.mtx.Lock()

	for nl, _ := range bos.nls {
		nl.ErrChan <- err
	}

	// If the session is being closed, unblock the close() call.
	if bos.closeChan != nil {
		bos.closeChan <- err
	}

	bos.mtx.Unlock()

	// Only execute client's disconnect callback if the disconnect was
	// unsolicited and the session was fully open.
	if dt == FSM_DISCONNECT_TYPE_OPENED && bos.onCloseCb != nil {
		bos.onCloseCb(bos, peer, err)
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
	return util.IntMin(mtu, BLE_ATT_ATTR_MAX_LEN)
}
