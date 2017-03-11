package nmble

import (
	"encoding/hex"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/omp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type BleOicSesn struct {
	bf  *BleFsm
	nls map[*nmp.NmpListener]struct{}
	od  *omp.OmpDispatcher
}

func NewBleOicSesn(bx *BleXport, ownAddrType AddrType,
	peer BleDev) *BleOicSesn {

	bos := &BleOicSesn{
		nls: map[*nmp.NmpListener]struct{}{},
		od:  omp.NewOmpDispatcher(),
	}

	rxNmpCb := func(d []byte) { bos.onRxNmp(d) }
	disconnectCb := func(e error) { bos.onDisconnect(e) }

	svcUuid, err := ParseUuid(NmpOicSvcUuid)
	if err != nil {
		panic(err.Error())
	}

	reqChrUuid, err := ParseUuid(NmpOicReqChrUuid)
	if err != nil {
		panic(err.Error())
	}

	rspChrUuid, err := ParseUuid(NmpOicRspChrUuid)
	if err != nil {
		panic(err.Error())
	}

	bos.bf = NewBleFsm(BleFsmParams{
		Bx:           bx,
		OwnAddrType:  ownAddrType,
		Peer:         peer,
		SvcUuid:      svcUuid,
		ReqChrUuid:   reqChrUuid,
		RspChrUuid:   rspChrUuid,
		RxNmpCb:      rxNmpCb,
		DisconnectCb: disconnectCb,
	})

	return bos
}

func (bos *BleOicSesn) addOmpListener(seq uint8) (*nmp.NmpListener, error) {
	nl := nmp.NewNmpListener()
	bos.nls[nl] = struct{}{}

	if err := bos.od.AddListener(seq, nl); err != nil {
		delete(bos.nls, nl)
		return nil, err
	}

	return nl, nil
}

func (bos *BleOicSesn) removeOmpListener(seq uint8) {
	listener := bos.od.RemoveListener(seq)
	if listener != nil {
		delete(bos.nls, listener)
	}
}

func (bos *BleOicSesn) AbortRx(seq uint8) error {
	return bos.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bos *BleOicSesn) Open() error {
	return bos.bf.Start()
}

func (bos *BleOicSesn) Close() error {
	return bos.bf.Stop()
}

func (bos *BleOicSesn) onRxNmp(data []byte) {
	bos.od.Dispatch(data)
}

func (bos *BleOicSesn) onDisconnect(err error) {
	for nl, _ := range bos.nls {
		nl.ErrChan <- err
	}
}

// Blocking.
func (bos *BleOicSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	// Make sure peer is connected.
	if err := bos.Open(); err != nil {
		return nil, err
	}

	nl, err := bos.addOmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bos.removeOmpListener(msg.Hdr.Seq)

	msg.Encode()
	b, err := omp.SerializeOmgrMsg(msg)
	if err != nil {
		return nil, err
	}

	if opt.Timeout != 0 {
		go func() {
			time.Sleep(opt.Timeout)
			nl.ErrChan <- sesn.NewTimeoutError("OMP timeout")
		}()
	}

	log.Debugf("Tx OMP request:\n%s", hex.Dump(b))
	if err := bos.bf.writeCmd(b); err != nil {
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

func (bos *BleOicSesn) MtuIn() int {
	return bos.bf.attMtu - WRITE_CMD_BASE_SZ
}

func (bos *BleOicSesn) MtuOut() int {
	return bos.bf.attMtu - WRITE_CMD_BASE_SZ
}
