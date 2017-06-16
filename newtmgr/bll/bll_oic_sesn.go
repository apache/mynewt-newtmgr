package bll

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"golang.org/x/net/context"

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
	rxer      *omp.Receiver
	mtx       sync.Mutex
	attMtu    int
}

func NewBllOicSesn(cfg BllSesnCfg) *BllOicSesn {
	return &BllOicSesn{
		cfg: cfg,
	}
}

func (bps *BllOicSesn) listenDisconnect() {
	go func() {
		<-bps.cln.Disconnected()

		bps.mtx.Lock()
		bps.rxer.ErrorAll(fmt.Errorf("Disconnected"))
		bps.mtx.Unlock()

		bps.cln = nil
	}()
}

func (bps *BllOicSesn) connect() error {
	log.Debugf("Connecting to peer")
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		10*time.Second))

	var err error
	bps.cln, err = ble.Connect(ctx, bps.cfg.AdvFilter)
	if err != nil {
		return err
	}

	bps.listenDisconnect()

	return nil
}

func (bps *BllOicSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := bps.cln.DiscoverProfile(true)
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
					bps.nmpReqChr = c
				} else if bledefs.CompareUuids(uuid, rspChrUuid) == 0 {
					bps.nmpRspChr = c
				}
			}
		}
	}

	if bps.nmpReqChr == nil || bps.nmpRspChr == nil {
		return fmt.Errorf(
			"Peer doesn't support a suitable service / characteristic")
	}

	return nil
}

// Subscribes to the peer's characteristic implementing NMP.
func (bps *BllOicSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		bps.rxer.Rx(data)
	}

	if err := bps.cln.Subscribe(bps.nmpRspChr, false, onNotify); err != nil {
		return err
	}

	return nil
}

func (bps *BllOicSesn) exchangeMtu() error {
	log.Debugf("Exchanging MTU")
	mtu, err := bps.cln.ExchangeMTU(bps.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	log.Debugf("Exchanged MTU; ATT MTU = %d", mtu)
	bps.attMtu = mtu
	return nil
}

func (bps *BllOicSesn) Open() error {
	if bps.IsOpen() {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

	bps.rxer = omp.NewReceiver(true)

	if err := bps.connect(); err != nil {
		return err
	}

	if err := bps.exchangeMtu(); err != nil {
		return err
	}

	if err := bps.discoverAll(); err != nil {
		return err
	}

	if err := bps.subscribe(); err != nil {
		return err
	}

	return nil
}

func (bps *BllOicSesn) Close() error {
	if !bps.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := bps.cln.CancelConnection(); err != nil {
		return err
	}

	bps.cln = nil

	return nil
}

// Indicates whether the session is currently open.
func (bps *BllOicSesn) IsOpen() bool {
	return bps.cln != nil
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (bps *BllOicSesn) MtuOut() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Retrieves the maximum data payload for incoming NMP responses.
func (bps *BllOicSesn) MtuIn() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (bps *BllOicSesn) AbortRx(nmpSeq uint8) error {
	return bps.rxer.FakeNmpError(nmpSeq, fmt.Errorf("Rx aborted"))
}

func (bps *BllOicSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpTcp(msg)
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (bps *BllOicSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	b, err := bps.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	nl, err := bps.rxer.AddNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.rxer.RemoveNmpListener(msg.Hdr.Seq)

	// Send request.
	if err := bps.cln.WriteCharacteristic(bps.nmpReqChr, b, true); err != nil {
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
				b[0], b[4]+b[5]<<8, b[7], b[6])

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bps *BllOicSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("BllOicSesn.GetResourceOnce() unimplemented")
}
