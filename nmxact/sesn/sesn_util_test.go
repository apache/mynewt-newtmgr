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

package sesn

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// stubSesn is a minimal scripted session.  Each management transaction
// consumes the next error from errs; once errs is exhausted, transactions
// succeed.
type stubSesn struct {
	errs     []error
	calls    int
	timeouts []time.Duration

	isTcp   bool
	coapTx  []coap.Message
	txErr   error
	listenc []nmcoap.MsgCriteria
	stopped []nmcoap.MsgCriteria
	lner    *nmcoap.Listener

	// Called for each transmitted CoAP message.
	onTxCoap func(m coap.Message)
}

func (s *stubSesn) nextErr(timeout time.Duration) error {
	s.calls++
	s.timeouts = append(s.timeouts, timeout)
	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

func (s *stubSesn) Open() error                { return nil }
func (s *stubSesn) Close() error               { return nil }
func (s *stubSesn) IsOpen() bool               { return true }
func (s *stubSesn) MtuIn() int                 { return 512 }
func (s *stubSesn) MtuOut() int                { return 512 }
func (s *stubSesn) MgmtProto() MgmtProto       { return MGMT_PROTO_NMP }
func (s *stubSesn) CoapIsTcp() bool            { return s.isTcp }
func (s *stubSesn) AbortRx(nmpSeq uint8) error { return nil }

func (s *stubSesn) RxAccept() (Sesn, *SesnCfg, error) {
	return nil, nil, fmt.Errorf("not supported")
}

func (s *stubSesn) RxCoap(opt TxOptions) (coap.Message, error) {
	return nil, fmt.Errorf("not supported")
}

func (s *stubSesn) TxRxMgmt(m *nmp.NmpMsg,
	timeout time.Duration) (nmp.NmpRsp, error) {

	if err := s.nextErr(timeout); err != nil {
		return nil, err
	}
	return nmp.NewEchoRsp(), nil
}

func (s *stubSesn) TxRxMgmtAsync(m *nmp.NmpMsg, timeout time.Duration,
	ch chan nmp.NmpRsp, errc chan error) error {

	return s.nextErr(timeout)
}

func (s *stubSesn) ListenCoap(
	mc nmcoap.MsgCriteria) (*nmcoap.Listener, error) {

	s.listenc = append(s.listenc, mc)
	s.lner = nmcoap.NewListener(mc)
	return s.lner, nil
}

func (s *stubSesn) StopListenCoap(mc nmcoap.MsgCriteria) {
	s.stopped = append(s.stopped, mc)
}

func (s *stubSesn) TxCoap(m coap.Message) error {
	s.coapTx = append(s.coapTx, m)
	if s.txErr != nil {
		return s.txErr
	}
	if s.onTxCoap != nil {
		s.onTxCoap(m)
	}
	return nil
}

func (s *stubSesn) Filters() (nmcoap.TxMsgFilter, nmcoap.RxMsgFilter) {
	return nil, nil
}

func (s *stubSesn) SetFilters(txFilter nmcoap.TxMsgFilter,
	rxFilter nmcoap.RxMsgFilter) {
}

func tmoErr() error {
	return nmxutil.NewRspTimeoutError("timeout")
}

var retryTests = []struct {
	name      string
	tries     int
	errs      []error
	wantCalls int
	wantErr   bool
}{
	{"success", 1, nil, 1, false},
	{"zero tries means one attempt", 0, []error{tmoErr()}, 1, true},
	{"one try", 1, []error{tmoErr()}, 1, true},
	{"retry after timeout", 3, []error{tmoErr(), tmoErr()}, 3, false},
	{"retries exhausted", 2, []error{tmoErr(), tmoErr(), tmoErr()}, 2,
		true},
	{"other error not retried", 3, []error{fmt.Errorf("boom")}, 1, true},
	{"other error after timeout", 3,
		[]error{tmoErr(), fmt.Errorf("boom")}, 2, true},
	{"closed not retried", 3,
		[]error{nmxutil.NewSesnClosedError("closed")}, 1, true},
}

func TestTxRxMgmtRetry(t *testing.T) {
	for _, tt := range retryTests {
		s := &stubSesn{errs: append([]error(nil), tt.errs...)}
		opt := TxOptions{Timeout: 7 * time.Second, Tries: tt.tries}

		rsp, err := TxRxMgmt(s, nmp.NewEchoReq().Msg(), opt)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err %v", tt.name, err)
		}
		if (rsp == nil) != tt.wantErr {
			t.Errorf("%s: rsp %v", tt.name, rsp)
		}
		if s.calls != tt.wantCalls {
			t.Errorf("%s: %d calls, want %d", tt.name, s.calls,
				tt.wantCalls)
		}
		for _, tmo := range s.timeouts {
			if tmo != opt.Timeout {
				t.Errorf("%s: timeout %v", tt.name, tmo)
			}
		}
	}
}

func TestTxRxMgmtAsyncRetry(t *testing.T) {
	for _, tt := range retryTests {
		s := &stubSesn{errs: append([]error(nil), tt.errs...)}
		opt := TxOptions{Timeout: 7 * time.Second, Tries: tt.tries}

		err := TxRxMgmtAsync(s, nmp.NewEchoReq().Msg(), opt, nil, nil)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err %v", tt.name, err)
		}
		if s.calls != tt.wantCalls {
			t.Errorf("%s: %d calls, want %d", tt.name, s.calls,
				tt.wantCalls)
		}
	}
}

func TestTxOptions(t *testing.T) {
	opt := NewTxOptions()
	if opt != DfltTxOptions || opt.Tries != 1 || opt.Timeout <= 0 {
		t.Errorf("default options %+v", opt)
	}

	opt.Timeout = 0
	if opt.AfterTimeout() != nil {
		t.Errorf("zero timeout yields a timer")
	}

	opt.Timeout = time.Millisecond
	select {
	case <-opt.AfterTimeout():
	case <-time.After(5 * time.Second):
		t.Fatal("timeout did not fire")
	}
}

