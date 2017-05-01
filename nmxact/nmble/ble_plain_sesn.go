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
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BlePlainSesn struct {
	bf           *BleFsm
	nls          map[*nmp.NmpListener]struct{}
	nd           *nmp.NmpDispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.BleOnCloseFn

	closeChan chan error
	mtx       sync.Mutex
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		nls:          map[*nmp.NmpListener]struct{}{},
		nd:           nmp.NewNmpDispatcher(),
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.Ble.OnCloseCb,
	}

	svcUuid, err := ParseUuid(NmpPlainSvcUuid)
	if err != nil {
		panic(err.Error())
	}

	chrUuid, err := ParseUuid(NmpPlainChrUuid)
	if err != nil {
		panic(err.Error())
	}

	bps.bf = NewBleFsm(BleFsmParams{
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		PeerSpec:    cfg.Ble.PeerSpec,
		ConnTries:   cfg.Ble.ConnTries,
		SvcUuid:     svcUuid,
		ReqChrUuid:  chrUuid,
		RspChrUuid:  chrUuid,
		RxNmpCb:     func(d []byte) { bps.onRxNmp(d) },
		DisconnectCb: func(dt BleFsmDisconnectType, p BleDev, e error) {
			bps.onDisconnect(dt, p, e)
		},
	})

	return bps
}

func (bps *BlePlainSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	_, file, line, _ := runtime.Caller(1)
	file = path.Base(file)
	nmxutil.ListenLog.Debugf("{add-nmp-listener} [%s:%d] seq=%+v",
		file, line, seq)

	nl := nmp.NewNmpListener()
	if err := bps.nd.AddListener(seq, nl); err != nil {
		return nil, err
	}

	bps.nls[nl] = struct{}{}
	return nl, nil
}

func (bps *BlePlainSesn) removeNmpListener(seq uint8) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	_, file, line, _ := runtime.Caller(1)
	file = path.Base(file)
	nmxutil.ListenLog.Debugf("{remove-nmp-listener} [%s:%d] seq=%+v",
		file, line, seq)

	listener := bps.nd.RemoveListener(seq)
	if listener != nil {
		delete(bps.nls, listener)
	}
}

func (bps *BlePlainSesn) setCloseChan() error {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	if bps.closeChan != nil {
		return fmt.Errorf("Multiple listeners waiting for session to close")
	}

	bps.closeChan = make(chan error, 1)
	return nil
}

func (bps *BlePlainSesn) clearCloseChan() {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	bps.closeChan = nil
}

func (bps *BlePlainSesn) listenForClose(timeout time.Duration) error {
	select {
	case <-bps.closeChan:
		return nil
	case <-time.After(timeout):
		// Session never closed.
		return fmt.Errorf("Timeout while waiting for session to close")
	}
}

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.nd.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	return bps.bf.Start()
}

func (bps *BlePlainSesn) Close() error {
	if err := bps.setCloseChan(); err != nil {
		return err
	}
	defer bps.clearCloseChan()

	done, err := bps.bf.Stop()
	if err != nil {
		return err
	}

	if done {
		// Close complete.
		return nil
	}

	// Block until close completes or times out.
	return bps.listenForClose(bps.closeTimeout)
}

func (bps *BlePlainSesn) IsOpen() bool {
	return bps.bf.IsOpen()
}

func (bps *BlePlainSesn) onRxNmp(data []byte) {
	bps.nd.Dispatch(data)
}

// Called by the FSM when a blehostd disconnect event is received.
func (bps *BlePlainSesn) onDisconnect(dt BleFsmDisconnectType, peer BleDev,
	err error) {

	bps.mtx.Lock()

	for nl, _ := range bps.nls {
		nl.ErrChan <- err
	}

	// If someone is waiting for the session to close, unblock them.
	if bps.closeChan != nil {
		bps.closeChan <- err
	}

	bps.mtx.Unlock()

	// Only execute client's disconnect callback if the disconnect was
	// unsolicited and the session was fully open.
	if dt == FSM_DISCONNECT_TYPE_OPENED && bps.onCloseCb != nil {
		bps.onCloseCb(bps, peer, err)
	}
}

func (bps *BlePlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(m)
}

// Blocking.
func (bps *BlePlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, bps.bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	nl, err := bps.addNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.removeNmpListener(msg.Hdr.Seq)

	b, err := bps.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	return bps.bf.TxNmp(b, nl, opt.Timeout)
}

func (bps *BlePlainSesn) MtuIn() int {
	return bps.bf.attMtu - NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

func (bps *BlePlainSesn) MtuOut() int {
	mtu := bps.bf.attMtu - WRITE_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
	return util.IntMin(mtu, BLE_ATT_ATTR_MAX_LEN)
}

func (bps *BlePlainSesn) ConnInfo() (BleConnDesc, error) {
	return bps.bf.connInfo()
}
