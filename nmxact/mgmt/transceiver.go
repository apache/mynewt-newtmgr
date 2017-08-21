package mgmt

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type TxFn func(req []byte) error

type Transceiver struct {
	// Only for plain NMP; nil for OMP transceivers.
	nd *nmp.Dispatcher

	// Used for OMP and CoAP resource requests.
	od *omp.Dispatcher

	isTcp bool
	wg    sync.WaitGroup
}

func NewTransceiver(isTcp bool, mgmtProto sesn.MgmtProto, logDepth int) (
	*Transceiver, error) {

	t := &Transceiver{
		isTcp: isTcp,
	}

	if mgmtProto == sesn.MGMT_PROTO_NMP {
		t.nd = nmp.NewDispatcher(logDepth)
	}

	od, err := omp.NewDispatcher(isTcp, logDepth)
	if err != nil {
		return nil, err
	}
	t.od = od

	return t, nil
}

func (t *Transceiver) txPlain(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	nl, err := t.nd.AddListener(req.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer t.nd.RemoveListener(req.Hdr.Seq)

	b, err := nmp.EncodeNmpPlain(req)
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx NMP request: %s", hex.Dump(b))

	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case <-nl.AfterTimeout(timeout):
			return nil, nmxutil.NewRspTimeoutError("NMP timeout")
		}
	}
}

func (t *Transceiver) txOmp(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	nl, err := t.od.AddNmpListener(req.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer t.od.RemoveNmpListener(req.Hdr.Seq)

	var b []byte
	if t.isTcp {
		b, err = omp.EncodeOmpTcp(req)
	} else {
		b, err = omp.EncodeOmpDgram(req)
	}
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx OMP request: %s", hex.Dump(b))

	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case <-nl.AfterTimeout(timeout):
			return nil, nmxutil.NewRspTimeoutError("NMP timeout")
		}
	}
}

func (t *Transceiver) TxNmp(txCb TxFn, req *nmp.NmpMsg, mtu int,
	timeout time.Duration) (nmp.NmpRsp, error) {

	if t.nd != nil {
		return t.txPlain(txCb, req, mtu, timeout)
	} else {
		return t.txOmp(txCb, req, mtu, timeout)
	}
}

func (t *Transceiver) TxOic(txCb TxFn, req coap.Message, mtu int,
	timeout time.Duration) (coap.Message, error) {

	b, err := oic.Encode(req)
	if err != nil {
		return nil, err
	}

	var rspExpected bool
	switch req.Type() {
	case coap.Confirmable:
		rspExpected = true
	case coap.NonConfirmable:
		rspExpected = req.Code() == coap.GET
	default:
		rspExpected = false
	}

	var ol *oic.Listener
	if rspExpected {
		ol, err = t.od.AddOicListener(req.Token())
		if err != nil {
			return nil, err
		}
		defer t.od.RemoveOicListener(req.Token())
	}

	log.Debugf("Tx OIC request: %s", hex.Dump(b))
	frags := nmxutil.Fragment(b, mtu)
	for _, frag := range frags {
		if err := txCb(frag); err != nil {
			return nil, err
		}
	}

	if !rspExpected {
		return nil, nil
	}

	for {
		select {
		case err := <-ol.ErrChan:
			return nil, err
		case rsp := <-ol.RspChan:
			return rsp, nil
		case <-ol.AfterTimeout(timeout):
			return nil, nmxutil.NewRspTimeoutError("OIC timeout")
		}
	}
}

func (t *Transceiver) DispatchNmpRsp(data []byte) {
	if t.nd != nil {
		t.nd.Dispatch(data)
	} else {
		t.od.Dispatch(data)
	}
}

func (t *Transceiver) DispatchCoap(data []byte) {
	t.od.Dispatch(data)
}

func (t *Transceiver) ErrorOne(seq uint8, err error) {
	if t.nd != nil {
		t.nd.ErrorOne(seq, err)
	} else {
		t.od.ErrorOneNmp(seq, err)
	}
}

func (t *Transceiver) ErrorAll(err error) {
	if t.nd != nil {
		t.nd.ErrorAll(err)
	}
	t.od.ErrorAll(err)
}

func (t *Transceiver) AbortRx(seq uint8) {
	t.ErrorOne(seq, fmt.Errorf("rx aborted"))
}

func (t *Transceiver) Stop() {
	t.od.Stop()
}
