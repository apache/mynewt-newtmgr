package bll

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"golang.org/x/net/context"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllOicSesn struct {
	cfg BllSesnCfg

	cln       ble.Client
	nmpReqChr *ble.Characteristic
	nmpRspChr *ble.Characteristic
	d         *omp.Dispatcher
	mtx       sync.Mutex
	attMtu    int
}

func NewBllOicSesn(cfg BllSesnCfg) *BllOicSesn {
	return &BllOicSesn{
		cfg: cfg,
	}
}

func (bos *BllOicSesn) listenDisconnect() {
	go func() {
		<-bos.cln.Disconnected()

		bos.mtx.Lock()
		bos.d.ErrorAll(fmt.Errorf("Disconnected"))
		bos.d.Stop()
		bos.mtx.Unlock()

		bos.cln = nil
	}()
}

func (bos *BllOicSesn) connect() error {
	log.Debugf("Connecting to peer")
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		bos.cfg.ConnTimeout))

	var err error
	bos.cln, err = ble.Connect(ctx, bos.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return fmt.Errorf("Failed to connect to peer after %s",
				bos.cfg.ConnTimeout.String())
		} else {
			return err
		}
	}

	bos.listenDisconnect()

	return nil
}

func (bos *BllOicSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := bos.cln.DiscoverProfile(true)
	if err != nil {
		return err
	}

	iotivitySvcUuid, _ := bledefs.ParseUuid(bledefs.IotivitySvcUuid)
	ompSvcUuid := bledefs.BleUuid{U16: bledefs.OmpSvcUuid}
	reqChrUuid, _ := bledefs.ParseUuid(bledefs.OmpReqChrUuid)
	rspChrUuid, _ := bledefs.ParseUuid(bledefs.OmpRspChrUuid)

	for _, s := range p.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return err
		}

		if bledefs.CompareUuids(uuid, iotivitySvcUuid) == 0 ||
			bledefs.CompareUuids(uuid, ompSvcUuid) == 0 {

			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return err
				}

				if bledefs.CompareUuids(uuid, reqChrUuid) == 0 {
					bos.nmpReqChr = c
				} else if bledefs.CompareUuids(uuid, rspChrUuid) == 0 {
					bos.nmpRspChr = c
				}
			}
		}
	}

	if bos.nmpReqChr == nil || bos.nmpRspChr == nil {
		return fmt.Errorf(
			"Peer doesn't support a suitable service / characteristic")
	}

	return nil
}

// Subscribes to the peer's characteristic implementing NMP.
func (bos *BllOicSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		bos.d.Dispatch(data)
	}

	if err := bos.cln.Subscribe(bos.nmpRspChr, false, onNotify); err != nil {
		return err
	}

	return nil
}

func (bos *BllOicSesn) exchangeMtu() error {
	mtu, err := exchangeMtu(bos.cln, bos.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	bos.attMtu = mtu
	return nil
}

func (bos *BllOicSesn) Open() error {
	if bos.IsOpen() {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

	d, err := omp.NewDispatcher(true, 3)
	if err != nil {
		return err
	}
	bos.d = d

	if err := bos.connect(); err != nil {
		return err
	}

	if err := bos.exchangeMtu(); err != nil {
		return err
	}

	if err := bos.discoverAll(); err != nil {
		return err
	}

	if err := bos.subscribe(); err != nil {
		return err
	}

	return nil
}

func (bos *BllOicSesn) Close() error {
	if !bos.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := bos.cln.CancelConnection(); err != nil {
		return err
	}

	bos.cln = nil

	return nil
}

// Indicates whether the session is currently open.
func (bos *BllOicSesn) IsOpen() bool {
	return bos.cln != nil
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (bos *BllOicSesn) MtuOut() int {
	return bos.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Retrieves the maximum data payload for incoming NMP responses.
func (bos *BllOicSesn) MtuIn() int {
	return bos.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (bos *BllOicSesn) AbortRx(nmpSeq uint8) error {
	return bos.d.ErrorOneNmp(nmpSeq, fmt.Errorf("Rx aborted"))
}

func (bos *BllOicSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpTcp(msg)
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (bos *BllOicSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bos.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	b, err := bos.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	nl, err := bos.d.AddNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bos.d.RemoveNmpListener(msg.Hdr.Seq)

	// Send request.
	if err := bos.cln.WriteCharacteristic(bos.nmpReqChr, b, true); err != nil {
		return nil, err
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case <-nl.AfterTimeout(opt.Timeout):
			msg := fmt.Sprintf(
				"NMP timeout; op=%d group=%d id=%d seq=%d",
				msg.Hdr.Op, msg.Hdr.Group, msg.Hdr.Id, msg.Hdr.Seq)

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bos *BllOicSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("BllOicSesn.GetResourceOnce() unimplemented")
}
