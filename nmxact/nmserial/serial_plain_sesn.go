package nmserial

import (
	"fmt"
	"sync"

	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type SerialPlainSesn struct {
	sx *SerialXport
	nd *nmp.NmpDispatcher

	// This mutex ensures each response get matched up with its corresponding
	// request.
	m sync.Mutex
}

func NewSerialPlainSesn(sx *SerialXport) *SerialPlainSesn {
	return &SerialPlainSesn{
		sx: sx,
		nd: nmp.NewNmpDispatcher(),
	}
}

func (sps *SerialPlainSesn) Open() error {
	return nil
}

func (sps *SerialPlainSesn) Close() error {
	return nil
}

func (sps *SerialPlainSesn) MtuIn() int {
	return 1024
}

func (sps *SerialPlainSesn) MtuOut() int {
	return 1024
}

func (sps *SerialPlainSesn) AbortRx(seq uint8) error {
	return sps.nd.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (sps *SerialPlainSesn) addNmpListener(seq uint8) (
	*nmp.NmpListener, error) {

	nl := nmp.NewNmpListener()
	if err := sps.nd.AddListener(seq, nl); err != nil {
		return nil, err
	}

	return nl, nil
}

func (sps *SerialPlainSesn) removeNmpListener(seq uint8) {
	sps.nd.RemoveListener(seq)
}

func (sps *SerialPlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	sps.m.Lock()
	defer sps.m.Unlock()

	nl, err := sps.addNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer sps.removeNmpListener(msg.Hdr.Seq)

	reqb, err := msg.Encode()
	if err != nil {
		return nil, err
	}

	if err := sps.sx.Tx(reqb); err != nil {
		return nil, err
	}

	for {
		rspb, err := sps.sx.Rx()
		if err != nil {
			return nil, err
		}

		// Now wait for newtmgr response.
		if sps.nd.Dispatch(rspb) {
			select {
			case err := <-nl.ErrChan:
				return nil, err
			case rsp := <-nl.RspChan:
				return rsp, nil
			}
		}
	}
}
