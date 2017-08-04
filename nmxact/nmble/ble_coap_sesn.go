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

type BleCoapSesn struct {
	bf           *BleFsm
	d            *oic.Dispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn
	wg           sync.WaitGroup

	closeChan chan struct{}
}

func NewBleCoapSesn(bx *BleXport, cfg sesn.SesnCfg) *BleCoapSesn {
	bcs := &BleCoapSesn{
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
	}

	bcs.bf = NewBleFsm(BleFsmParams{
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		Central: BleFsmParamsCentral{
			PeerDev:     cfg.PeerSpec.Ble,
			ConnTries:   cfg.Ble.Central.ConnTries,
			ConnTimeout: cfg.Ble.Central.ConnTimeout,
		},

		EncryptWhen: cfg.Ble.EncryptWhen,
	})

	return bcs
}

func (bcs *BleCoapSesn) AbortRx(seq uint8) error {
	return fmt.Errorf("BleCoapSesn.AbortRx unimplemented")
}

func (bcs *BleCoapSesn) openInit() {
	// This channel gets closed when the session closes.
	bcs.closeChan = make(chan struct{})
	bcs.d = oic.NewDispatcher(true, 3)

	// Listen for disconnect in the background.
	bcs.wg.Add(1)
	go func() {
		// If the session is being closed, unblock the close() call.
		defer close(bcs.closeChan)

		// Block until disconnect.
		<-bcs.bf.DisconnectChan()
		nmxutil.Assert(!bcs.IsOpen())
		pd := bcs.bf.PrevDisconnect()

		// Signal error to all listeners.
		bcs.d.ErrorAll(pd.Err)
		bcs.wg.Done()
		bcs.wg.Wait()

		// Only execute the client's disconnect callback if the disconnect was
		// unsolicited.
		if pd.Dt != FSM_DISCONNECT_TYPE_REQUESTED && bcs.onCloseCb != nil {
			bcs.onCloseCb(bcs, pd.Err)
		}
	}()
}

func (bcs *BleCoapSesn) OpenConnected(
	connHandle uint16, eventListener *Listener) error {

	bcs.openInit()

	if err := bcs.bf.StartConnected(connHandle, eventListener); err != nil {
		close(bcs.closeChan)
		return err
	}
	return nil
}

func (bcs *BleCoapSesn) Open() error {
	bcs.openInit()

	if err := bcs.bf.Start(); err != nil {
		close(bcs.closeChan)
		return err
	}
	return nil
}

func (bcs *BleCoapSesn) Close() error {
	err := bcs.bf.Stop()
	if err != nil {
		return err
	}

	// Block until close completes.
	<-bcs.closeChan
	return nil
}

func (bcs *BleCoapSesn) IsOpen() bool {
	return bcs.bf.IsOpen()
}

func (bcs *BleCoapSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nil, fmt.Errorf("BleCoapSesn.EncodeNmpMsg unimplemented")
}

// Blocking.
func (bcs *BleCoapSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	return nil, fmt.Errorf("BleCoapSesn.TxNmpOnce unimplemented")
}

func (bcs *BleCoapSesn) MtuIn() int {
	return bcs.bf.attMtu -
		NOTIFY_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (bcs *BleCoapSesn) MtuOut() int {
	mtu := bcs.bf.attMtu -
		WRITE_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
	return util.IntMin(mtu, BLE_ATT_ATTR_MAX_LEN)
}

func (bcs *BleCoapSesn) ConnInfo() (BleConnDesc, error) {
	return bcs.bf.connInfo()
}

func (bcs *BleCoapSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	token := nmxutil.NextToken()

	ol, err := bcs.d.AddListener(token)
	if err != nil {
		return nil, err
	}
	defer bcs.d.RemoveListener(token)

	req, err := oic.EncodeGet(uri, token)
	if err != nil {
		return nil, err
	}

	rsp, err := bcs.bf.TxOic(req, ol, opt.Timeout)
	if err != nil {
		return nil, err
	}

	if rsp.Code != coap.Content {
		return nil, fmt.Errorf("UNEXPECTED OIC ACK: %#v", rsp)
	}

	return rsp.Payload, nil
}
