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

	closeChan chan struct{}
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
	}

	svcUuid, _ := ParseUuid(NmpPlainSvcUuid)
	chrUuid, _ := ParseUuid(NmpPlainChrUuid)

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
	})

	return bps
}

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.d.ErrorOne(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	// This channel gets closed when the session closes.
	bps.closeChan = make(chan struct{})

	if err := bps.bf.Start(); err != nil {
		close(bps.closeChan)
		return err
	}

	bps.d = nmp.NewDispatcher(3)

	// Listen for disconnect in the background.
	go func() {
		// Block until disconnect.
		entry := <-bps.bf.DisconnectChan()

		// Signal error to all listeners.
		bps.d.ErrorAll(entry.Err)

		// If the session is being closed, unblock the close() call.
		close(bps.closeChan)

		// Only execute the client's disconnect callback if the disconnect was
		// unsolicited.
		if entry.Dt != FSM_DISCONNECT_TYPE_REQUESTED && bps.onCloseCb != nil {
			bps.onCloseCb(bps, entry.Err)
		}
	}()

	// Listen for NMP responses in the background.
	go func() {
		for {
			data, ok := <-bps.bf.RxNmpChan()
			if !ok {
				// Disconnected.
				return
			} else {
				bps.d.Dispatch(data)
			}
		}
	}()

	return nil
}

func (bps *BlePlainSesn) Close() error {
	err := bps.bf.Stop()
	if err != nil {
		return err
	}

	// Block until close completes.
	<-bps.closeChan
	return nil
}

func (bps *BlePlainSesn) IsOpen() bool {
	return bps.bf.IsOpen()
}

// Called by the FSM when a blehostd disconnect event is received.
func (bps *BlePlainSesn) onDisconnect(dt BleFsmDisconnectType, peer BleDev,
	err error) {

	bps.d.ErrorAll(err)

	// If the session is being closed, unblock the close() call.
	close(bps.closeChan)

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
