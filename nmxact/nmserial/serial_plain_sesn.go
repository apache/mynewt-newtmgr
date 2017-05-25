package nmserial

import (
	"fmt"
	"sync"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialPlainSesn struct {
	sx     *SerialXport
	nd     *nmp.NmpDispatcher
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex
}

func NewSerialPlainSesn(sx *SerialXport) *SerialPlainSesn {
	return &SerialPlainSesn{
		sx: sx,
		nd: nmp.NewNmpDispatcher(),
	}
}

func (sps *SerialPlainSesn) Open() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if sps.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	sps.isOpen = true
	return nil
}

func (sps *SerialPlainSesn) Close() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if !sps.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}
	sps.isOpen = false
	return nil
}

func (sps *SerialPlainSesn) IsOpen() bool {
	sps.m.Lock()
	defer sps.m.Unlock()

	return sps.isOpen
}

func (sps *SerialPlainSesn) MtuIn() int {
	return 1024
}

func (sps *SerialPlainSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return 512 * 3 / 4
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

func (sps *SerialPlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(m)
}

func (sps *SerialPlainSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	sps.m.Lock()
	defer sps.m.Unlock()

	if !sps.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed serial session")
	}

	nl, err := sps.addNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer sps.removeNmpListener(m.Hdr.Seq)

	reqb, err := sps.EncodeNmpMsg(m)
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

func (sps *SerialPlainSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("SerialPlainSesn.GetResourceOnce() unsupported")
}
