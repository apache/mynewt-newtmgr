package nmble

import (
	"encoding/hex"
	"fmt"
	"sync"
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

	closeChan chan error
	mx        sync.Mutex
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

func (bos *BleOicSesn) addNmpListener(seq uint8) (*nmp.NmpListener, error) {
	nl := nmp.NewNmpListener()
	bos.nls[nl] = struct{}{}

	if err := bos.od.AddListener(seq, nl); err != nil {
		delete(bos.nls, nl)
		return nil, err
	}

	return nl, nil
}

func (bos *BleOicSesn) removeNmpListener(seq uint8) {
	listener := bos.od.RemoveListener(seq)
	if listener != nil {
		delete(bos.nls, listener)
	}
}

// Returns true if a new channel was assigned.
func (bos *BleOicSesn) setCloseChan() bool {
	bos.mx.Lock()
	defer bos.mx.Unlock()

	if bos.closeChan != nil {
		return false
	}

	bos.closeChan = make(chan error, 1)
	return true
}

func (bos *BleOicSesn) clearCloseChan() {
	bos.mx.Lock()
	defer bos.mx.Unlock()

	bos.closeChan = nil
}

func (bos *BleOicSesn) AbortRx(seq uint8) error {
	return bos.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (bos *BleOicSesn) Open() error {
	return bos.bf.Start()
}

func (bos *BleOicSesn) Close() error {
	if !bos.setCloseChan() {
		return fmt.Errorf("BLE session already being closed")
	}
	defer bos.clearCloseChan()

	done, err := bos.bf.Stop()
	if err != nil {
		return err
	}

	if done {
		// Close complete.
		return nil
	}

	// Block until close completes or timeout.
	select {
	case <-bos.closeChan:
	case <-time.After(CLOSE_TIMEOUT):
	}

	return nil
}

func (bos *BleOicSesn) onRxNmp(data []byte) {
	bos.od.Dispatch(data)
}

func (bos *BleOicSesn) onDisconnect(err error) {
	for nl, _ := range bos.nls {
		nl.ErrChan <- err
	}

	// If the session is being closed, unblock the close() call.
	if bos.closeChan != nil {
		bos.closeChan <- err
	}
}

// Blocking.
func (bos *BleOicSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	// Make sure peer is connected.
	if err := bos.Open(); err != nil {
		return nil, err
	}

	nl, err := bos.addNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bos.removeNmpListener(msg.Hdr.Seq)

	b, err := omp.EncodeOmpTcp(msg)
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx NMP request: %s", hex.Dump(b))
	if err := bos.bf.writeCmd(b); err != nil {
		return nil, err
	}

	// Now wait for newtmgr response.
	select {
	case err := <-nl.ErrChan:
		return nil, err
	case rsp := <-nl.RspChan:
		return rsp, nil
	case <-opt.AfterTimeout():
		return nil, sesn.NewTimeoutError("NMP timeout")
	}
}

func (bos *BleOicSesn) MtuIn() int {
	return bos.bf.attMtu -
		NOTIFY_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (bos *BleOicSesn) MtuOut() int {
	return bos.bf.attMtu -
		WRITE_CMD_BASE_SZ -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}
