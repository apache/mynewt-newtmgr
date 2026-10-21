/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package mgmt

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

const testTimeout = 5 * time.Second

// Long enough that a response produced synchronously by the fake device
// always wins, short enough to keep the timeout tests fast.
const shortTimeout = 50 * time.Millisecond

func cborEncode(v interface{}) ([]byte, error) {
	var b []byte
	enc := codec.NewEncoderBytes(&b, new(codec.CborHandle))
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b, nil
}

func cborDecodeMap(b []byte) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	dec := codec.NewDecoderBytes(b, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// fakeDevice is an in-memory management server used as the transceiver's
// TxFn.  It reassembles requests and, for echo requests, dispatches the
// matching response straight back into the transceiver.
type fakeDevice struct {
	txvr    *Transceiver
	isTcp   bool
	proto   sesn.MgmtProto
	respond bool
	txErr   error

	mtx    sync.Mutex
	frags  [][]byte
	nmpBuf []byte
	rxer   nmcoap.Receiver
	errs   []error
}

func newFakeDevice(t *testing.T, proto sesn.MgmtProto,
	isTcp bool) *fakeDevice {

	t.Helper()

	txvr, err := NewTransceiver(nil, nil, isTcp, proto, 0)
	if err != nil {
		t.Fatalf("NewTransceiver failed: %v", err)
	}
	t.Cleanup(txvr.Stop)

	return &fakeDevice{
		txvr:    txvr,
		isTcp:   isTcp,
		proto:   proto,
		respond: true,
		rxer:    nmcoap.NewReceiver(isTcp),
	}
}

func (f *fakeDevice) numFrags() int {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	return len(f.frags)
}

func (f *fakeDevice) checkErrs(t *testing.T) {
	t.Helper()

	f.mtx.Lock()
	defer f.mtx.Unlock()

	for _, err := range f.errs {
		t.Errorf("fake device: %v", err)
	}
}

func (f *fakeDevice) tx(b []byte) error {
	f.mtx.Lock()
	f.frags = append(f.frags, append([]byte(nil), b...))
	f.mtx.Unlock()

	if f.txErr != nil {
		return f.txErr
	}

	rsp, err := f.process(b)
	if err != nil {
		f.mtx.Lock()
		f.errs = append(f.errs, err)
		f.mtx.Unlock()
		return nil
	}

	if rsp != nil && f.respond {
		f.txvr.DispatchNmpRsp(rsp)
	}
	return nil
}

// Returns the encoded response once a complete request has been received.
func (f *fakeDevice) process(frag []byte) ([]byte, error) {
	if f.proto == sesn.MGMT_PROTO_NMP {
		return f.processNmp(frag)
	}
	return f.processOmp(frag)
}

func (f *fakeDevice) processNmp(frag []byte) ([]byte, error) {
	f.nmpBuf = append(f.nmpBuf, frag...)
	hdr, err := nmp.DecodeNmpHdr(f.nmpBuf)
	if err != nil {
		return nil, nil
	}
	if len(f.nmpBuf) < nmp.NMP_HDR_SIZE+int(hdr.Len) {
		return nil, nil
	}

	body, err := cborDecodeMap(f.nmpBuf[nmp.NMP_HDR_SIZE:])
	f.nmpBuf = nil
	if err != nil {
		return nil, err
	}

	rspHdr := echoRspHdr(hdr.Seq)
	return nmp.EncodeNmpPlain(&nmp.NmpMsg{
		Hdr:  rspHdr,
		Body: map[string]interface{}{"r": body["d"], "rc": 0},
	})
}

func (f *fakeDevice) processOmp(frag []byte) ([]byte, error) {
	m := f.rxer.Rx(frag)
	if m == nil {
		return nil, nil
	}

	if m.PathString() != strings.TrimPrefix(nmxutil.OmpRes, "/") {
		return nil, fmt.Errorf("unexpected OMP path %q", m.PathString())
	}

	body, err := cborDecodeMap(m.Payload())
	if err != nil {
		return nil, err
	}
	hb, ok := body["_h"].([]byte)
	if !ok {
		return nil, fmt.Errorf("OMP request without NMP header")
	}
	hdr, err := nmp.DecodeNmpHdr(hb)
	if err != nil {
		return nil, err
	}

	rspHdr := echoRspHdr(hdr.Seq)
	payload, err := cborEncode(map[string]interface{}{
		"_h": rspHdr.Bytes(),
		"r":  body["d"],
		"rc": 0,
	})
	if err != nil {
		return nil, err
	}

	mp := coap.MessageParams{
		Type:    coap.Acknowledgement,
		Code:    coap.Content,
		Token:   m.Token(),
		Payload: payload,
	}
	var rsp coap.Message
	if f.isTcp {
		rsp = coap.NewTcpMessage(mp)
	} else {
		rsp = coap.NewDgramMessage(mp)
	}
	return rsp.MarshalBinary()
}

func echoRspHdr(seq uint8) nmp.NmpHdr {
	return nmp.NmpHdr{
		Op:    nmp.NMP_OP_WRITE_RSP,
		Group: nmp.NMP_GROUP_DEFAULT,
		Id:    nmp.NMP_ID_DEF_ECHO,
		Seq:   seq,
	}
}

func echoReq(payload string) *nmp.NmpMsg {
	r := nmp.NewEchoReq()
	r.Payload = payload
	return r.Msg()
}

type txvrMode struct {
	name  string
	proto sesn.MgmtProto
	isTcp bool
}

var txvrModes = []txvrMode{
	{"nmp", sesn.MGMT_PROTO_NMP, false},
	{"nmp-tcp", sesn.MGMT_PROTO_NMP, true},
	{"omp", sesn.MGMT_PROTO_OMP, false},
	{"omp-tcp", sesn.MGMT_PROTO_OMP, true},
}

func checkEcho(t *testing.T, rsp nmp.NmpRsp, seq uint8, payload string) {
	t.Helper()

	echo, ok := rsp.(*nmp.EchoRsp)
	if !ok {
		t.Fatalf("got %T, want *nmp.EchoRsp", rsp)
	}
	if echo.Payload != payload {
		t.Errorf("payload = %q, want %q", echo.Payload, payload)
	}
	if echo.Hdr().Seq != seq {
		t.Errorf("seq = %d, want %d", echo.Hdr().Seq, seq)
	}
}

func TestTxRxMgmt(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)

			if got := f.txvr.MgmtProto(); got != mode.proto {
				t.Errorf("MgmtProto() = %v, want %v", got, mode.proto)
			}

			for i := 0; i < 2; i++ {
				req := echoReq("hello")

				rsp, err := f.txvr.TxRxMgmt(f.tx, req, 1024, testTimeout)
				if err != nil {
					t.Fatalf("TxRxMgmt failed: %v", err)
				}
				checkEcho(t, rsp, req.Hdr.Seq, "hello")
			}

			if n := f.numFrags(); n != 2 {
				t.Errorf("TxFn called %d times, want 2", n)
			}
			f.checkErrs(t)
		})
	}
}

