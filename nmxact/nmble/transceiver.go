package nmble

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

type Transceiver struct {
	connHandle uint16
	bx         *BleXport
	rxer       *Receiver
	nd         *nmp.Dispatcher
	od         *omp.Dispatcher

	wg         sync.WaitGroup
	nmpErrChan chan error
}

func NewTransceiver(id uint32, connHandle uint16, mgmtProto sesn.MgmtProto,
	bx *BleXport, rxer *Receiver, logDepth int) (*Transceiver, error) {

	t := &Transceiver{
		connHandle: connHandle,
		bx:         bx,
		rxer:       rxer,
	}

	switch mgmtProto {
	case sesn.MGMT_PROTO_NMP:
		t.nd = nmp.NewDispatcher(logDepth)

	case sesn.MGMT_PROTO_OMP:
		od, err := omp.NewDispatcher(true, logDepth)
		if err != nil {
			return nil, err
		}
		t.od = od

	default:
		return nil,
			fmt.Errorf("Invalid session management protocol: %#v", mgmtProto)
	}

	return t, nil
}

func (t *Transceiver) WriteCmd(attHandle uint16, payload []byte,
	name string) error {

	r := NewBleWriteCmdReq()
	r.ConnHandle = t.connHandle
	r.AttrHandle = int(attHandle)
	r.Data.Bytes = payload

	bl, err := t.rxer.AddListener(name, SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer t.rxer.RemoveListener(name, bl)

	if err := writeCmd(t.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (t *Transceiver) txPlain(attHandle uint16, req *nmp.NmpMsg,
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

	if err := t.WriteCmd(attHandle, b, "nmp-rsp"); err != nil {
		return nil, err
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

func (t *Transceiver) txOmp(attHandle uint16, req *nmp.NmpMsg,
	timeout time.Duration) (nmp.NmpRsp, error) {

	nl, err := t.od.AddNmpListener(req.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer t.od.RemoveNmpListener(req.Hdr.Seq)

	b, err := omp.EncodeOmpTcp(req)
	if err != nil {
		return nil, err
	}

	log.Debugf("Tx OMP request: %s", hex.Dump(b))

	if err := t.WriteCmd(attHandle, b, "omp-rsp"); err != nil {
		return nil, err
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

func (t *Transceiver) TxNmp(attHandle uint16, req *nmp.NmpMsg,
	timeout time.Duration) (nmp.NmpRsp, error) {

	if t.nd != nil {
		return t.txPlain(attHandle, req, timeout)
	} else {
		return t.txOmp(attHandle, req, timeout)
	}
}

func (t *Transceiver) TxOic(attHandle uint16, req coap.Message,
	timeout time.Duration) (coap.Message, error) {

	b, err := oic.Encode(req)
	if err != nil {
		return nil, err
	}

	// XXX: Only listen if message expects an ack.

	ol, err := t.od.AddOicListener(req.Token())
	if err != nil {
		return nil, err
	}
	defer t.od.RemoveOicListener(req.Token())

	log.Debugf("Tx OIC request: %s", hex.Dump(b))
	if err := t.WriteCmd(attHandle, b, "oic-rsp"); err != nil {
		return nil, err
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

func (t *Transceiver) NmpRspListen(attHandle uint16) (<-chan error, error) {
	if t.nmpErrChan != nil {
		return t.nmpErrChan, nil
	}

	key := TchKey(MSG_TYPE_NOTIFY_RX_EVT, int(t.connHandle))
	bl, err := t.rxer.AddListener("nmp-rsp", key)
	if err != nil {
		return nil, err
	}

	t.wg.Add(1)

	t.nmpErrChan = make(chan error)

	go func() {
		defer t.wg.Done()
		defer t.rxer.RemoveListener("nmp-rsp", bl)

		for {
			select {
			case err, ok := <-bl.ErrChan:
				if !ok {
					return
				}
				t.nmpErrChan <- err

			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleNotifyRxEvt:
					if msg.AttrHandle == int(attHandle) {
						if t.nd != nil {
							t.nd.Dispatch(msg.Data.Bytes)
						} else {
							t.od.Dispatch(msg.Data.Bytes)
						}
					} else {
						// XXX: Forward non-management notifications elsewhere.
					}
				}
			}
		}
	}()
	return t.nmpErrChan, nil
}

func (t *Transceiver) Stop(err error) {
	if t.nd != nil {
		t.nd.ErrorAll(err)
	} else {
		t.od.ErrorAll(err)
		t.od.Stop()
	}

	t.wg.Wait()

	ch := t.nmpErrChan
	if ch != nil {
		close(ch)
	}
}

func (t *Transceiver) AbortNmpRx(seq uint8) error {
	return t.nd.ErrorOne(seq, fmt.Errorf("Rx aborted"))
}

func (t *Transceiver) AbortOmpRx(seq uint8) error {
	return t.od.ErrorOneNmp(seq, fmt.Errorf("Rx aborted"))
}
