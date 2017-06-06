package nmserial

import (
	"fmt"
	"sync"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialOicSesn struct {
	sx     *SerialXport
	od     *omp.OmpDispatcher
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex
}

func NewSerialOicSesn(sx *SerialXport) *SerialOicSesn {
	return &SerialOicSesn{
		sx: sx,
		od: omp.NewOmpDispatcher(false),
	}
}

func (sps *SerialOicSesn) Open() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if sps.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	sps.isOpen = true
	return nil
}

func (sps *SerialOicSesn) Close() error {
	sps.m.Lock()
	defer sps.m.Unlock()

	if !sps.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}
	sps.isOpen = false
	return nil
}

func (sps *SerialOicSesn) IsOpen() bool {
	sps.m.Lock()
	defer sps.m.Unlock()

	return sps.isOpen
}

func (sps *SerialOicSesn) MtuIn() int {
	return 1024 - omp.OMP_MSG_OVERHEAD
}

func (sps *SerialOicSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return 512*3/4 - omp.OMP_MSG_OVERHEAD
}

func (sps *SerialOicSesn) AbortRx(seq uint8) error {
	return sps.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}

func (sps *SerialOicSesn) addNmpListener(seq uint8) (
	*nmp.NmpListener, error) {

	nl := nmp.NewNmpListener()
	if err := sps.od.AddListener(seq, nl); err != nil {
		return nil, err
	}

	return nl, nil
}

func (sps *SerialOicSesn) removeNmpListener(seq uint8) {
	sps.od.RemoveListener(seq)
}

func (sps *SerialOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (sps *SerialOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
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
		if sps.od.Dispatch(rspb) {
			select {
			case err := <-nl.ErrChan:
				return nil, err
			case rsp := <-nl.RspChan:
				return rsp, nil
			}
		}
	}
}