func TestTxRxMgmtTimeout(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			f.respond = false

			_, err := f.txvr.TxRxMgmt(f.tx, echoReq("x"), 1024, shortTimeout)
			if !nmxutil.IsRspTimeout(err) {
				t.Fatalf("got error %v (%T), want RspTimeoutError", err, err)
			}
			f.checkErrs(t)
		})
	}
}

func TestTxRxMgmtTxError(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			f.txErr = errors.New("tx failed")

			req := echoReq("x")
			_, err := f.txvr.TxRxMgmt(f.tx, req, 1024, testTimeout)
			if err != f.txErr {
				t.Fatalf("got error %v, want %v", err, f.txErr)
			}

			// The transceiver is still usable.
			f.txErr = nil
			req = echoReq("y")
			rsp, err := f.txvr.TxRxMgmt(f.tx, req, 1024, testTimeout)
			if err != nil {
				t.Fatalf("TxRxMgmt after tx error failed: %v", err)
			}
			checkEcho(t, rsp, req.Hdr.Seq, "y")
		})
	}
}

func TestTxRxMgmtRequestTooBig(t *testing.T) {
	for _, mode := range txvrModes {
		if mode.isTcp {
			continue
		}
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)

			_, err := f.txvr.TxRxMgmt(f.tx, echoReq(strings.Repeat("x", 64)),
				16, testTimeout)
			if err == nil || !strings.Contains(err.Error(), "Request too big") {
				t.Fatalf("got error %v, want \"Request too big\"", err)
			}
			if n := f.numFrags(); n != 0 {
				t.Errorf("TxFn called %d times, want 0", n)
			}
		})
	}
}

