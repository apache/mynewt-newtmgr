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

package omp

import (
	"bytes"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

func cborMap(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()

	m := map[string]interface{}{}
	if err := codec.NewDecoderBytes(b, new(codec.CborHandle)).
		Decode(&m); err != nil {

		t.Fatalf("failed to decode CBOR: %v", err)
	}
	return m
}

func cborBytes(t *testing.T, v interface{}) []byte {
	t.Helper()

	b := []byte{}
	if err := codec.NewEncoderBytes(&b, new(codec.CborHandle)).
		Encode(v); err != nil {

		t.Fatalf("failed to encode CBOR: %v", err)
	}
	return b
}

func keys(m map[string]interface{}) []string {
	ks := []string{}
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// checkOmpReq verifies the CoAP envelope of an encoded OMP request and
// returns the decoded CBOR payload.
func checkOmpReq(t *testing.T, m coap.Message,
	hdr nmp.NmpHdr) map[string]interface{} {

	t.Helper()

	if m.Code() != coap.PUT {
		t.Fatalf("code = %v, want PUT", m.Code())
	}
	if m.PathString() != "omgr" {
		t.Fatalf("path = %q, want omgr", m.PathString())
	}
	if !bytes.Equal(m.Token(), []byte{hdr.Seq}) {
		t.Fatalf("token = % x, want %02x", m.Token(), hdr.Seq)
	}

	pl := cborMap(t, m.Payload())
	h, ok := pl["_h"].([]byte)
	if !ok {
		t.Fatalf("_h missing or wrong type: %#v", pl["_h"])
	}
	if !bytes.Equal(h, hdr.Bytes()) {
		t.Fatalf("_h = % x, want % x", h, hdr.Bytes())
	}

	return pl
}

func TestEncodeOmpDgram(t *testing.T) {
	req := nmp.NewEchoReq()
	req.Payload = "hello"
	msg := req.Msg()

	data, err := EncodeOmpDgram(nil, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, err := coap.ParseDgramMessage(data)
	if err != nil {
		t.Fatalf("failed to parse CoAP: %v", err)
	}
	if m.Type() != coap.Confirmable {
		t.Fatalf("type = %v, want Confirmable", m.Type())
	}

	pl := checkOmpReq(t, m, msg.Hdr)
	if want := []string{"_h", "d"}; !reflect.DeepEqual(keys(pl), want) {
		t.Fatalf("payload keys = %v, want %v", keys(pl), want)
	}
	if pl["d"] != "hello" {
		t.Fatalf("d = %v, want hello", pl["d"])
	}
}

func TestEncodeOmpTcp(t *testing.T) {
	req := nmp.NewEchoReq()
	req.Payload = "hello"
	msg := req.Msg()

	data, err := EncodeOmpTcp(nil, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, rest, err := coap.PullTcp(data)
	if err != nil {
		t.Fatalf("failed to parse CoAP: %v", err)
	}
	if m == nil || len(rest) != 0 {
		t.Fatalf("PullTcp() = %v, rest % x", m, rest)
	}

	pl := checkOmpReq(t, m, msg.Hdr)
	if want := []string{"_h", "d"}; !reflect.DeepEqual(keys(pl), want) {
		t.Fatalf("payload keys = %v, want %v", keys(pl), want)
	}
}

// The OMP encoder builds a field map from the request struct; it must
// honour "omitempty" and must not include the NmpBase header.  Regression
// test for 'nmxact: Fix "omitempty" handling in NMP requests'.
func TestEncodeOmpOmitEmpty(t *testing.T) {
	tests := []struct {
		name string
		req  nmp.NmpReq
		keys []string
	}{
		{
			name: "image upload first chunk",
			req: func() nmp.NmpReq {
				r := nmp.NewImageUploadReq()
				r.Len = 100
				r.DataSha = []byte{1, 2}
				r.Upgrade = true
				r.Data = []byte{0xaa}
				return r
			}(),
			keys: []string{"_h", "data", "image", "len", "off", "sha",
				"upgrade"},
		},
		{
			name: "image upload next chunk",
			req: func() nmp.NmpReq {
				r := nmp.NewImageUploadReq()
				r.Off = 50
				r.Data = []byte{0xbb}
				return r
			}(),
			keys: []string{"_h", "data", "image", "off"},
		},
		{
			name: "config write save only",
			req: func() nmp.NmpReq {
				r := nmp.NewConfigWriteReq()
				r.Save = true
				return r
			}(),
			keys: []string{"_h", "save"},
		},
		{
			name: "config write name val",
			req: func() nmp.NmpReq {
				r := nmp.NewConfigWriteReq()
				r.Name = "a/b"
				r.Val = "1"
				return r
			}(),
			keys: []string{"_h", "name", "val"},
		},
		{
			name: "fs upload",
			req: func() nmp.NmpReq {
				r := nmp.NewFsUploadReq()
				r.Name = "/f"
				r.Off = 10
				r.Data = []byte{1}
				return r
			}(),
			keys: []string{"_h", "data", "len", "name", "off"},
		},
		{
			name: "reset",
			req:  nmp.NewResetReq(),
			keys: []string{"_h"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.req.Msg()
			data, err := EncodeOmpDgram(nil, msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m, err := coap.ParseDgramMessage(data)
			if err != nil {
				t.Fatalf("failed to parse CoAP: %v", err)
			}

			pl := checkOmpReq(t, m, msg.Hdr)
			if !reflect.DeepEqual(keys(pl), tt.keys) {
				t.Fatalf("payload keys = %v, want %v", keys(pl), tt.keys)
			}
		})
	}
}

type testTxFilter struct {
	called int
	err    error
}

func (f *testTxFilter) Filter(m coap.Message) (coap.Message, error) {
	f.called++
	if f.err != nil {
		return nil, f.err
	}
	m.SetURIQuery("filtered")
	return m, nil
}

func (f *testTxFilter) Freeze()   {}
func (f *testTxFilter) Unfreeze() {}

func TestEncodeOmpTxFilter(t *testing.T) {
	f := &testTxFilter{}
	data, err := EncodeOmpDgram(f, nmp.NewResetReq().Msg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.called != 1 {
		t.Fatalf("filter called %d times, want 1", f.called)
	}

	m, err := coap.ParseDgramMessage(data)
	if err != nil {
		t.Fatalf("failed to parse CoAP: %v", err)
	}
	if q := m.Option(coap.URIQuery); q != "filtered" {
		t.Fatalf("query = %v, want filtered", q)
	}

	f = &testTxFilter{err: errors.New("boom")}
	if _, err := EncodeOmpTcp(f, nmp.NewResetReq().Msg()); err == nil {
		t.Fatalf("expected filter error")
	}
}

func echoRspMsg(t *testing.T, code coap.COAPCode, withHdr bool,
	seq uint8) coap.Message {

	t.Helper()

	hdr := nmp.NmpHdr{
		Op:    nmp.NMP_OP_WRITE_RSP,
		Group: nmp.NMP_GROUP_DEFAULT,
		Id:    nmp.NMP_ID_DEF_ECHO,
		Seq:   seq,
	}

	pl := map[string]interface{}{
		"r":  "pong",
		"rc": 0,
	}
	if withHdr {
		pl["_h"] = hdr.Bytes()
	}

	return coap.NewDgramMessage(coap.MessageParams{
		Type:    coap.Acknowledgement,
		Code:    code,
		Token:   []byte{seq},
		Payload: cborBytes(t, pl),
	})
}

func TestDecodeOmp(t *testing.T) {
	codes := []coap.COAPCode{
		coap.Created, coap.Deleted, coap.Valid, coap.Changed, coap.Content,
	}

	for _, c := range codes {
		rsp, err := DecodeOmp(echoRspMsg(t, c, true, 0x21), nil)
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", c, err)
		}

		er, ok := rsp.(*nmp.EchoRsp)
		if !ok {
			t.Fatalf("%v: got %T, want *nmp.EchoRsp", c, rsp)
		}
		if er.Payload != "pong" || er.Hdr().Seq != 0x21 {
			t.Fatalf("%v: unexpected rsp %+v hdr %+v", c, er, *er.Hdr())
		}
	}
}

func TestDecodeOmpIgnoresRequests(t *testing.T) {
	for _, c := range []coap.COAPCode{coap.GET, coap.PUT, coap.POST,
		coap.DELETE} {

		rsp, err := DecodeOmp(echoRspMsg(t, c, true, 1), nil)
		if rsp != nil || err != nil {
			t.Errorf("%v: DecodeOmp() = (%v, %v), want (nil, nil)",
				c, rsp, err)
		}
	}
}

func TestDecodeOmpErrors(t *testing.T) {
	tests := []struct {
		name string
		msg  coap.Message
	}{
		{"error code", echoRspMsg(t, coap.NotFound, true, 1)},
		{"internal error code", echoRspMsg(t, coap.InternalServerError,
			true, 1)},
		{"missing _h", echoRspMsg(t, coap.Changed, false, 1)},
		{"bad cbor", coap.NewDgramMessage(coap.MessageParams{
			Code:    coap.Changed,
			Payload: []byte{0xa1, 0x62},
		})},
		{"short _h", coap.NewDgramMessage(coap.MessageParams{
			Code: coap.Changed,
			Payload: cborBytes(t, map[string]interface{}{
				"_h": []byte{1, 2, 3},
			}),
		})},
		{"unknown nmp id", coap.NewDgramMessage(coap.MessageParams{
			Code: coap.Changed,
			Payload: cborBytes(t, map[string]interface{}{
				"_h": (&nmp.NmpHdr{Op: nmp.NMP_OP_WRITE_RSP,
					Group: 99, Id: 99}).Bytes(),
			}),
		})},
	}

	for _, tt := range tests {
		rsp, err := DecodeOmp(tt.msg, nil)
		if err == nil {
			t.Errorf("%s: expected error, got %+v", tt.name, rsp)
		}
	}
}

func TestDecodeOmpRxFilter(t *testing.T) {
	// Filter is applied before the code check.
	var called bool
	f := nmcoap.RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		called = true
		return echoRspMsg(t, coap.Changed, true, 5), nil
	})

	rsp, err := DecodeOmp(echoRspMsg(t, coap.NotFound, false, 5), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called || rsp == nil || rsp.Hdr().Seq != 5 {
		t.Fatalf("unexpected rsp %+v (filter called=%v)", rsp, called)
	}

	// Filter error is propagated.
	f = nmcoap.RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		return nil, errors.New("boom")
	})
	if _, err := DecodeOmp(echoRspMsg(t, coap.Changed, true, 5),
		f); err == nil {

		t.Fatalf("expected filter error")
	}

	// Requests are ignored before the filter runs.
	called = false
	f = nmcoap.RxFilterFunc(func(m coap.Message) (coap.Message, error) {
		called = true
		return m, nil
	})
	if rsp, err := DecodeOmp(echoRspMsg(t, coap.PUT, true, 5),
		f); rsp != nil || err != nil || called {

		t.Fatalf("request: rsp=%v err=%v called=%v", rsp, err, called)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	// A device answering an OMP request echoes the request header with the
	// response op.
	req := nmp.NewEchoReq()
	req.Payload = "x"
	msg := req.Msg()

	data, err := EncodeOmpTcp(nil, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, _, err := coap.PullTcp(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pl := cborMap(t, m.Payload())
	hdr, err := nmp.DecodeNmpHdr(pl["_h"].([]byte))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hdr.Op = nmp.NMP_OP_WRITE_RSP

	rm := coap.NewTcpMessage(coap.MessageParams{
		Code:  coap.Changed,
		Token: m.Token(),
		Payload: cborBytes(t, map[string]interface{}{
			"_h": hdr.Bytes(),
			"r":  pl["d"],
		}),
	})

	rsp, err := DecodeOmp(rm, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if er := rsp.(*nmp.EchoRsp); er.Payload != "x" ||
		er.Hdr().Seq != msg.Hdr.Seq {

		t.Fatalf("unexpected rsp %+v", er)
	}
}
