package nmble

import (
	"fmt"
	"time"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BlePlainSesn struct {
	bf           *BleFsm
	d            *nmp.Dispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn

	closeChan chan error
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		d:            nmp.NewDispatcher(1),
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
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		Central: BleFsmParamsCentral{
			PeerDev:     cfg.PeerSpec.Ble,
			ConnTries:   cfg.Ble.Central.ConnTries,
			ConnTimeout: cfg.Ble.Central.ConnTimeout,
		},
		SvcUuids:    []BleUuid{svcUuid},
		ReqChrUuid:  chrUuid,
		RspChrUuid:  chrUuid,
		EncryptWhen: cfg.Ble.EncryptWhen,
		RxDataCb:    func(d []byte) { bps.onRxNmp(d) },
		DisconnectCb: func(dt BleFsmDisconnectType, p BleDev, e error) {
			bps.onDisconnect(dt, p, e)
		},
	})

	return bps
}

func (bps *BlePlainSesn) setCloseChan() error {
	if bps.closeChan != nil {
		return fmt.Errorf("Multiple listeners waiting for session to close")
	}

	bps.closeChan = make(chan error, 1)
	return nil
}

func (bps *BlePlainSesn) clearCloseChan() {
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
	return bps.d.ErrorOne(seq, fmt.Errorf("Rx aborted"))
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
	bps.d.Dispatch(data)
}

// Called by the FSM when a blehostd disconnect event is received.
func (bps *BlePlainSesn) onDisconnect(dt BleFsmDisconnectType, peer BleDev,
	err error) {

	bps.d.ErrorAll(err)

	// If someone is waiting for the session to close, unblock them.
	if bps.closeChan != nil {
		bps.closeChan <- err
	}

	// Only execute client's disconnect callback if the disconnect was
	// unsolicited and the session was fully open.
	if dt == FSM_DISCONNECT_TYPE_OPENED && bps.onCloseCb != nil {
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
		return nil, bps.bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	nl, err := bps.d.AddListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.d.RemoveListener(msg.Hdr.Seq)

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

func (bps *BlePlainSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("BlePlainSesn.GetResourceOnce() unsupported")
}
