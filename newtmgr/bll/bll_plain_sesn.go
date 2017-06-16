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
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllPlainSesn struct {
	cfg BllSesnCfg

	cln    ble.Client
	nmpChr *ble.Characteristic
	nd     *nmp.NmpDispatcher
	nls    map[*nmp.NmpListener]struct{}
	mtx    sync.Mutex
	attMtu int
}

func NewBllPlainSesn(cfg BllSesnCfg) *BllPlainSesn {
	return &BllPlainSesn{
		cfg: cfg,
		nd:  nmp.NewNmpDispatcher(),
		nls: map[*nmp.NmpListener]struct{}{},
	}
}

func (bps *BllPlainSesn) listenDisconnect() {
	go func() {
		<-bps.cln.Disconnected()

		bps.mtx.Lock()
		for nl, _ := range bps.nls {
			nl.ErrChan <- fmt.Errorf("Disconnected")
		}
		bps.mtx.Unlock()

		bps.cln = nil
	}()
}

func (bps *BllPlainSesn) connect() error {
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

func (bps *BllPlainSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := bps.cln.DiscoverProfile(true)
	if err != nil {
		return err
	}

	svcUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainSvcUuid)
	chrUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainChrUuid)

	for _, s := range p.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return err
		}

		if bledefs.CompareUuids(uuid, svcUuid) == 0 {
			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return err
				}

				if bledefs.CompareUuids(uuid, chrUuid) == 0 {
					bps.nmpChr = c
					return nil
				}
			}
		}
	}

	return fmt.Errorf(
		"Peer doesn't support a suitable service / characteristic")
}

// Subscribes to the peer's characteristic implementing NMP.
func (bps *BllPlainSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		bps.nd.Dispatch(data)
	}

	if err := bps.cln.Subscribe(bps.nmpChr, false, onNotify); err != nil {
		return err
	}

	return nil
}

func (bps *BllPlainSesn) exchangeMtu() error {
	log.Debugf("Exchanging MTU")
	mtu, err := bps.cln.ExchangeMTU(bps.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	log.Debugf("Exchanged MTU; ATT MTU = %d", mtu)
	bps.attMtu = mtu
	return nil
}

func (bps *BllPlainSesn) Open() error {
	if bps.IsOpen() {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

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

func (bps *BllPlainSesn) Close() error {
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
func (bps *BllPlainSesn) IsOpen() bool {
	return bps.cln != nil
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (bps *BllPlainSesn) MtuOut() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Retrieves the maximum data payload for incoming NMP responses.
func (bps *BllPlainSesn) MtuIn() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (bps *BllPlainSesn) AbortRx(nmpSeq uint8) error {
	return bps.nd.FakeRxError(nmpSeq, fmt.Errorf("Rx aborted"))
}

func (bps *BllPlainSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(msg)
}

func (bps *BllPlainSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	nmxutil.LogAddNmpListener(2, seq)

	nl := nmp.NewNmpListener()
	if err := bps.nd.AddListener(seq, nl); err != nil {
		return nil, err
	}

	bps.nls[nl] = struct{}{}
	return nl, nil
}

func (bps *BllPlainSesn) removeNmpListener(seq uint8) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	nmxutil.LogRemoveNmpListener(2, seq)

	listener := bps.nd.RemoveListener(seq)
	if listener != nil {
		delete(bps.nls, listener)
	}
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (bps *BllPlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	b, err := bps.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	nl, err := bps.addNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.removeNmpListener(msg.Hdr.Seq)

	// Send request.
	if err := bps.cln.WriteCharacteristic(bps.nmpChr, b, true); err != nil {
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

func (bps *BllPlainSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil,
		fmt.Errorf("Resource API not supported by plain (non-OIC) session")
}