func TestMgmtProtoString(t *testing.T) {
	tests := map[MgmtProto]string{
		MGMT_PROTO_NMP:         "nmp",
		MGMT_PROTO_OMP:         "omp",
		MGMT_PROTO_COAP_SERVER: "coapserver",
	}
	for p, want := range tests {
		if p.String() != want {
			t.Errorf("%d: %q, want %q", p, p.String(), want)
		}
	}
}

func TestTxCoap(t *testing.T) {
	for _, isTcp := range []bool{false, true} {
		s := &stubSesn{isTcp: isTcp}
		mp := nmcoap.MsgParams{
			Code:    coap.PUT,
			Uri:     "/x/y",
			Token:   []byte{9, 8},
			Payload: []byte{1, 2, 3},
		}
		if err := TxCoap(s, mp); err != nil {
			t.Fatalf("tcp %v: %v", isTcp, err)
		}
		if len(s.coapTx) != 1 {
			t.Fatalf("tcp %v: %d messages", isTcp, len(s.coapTx))
		}
		m := s.coapTx[0]
		_, isTcpMsg := m.(*coap.TcpMessage)
		if isTcpMsg != isTcp {
			t.Errorf("tcp %v: message type %T", isTcp, m)
		}
		if m.Code() != coap.PUT || m.PathString() != "x/y" ||
			!bytes.Equal(m.Token(), mp.Token) ||
			!bytes.Equal(m.Payload(), mp.Payload) {

			t.Errorf("tcp %v: wrong message", isTcp)
		}
	}

	// A missing token is generated.
	get := nmcoap.MsgParams{Code: coap.GET, Uri: "/r"}
	s := &stubSesn{}
	if err := TxCoap(s, get); err != nil {
		t.Fatal(err)
	}
	if len(s.coapTx[0].Token()) == 0 {
		t.Errorf("no token generated")
	}

	s = &stubSesn{txErr: fmt.Errorf("tx failed")}
	if err := TxCoap(s, get); err == nil {
		t.Errorf("tx error not reported")
	}
}

func TestRxCoap(t *testing.T) {
	msg := coap.NewDgramMessage(coap.MessageParams{Code: coap.Content})
	rxErr := fmt.Errorf("rx failed")

	for _, tmo := range []time.Duration{0, 5 * time.Second} {
		l := nmcoap.NewListener(nmcoap.MsgCriteria{})
		l.RspChan <- msg
		rsp, err := RxCoap(l, tmo)
		if err != nil || rsp != msg {
			t.Errorf("tmo %v: rsp %v err %v", tmo, rsp, err)
		}

		l = nmcoap.NewListener(nmcoap.MsgCriteria{})
		l.ErrChan <- rxErr
		rsp, err = RxCoap(l, tmo)
		if err != rxErr || rsp != nil {
			t.Errorf("tmo %v: rsp %v err %v", tmo, rsp, err)
		}
	}

	l := nmcoap.NewListener(nmcoap.MsgCriteria{})
	rsp, err := RxCoap(l, time.Millisecond)
	if !nmxutil.IsRspTimeout(err) || rsp != nil {
		t.Errorf("timeout: rsp %v err %v", rsp, err)
	}
}

func TestTxRxCoap(t *testing.T) {
	tests := []struct {
		name    string
		tries   int
		drops   int
		wantTx  int
		wantErr bool
	}{
		{"reply", 1, 0, 1, false},
		{"no reply", 1, 1, 1, true},
		{"zero tries", 0, 1, 1, true},
		{"retry", 3, 2, 3, false},
		{"retries exhausted", 2, 5, 2, true},
	}

	for _, tt := range tests {
		s := &stubSesn{}
		drops := tt.drops
		s.onTxCoap = func(m coap.Message) {
			if drops > 0 {
				drops--
				return
			}
			s.lner.RspChan <- coap.NewDgramMessage(coap.MessageParams{
				Code:  coap.Content,
				Token: m.Token(),
			})
		}

		mp := nmcoap.MsgParams{Code: coap.GET, Uri: "/r", Token: []byte{1}}
		opt := TxOptions{Timeout: 10 * time.Millisecond, Tries: tt.tries}

		rsp, err := TxRxCoap(s, mp, opt)
		if tt.wantErr {
			if !nmxutil.IsRspTimeout(err) || rsp != nil {
				t.Errorf("%s: rsp %v err %v", tt.name, rsp, err)
			}
		} else if err != nil || rsp == nil || rsp.Code() != coap.Content {
			t.Errorf("%s: rsp %v err %v", tt.name, rsp, err)
		}

		if len(s.coapTx) != tt.wantTx {
			t.Errorf("%s: %d messages sent, want %d", tt.name,
				len(s.coapTx), tt.wantTx)
		}
		if len(s.listenc) != 1 || len(s.stopped) != 1 ||
			!bytes.Equal(s.listenc[0].Token, mp.Token) {

			t.Errorf("%s: listen %v stop %v", tt.name, s.listenc,
				s.stopped)
		}
	}

	// A transmit error ends the transaction without retries.
	s := &stubSesn{txErr: fmt.Errorf("tx failed")}
	_, err := TxRxCoap(s, nmcoap.MsgParams{Code: coap.GET, Uri: "/r"},
		TxOptions{Timeout: time.Second, Tries: 3})
	if err == nil || len(s.coapTx) != 1 || len(s.stopped) != 1 {
		t.Errorf("tx error: err %v, %d sent", err, len(s.coapTx))
	}
}
