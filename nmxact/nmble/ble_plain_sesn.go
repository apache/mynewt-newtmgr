package nmble

import (
	"encoding/hex"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type BlePlainSesn struct {
	bf  *BleFsm
	nls map[*nmp.NmpListener]struct{}
	nd  *nmp.NmpDispatcher
}

func NewBlePlainSesn(bx *BleXport, ownAddrType AddrType,
	peer BleDev) *BlePlainSesn {

	bps := &BlePlainSesn{
		nls: map[*nmp.NmpListener]struct{}{},
		nd:  nmp.NewNmpDispatcher(),
	}

	rxNmpCb := func(d []byte) { bps.onRxNmp(d) }
	disconnectCb := func(e error) { bps.onDisconnect(e) }

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
		OwnAddrType:  ownAddrType,
		Peer:         peer,
		SvcUuid:      svcUuid,
		ReqChrUuid:   chrUuid,
		RspChrUuid:   chrUuid,
		RxNmpCb:      rxNmpCb,
		DisconnectCb: disconnectCb,
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

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.nd.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	return bps.bf.Start()
}

func (bps *BlePlainSesn) Close() error {
	return bps.bf.Stop()
}

func (bps *BlePlainSesn) onRxNmp(data []byte) {
	bps.nd.Dispatch(data)
}

func (bps *BlePlainSesn) onDisconnect(err error) {
	for nl, _ := range bps.nls {
		nl.ErrChan <- err
	}
}

// Blocking.
func (bps *BlePlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	// Make sure peer is connected.
	if err := bps.Open(); err != nil {
		return nil, err
	}

	nl, err := bps.addNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.removeNmpListener(msg.Hdr.Seq)

	b, err := msg.Encode()
	if err != nil {
		return nil, err
	}

	if opt.Timeout != 0 {
		go func() {
			time.Sleep(opt.Timeout)
			nl.ErrChan <- sesn.NewTimeoutError("NMP timeout")
		}()
	}

	log.Debugf("Tx NMP request: %s", hex.Dump(b))
	if err := bps.bf.writeCmd(b); err != nil {
		return nil, err
	}

	// Now wait for newtmgr response.
	select {
	case err := <-nl.ErrChan:
		return nil, err
	case rsp := <-nl.RspChan:
		return rsp, nil
	}
}

func (bps *BlePlainSesn) MtuIn() int {
	return bps.bf.attMtu - WRITE_CMD_BASE_SZ
}
func (bps *BlePlainSesn) MtuOut() int {
	return bps.bf.attMtu - WRITE_CMD_BASE_SZ
}
