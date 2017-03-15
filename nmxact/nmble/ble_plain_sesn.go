package nmble

import (
	"fmt"
	"sync"
	"time"

	"mynewt.apache.org/newt/nmxact/bledefs"
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/nmxutil"
	"mynewt.apache.org/newt/nmxact/sesn"
	"mynewt.apache.org/newt/util"
)

type BlePlainSesn struct {
	bf           *BleFsm
	nls          map[*nmp.NmpListener]struct{}
	nd           *nmp.NmpDispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn

	closeChan chan error
	mx        sync.Mutex
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		nls:          map[*nmp.NmpListener]struct{}{},
		nd:           nmp.NewNmpDispatcher(),
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
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
		Bx:           bx,
		OwnAddrType:  cfg.Ble.OwnAddrType,
		Peer:         cfg.Ble.Peer,
		SvcUuid:      svcUuid,
		ReqChrUuid:   chrUuid,
		RspChrUuid:   chrUuid,
		RxNmpCb:      func(d []byte) { bps.onRxNmp(d) },
		DisconnectCb: func(e error) { bps.onDisconnect(e) },
	})

	return bps
}

func (bps *BlePlainSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	nl := nmp.NewNmpListener()
	bps.nls[nl] = struct{}{}

	if err := bps.nd.AddListener(seq, nl); err != nil {
		delete(bps.nls, nl)
		return nil, err
	}

	return nl, nil
}

func (bps *BlePlainSesn) removeNmpListener(seq uint8) {
	listener := bps.nd.RemoveListener(seq)
	if listener != nil {
		delete(bps.nls, listener)
	}
}

// Returns true if a new channel was assigned.
func (bps *BlePlainSesn) setCloseChan() bool {
	bps.mx.Lock()
	defer bps.mx.Unlock()

	if bps.closeChan != nil {
		return false
	}

	bps.closeChan = make(chan error, 1)
	return true
}

func (bps *BlePlainSesn) clearCloseChan() {
	bps.mx.Lock()
	defer bps.mx.Unlock()

	bps.closeChan = nil
}

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.nd.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	return bps.bf.Start()
}

func (bps *BlePlainSesn) Close() error {
	if !bps.setCloseChan() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened BLE session")
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

	// Block until close completes or timeout.
	select {
	case <-bps.closeChan:
	case <-time.After(bps.closeTimeout):
	}

	return nil
}

func (bps *BlePlainSesn) IsOpen() bool {
	return bps.bf.IsOpen()
}

func (bps *BlePlainSesn) onRxNmp(data []byte) {
	bps.nd.Dispatch(data)
}

func (bps *BlePlainSesn) onDisconnect(err error) {
	for nl, _ := range bps.nls {
		nl.ErrChan <- err
	}

	// If the session is being closed, unblock the close() call.
	if bps.closeChan != nil {
		bps.closeChan <- err
	}
	if bps.onCloseCb != nil {
		bps.onCloseCb(bps, err)
	}
}

func (bps *BlePlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(m)
}

// Blocking.
func (bps *BlePlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
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
	return util.IntMin(mtu, bledefs.BLE_ATT_ATTR_MAX_LEN)
}
