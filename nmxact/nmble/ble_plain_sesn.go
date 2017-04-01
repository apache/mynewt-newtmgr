package nmble

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BlePlainSesn struct {
	bf           *BleFsm
	nls          map[*nmp.NmpListener]struct{}
	nd           *nmp.NmpDispatcher
	connTries    int
	closeTimeout time.Duration
	onCloseCb    sesn.BleOnCloseFn

	closeChan chan error
	mx        sync.Mutex
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		nls:          map[*nmp.NmpListener]struct{}{},
		nd:           nmp.NewNmpDispatcher(),
		connTries:    cfg.Ble.ConnTries,
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

func (bps *BlePlainSesn) setCloseChan() error {
	bps.mx.Lock()
	defer bps.mx.Unlock()

	if bps.closeChan != nil {
		return fmt.Errorf("Multiple listeners waiting for session to close")
	}

	bps.closeChan = make(chan error, 1)
	return nil
}

func (bps *BlePlainSesn) clearCloseChan() {
	bps.mx.Lock()
	defer bps.mx.Unlock()

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

func (bps *BlePlainSesn) blockUntilClosed(timeout time.Duration) error {
	if err := bps.setCloseChan(); err != nil {
		return err
	}
	defer bps.clearCloseChan()

	// If the session is already closed, we're done.
	if bps.bf.IsClosed() {
		return nil
	}

	// Block until close completes or timeout.
	return bps.listenForClose(timeout)
}

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.nd.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	var err error
	for i := 0; i < bps.connTries; i++ {
		log.Debugf("Opening BLE session; try %d/%d", i+1, bps.connTries)

		var retry bool
		retry, err = bps.bf.Start()
		if !retry {
			break
		}

		if bps.blockUntilClosed(1*time.Second) != nil {
			// Just close the session manually and report the original error.
			bps.Close()
			return err
		}

		log.Debugf("Connection to BLE peer dropped immediately; retrying")
	}

	return err
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

	// Block until close completes or timeout.
	return bps.listenForClose(bps.closeTimeout)
}

func (bps *BlePlainSesn) IsOpen() bool {
	return bps.bf.IsOpen()
}

func (bps *BlePlainSesn) onRxNmp(data []byte) {
	bps.nd.Dispatch(data)
}

func (bps *BlePlainSesn) onDisconnect(dt BleFsmDisconnectType, peer BleDev,
	err error) {

	for nl, _ := range bps.nls {
		nl.ErrChan <- err
	}

	// If someone is waiting for the session to close, unblock them.
	if bps.closeChan != nil {
		bps.closeChan <- err
	}

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
