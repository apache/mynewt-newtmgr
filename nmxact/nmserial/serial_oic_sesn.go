package nmserial

import (
	"fmt"
	"sync"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type SerialOicSesn struct {
	sx     *SerialXport
	rxer   *omp.Receiver
	isOpen bool

	// This mutex ensures:
	//     * each response get matched up with its corresponding request.
	//     * accesses to isOpen are protected.
	m sync.Mutex
}

func NewSerialOicSesn(sx *SerialXport) *SerialOicSesn {
	return &SerialOicSesn{
		sx:   sx,
		rxer: omp.NewReceiver(false),
	}
}

func (sos *SerialOicSesn) Open() error {
	sos.m.Lock()
	defer sos.m.Unlock()

	if sos.isOpen {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open serial session")
	}

	sos.isOpen = true
	return nil
}

func (sos *SerialOicSesn) Close() error {
	sos.m.Lock()
	defer sos.m.Unlock()

	if !sos.isOpen {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened serial session")
	}
	sos.isOpen = false
	return nil
}

func (sos *SerialOicSesn) IsOpen() bool {
	sos.m.Lock()
	defer sos.m.Unlock()

	return sos.isOpen
}

func (sos *SerialOicSesn) MtuIn() int {
	return 1024 - omp.OMP_MSG_OVERHEAD
}

func (sos *SerialOicSesn) MtuOut() int {
	// Mynewt commands have a default chunk buffer size of 512.  Account for
	// base64 encoding.
	return 512*3/4 - omp.OMP_MSG_OVERHEAD
}

func (sos *SerialOicSesn) AbortRx(seq uint8) error {
	sos.rxer.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (sos *SerialOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (sos *SerialOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	sos.m.Lock()
	defer sos.m.Unlock()

	if !sos.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed serial session")
	}

	nl, err := sos.rxer.AddNmpListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer sos.rxer.RemoveNmpListener(m.Hdr.Seq)

	reqb, err := sos.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if err := sos.sx.Tx(reqb); err != nil {
		return nil, err
	}

	for {
		rspb, err := sos.sx.Rx()
		if err != nil {
			return nil, err
		}

		// Now wait for newtmgr response.
		if sos.rxer.Rx(rspb) {
			select {
			case err := <-nl.ErrChan:
				return nil, err
			case rsp := <-nl.RspChan:
				return rsp, nil
			}
		}
	}
}

func (sos *SerialOicSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	token := nmxutil.NextOicToken()

	ol, err := sos.rxer.AddOicListener(token)
	if err != nil {
		return nil, err
	}
	defer sos.rxer.RemoveOicListener(token)

	req, err := oic.EncodeGet(uri, token)
	if err != nil {
		return nil, err
	}

	if err := sos.sx.Tx(req); err != nil {
		return nil, err
	}

	for {
		select {
		case err := <-ol.ErrChan:
			return nil, err
		case rsp := <-ol.RspChan:
			if rsp.Code != coap.Content {
				return nil, fmt.Errorf("UNEXPECTED OIC ACK: %#v", rsp)
			}
			return rsp.Payload, nil
		case <-ol.AfterTimeout(opt.Timeout):
			return nil, nmxutil.NewRspTimeoutError("OIC timeout")
		}
	}
}