func TestTxRxMgmtTcpFragments(t *testing.T) {
	for _, mode := range txvrModes {
		if !mode.isTcp {
			continue
		}
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			payload := strings.Repeat("y", 64)
			const mtu = 16

			rsp, err := f.txvr.TxRxMgmt(f.tx, echoReq(payload), mtu,
				testTimeout)
			if err != nil {
				t.Fatalf("TxRxMgmt failed: %v", err)
			}
			checkEcho(t, rsp, rsp.Hdr().Seq, payload)

			f.mtx.Lock()
			defer f.mtx.Unlock()
			if len(f.frags) < 2 {
				t.Errorf("request sent in %d fragments, want several",
					len(f.frags))
			}
			for i, frag := range f.frags {
				if len(frag) > mtu {
					t.Errorf("fragment %d is %d bytes, mtu %d",
						i, len(frag), mtu)
				}
			}
		})
	}
}

func TestTxRxMgmtAbortRx(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			f.respond = false

			req := echoReq("x")
			txFn := func(b []byte) error {
				f.txvr.AbortRx(req.Hdr.Seq)
				return nil
			}

			_, err := f.txvr.TxRxMgmt(txFn, req, 1024, testTimeout)
			if err == nil || !strings.Contains(err.Error(), "rx aborted") {
				t.Fatalf("got error %v, want \"rx aborted\"", err)
			}
		})
	}
}

func TestTxRxMgmtErrorAll(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			want := errors.New("closed")

			txFn := func(b []byte) error {
				f.txvr.ErrorAll(want)
				return nil
			}

			_, err := f.txvr.TxRxMgmt(txFn, echoReq("x"), 1024, testTimeout)
			if err != want {
				t.Fatalf("got error %v, want %v", err, want)
			}
		})
	}
}

func TestTxRxMgmtAsync(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)

			ch := make(chan nmp.NmpRsp, 1)
			errc := make(chan error, 1)
			req := echoReq("async")

			err := f.txvr.TxRxMgmtAsync(f.tx, req, 1024, testTimeout, ch, errc)
			if err != nil {
				t.Fatalf("TxRxMgmtAsync failed: %v", err)
			}

			select {
			case rsp := <-ch:
				checkEcho(t, rsp, req.Hdr.Seq, "async")
			case err := <-errc:
				t.Fatalf("unexpected error: %v", err)
			case <-time.After(testTimeout):
				t.Fatalf("timeout waiting for async response")
			}
			f.checkErrs(t)
		})
	}
}

// Regression test for d1f7072: an async request that times out must report
// the timeout on errc rather than panic with "send on closed channel".
func TestTxRxMgmtAsyncTimeout(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			f.respond = false

			ch := make(chan nmp.NmpRsp, 1)
			errc := make(chan error, 1)

			err := f.txvr.TxRxMgmtAsync(f.tx, echoReq("x"), 1024,
				time.Millisecond, ch, errc)
			if err != nil {
				t.Fatalf("TxRxMgmtAsync failed: %v", err)
			}

			select {
			case err := <-errc:
				if err == nil {
					t.Fatalf("got nil error")
				}
				if mode.proto == sesn.MGMT_PROTO_NMP &&
					!nmxutil.IsRspTimeout(err) {

					t.Errorf("got %v (%T), want RspTimeoutError", err, err)
				}
			case rsp := <-ch:
				t.Fatalf("unexpected response: %+v", rsp)
			case <-time.After(testTimeout):
				t.Fatalf("timeout waiting for async timeout error")
			}
		})
	}
}

func TestTxRxMgmtAsyncErrors(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)
			ch := make(chan nmp.NmpRsp, 1)
			errc := make(chan error, 1)

			// Request too big.
			if !mode.isTcp {
				err := f.txvr.TxRxMgmtAsync(f.tx,
					echoReq(strings.Repeat("x", 64)), 16, testTimeout,
					ch, errc)
				if err == nil {
					t.Errorf("oversized async request succeeded")
				}
			}

			// Tx failure.
			f.txErr = errors.New("tx failed")
			err := f.txvr.TxRxMgmtAsync(f.tx, echoReq("x"), 1024,
				testTimeout, ch, errc)
			if err != f.txErr {
				t.Errorf("got error %v, want %v", err, f.txErr)
			}
		})
	}
}

