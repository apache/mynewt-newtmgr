package nmble

import (
	"fmt"
	"sync"
	"time"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BleOicSesn struct {
	bf           *BleFsm
	d            *omp.Dispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn
	wg           sync.WaitGroup

	closeChan chan struct{}
}

func NewBleOicSesn(bx *BleXport, cfg sesn.SesnCfg) *BleOicSesn {
	bos := &BleOicSesn{
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
	}

	iotUuid, _ := ParseUuid(OmpUnsecSvcUuid)
	svcUuids := []BleUuid{
		iotUuid,
	}

	reqChrUuid, _ := ParseUuid(OmpUnsecReqChrUuid)
	rspChrUuid, _ := ParseUuid(OmpUnsecRspChrUuid)

	bos.bf = NewBleFsm(BleFsmParams{
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		Central: BleFsmParamsCentral{
			PeerDev:     cfg.PeerSpec.Ble,
			ConnTries:   cfg.Ble.Central.ConnTries,
			ConnTimeout: cfg.Ble.Central.ConnTimeout,
		},
		SvcUuids:    svcUuids,
		ReqChrUuid:  reqChrUuid,
		RspChrUuid:  rspChrUuid,
		EncryptWhen: cfg.Ble.EncryptWhen,
	})

	return bos
}

func (bos *BleOicSesn) AbortRx(seq uint8) error {
	return bos.d.ErrorOneNmp(seq, fmt.Errorf("Rx aborted"))
}

func (bos *BleOicSesn) Open() error {
	// This channel gets closed when the session closes.
	bos.closeChan = make(chan struct{})

	if err := bos.bf.Start(); err != nil {
		close(bos.closeChan)
		return err
	}

	d, err := omp.NewDispatcher(true, 3)
	if err != nil {
		close(bos.closeChan)
		return err
	}
	bos.d = d

	// Listen for disconnect in the background.
	bos.wg.Add(1)
	go func() {
		// If the session is being closed, unblock the close() call.
		defer close(bos.closeChan)

		// Block until disconnect.
		<-bos.bf.DisconnectChan()
		nmxutil.Assert(!bos.IsOpen())
		pd := bos.bf.PrevDisconnect()

		// Signal error to all listeners.
		bos.d.ErrorAll(pd.Err)
		bos.d.Stop()
		bos.wg.Done()
		bos.wg.Wait()

		// Only execute the client's disconnect callback if the disconnect was
		// unsolicited.
		if pd.Dt != FSM_DISCONNECT_TYPE_REQUESTED && bos.onCloseCb != nil {
			bos.onCloseCb(bos, pd.Err)
		}
	}()

	// Listen for NMP responses in the background.
	bos.wg.Add(1)
	go func() {
		defer bos.wg.Done()

		for {
			data, ok := <-bos.bf.RxNmpChan()
			if !ok {
				// Disconnected.
				return
			} else {
				bos.d.Dispatch(data)
			}
		}
	}()

	return nil
}

func (bos *BleOicSesn) Close() error {
	err := bos.bf.Stop()
	if err != nil {
		return err
	}

	// Block until close completes.
	<-bos.closeChan
	return nil
}

func (bos *BleOicSesn) IsOpen() bool {
	return bos.bf.IsOpen()
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

	nl, err := bos.d.AddNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bos.d.RemoveNmpListener(m.Hdr.Seq)

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

func (bos *BleOicSesn) ConnInfo() (BleConnDesc, error) {
	return bos.bf.connInfo()
}

func (bos *BleOicSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	token := nmxutil.NextToken()

	ol, err := bos.d.AddOicListener(token)
	if err != nil {
		return nil, err
	}
	defer bos.d.RemoveOicListener(token)

	req, err := oic.EncodeGet(uri, token)
	if err != nil {
		return nil, err
	}

	rsp, err := bos.bf.TxOic(req, ol, opt.Timeout)
	if err != nil {
		return nil, err
	}

	if rsp.Code != coap.Content {
		return nil, fmt.Errorf("UNEXPECTED OIC ACK: %#v", rsp)
	}

	return rsp.Payload, nil
}