func TestListenCoapStripsLeadingSlash(t *testing.T) {
	for _, mode := range txvrModes {
		t.Run(mode.name, func(t *testing.T) {
			f := newFakeDevice(t, mode.proto, mode.isTcp)

			mc := nmcoap.MsgCriteria{Path: "/a/b"}
			l, err := f.txvr.ListenCoap(mc)
			if err != nil {
				t.Fatalf("ListenCoap failed: %v", err)
			}
			if l.Criteria.Path != "a/b" {
				t.Errorf("listener path = %q, want %q", l.Criteria.Path,
					"a/b")
			}

			// With or without the slash, it is the same listener.
			if _, err := f.txvr.ListenCoap(
				nmcoap.MsgCriteria{Path: "a/b"}); err == nil {

				t.Errorf("duplicate ListenCoap succeeded")
			}

			mp := coap.MessageParams{
				Type:    coap.NonConfirmable,
				Code:    coap.Content,
				Token:   []byte{1},
				Payload: []byte("notify"),
			}
			var m coap.Message
			if mode.isTcp {
				m = coap.NewTcpMessage(mp)
			} else {
				m = coap.NewDgramMessage(mp)
			}
			m.SetPathString("a/b")
			b, err := m.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			f.txvr.DispatchCoap(b)
			select {
			case got := <-l.RspChan:
				if string(got.Payload()) != "notify" {
					t.Errorf("payload = %q", got.Payload())
				}
			case <-time.After(testTimeout):
				t.Fatalf("timeout waiting for CoAP message")
			}

			f.txvr.StopListenCoap(mc)
			if _, ok := <-l.RspChan; ok {
				t.Errorf("listener not closed by StopListenCoap")
			}
		})
	}
}

// Regression test for 1e2b81b: TxCoap must hand the whole encoded message
// to the TxFn in one call; fragmentation is up to the transport.
func TestTxCoapNoFragmentation(t *testing.T) {
	for _, isTcp := range []bool{false, true} {
		t.Run(fmt.Sprintf("tcp=%v", isTcp), func(t *testing.T) {
			txvr, err := NewTransceiver(nil, nil, isTcp,
				sesn.MGMT_PROTO_OMP, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer txvr.Stop()

			m, err := nmcoap.CreateMsg(isTcp, nmcoap.MsgParams{
				Code:    coap.PUT,
				Uri:     "res",
				Payload: bytes.Repeat([]byte{0x5a}, 3000),
			})
			if err != nil {
				t.Fatal(err)
			}
			want, err := nmcoap.Encode(m)
			if err != nil {
				t.Fatal(err)
			}

			var calls [][]byte
			txFn := func(b []byte) error {
				calls = append(calls, b)
				return nil
			}

			if err := txvr.TxCoap(txFn, m); err != nil {
				t.Fatalf("TxCoap failed: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("TxFn called %d times, want 1", len(calls))
			}
			if !bytes.Equal(calls[0], want) {
				t.Errorf("TxFn got %d bytes, want the %d encoded bytes",
					len(calls[0]), len(want))
			}

			txErr := errors.New("tx failed")
			err = txvr.TxCoap(func([]byte) error { return txErr }, m)
			if err != txErr {
				t.Errorf("got error %v, want %v", err, txErr)
			}
		})
	}
}

func TestFilters(t *testing.T) {
	txvr, err := NewTransceiver(nil, nil, false, sesn.MGMT_PROTO_OMP, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer txvr.Stop()

	if tx, rx := txvr.Filters(); tx != nil || rx != nil {
		t.Errorf("Filters() = %v, %v; want nil, nil", tx, rx)
	}

	rxf := nmcoap.RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		return m, nil
	})
	txvr.SetFilters(nil, rxf)
	if _, rx := txvr.Filters(); rx == nil {
		t.Errorf("rx filter not set")
	}
}
