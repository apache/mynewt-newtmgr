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

package nmp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDecodeRspBodyAllTypes(t *testing.T) {
	body, err := BodyBytes(map[string]interface{}{"rc": 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for ogi, ctor := range rspCtorMap {
		hdr := &NmpHdr{
			Op:    ogi.Op,
			Group: ogi.Group,
			Id:    ogi.Id,
			Seq:   0x5a,
			Len:   uint16(len(body)),
		}

		rsp, err := DecodeRspBody(hdr, body)
		if err != nil {
			t.Errorf("%+v: unexpected error: %v", ogi, err)
			continue
		}

		if reflect.TypeOf(rsp) != reflect.TypeOf(ctor()) {
			t.Errorf("%+v: got type %T, want %T", ogi, rsp, ctor())
		}
		if *rsp.Hdr() != *hdr {
			t.Errorf("%+v: hdr = %+v, want %+v", ogi, *rsp.Hdr(), *hdr)
		}

		// Every response that has an rc field must have it decoded.
		f := reflect.ValueOf(rsp).Elem().FieldByName("Rc")
		if f.IsValid() && f.Int() != 7 {
			t.Errorf("%+v: Rc = %d, want 7", ogi, f.Int())
		}
	}
}

func TestDecodeMcumgrParamsRsp(t *testing.T) {
	body, err := BodyBytes(map[string]interface{}{
		"buf_size":  1024,
		"buf_count": 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hdr := &NmpHdr{
		Op:    NMP_OP_READ_RSP,
		Group: NMP_GROUP_DEFAULT,
		Id:    NMP_ID_DEF_PARAMS,
		Len:   uint16(len(body)),
	}

	rsp, err := DecodeRspBody(hdr, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prsp := rsp.(*McumgrParamsRsp)
	if prsp.Rc != 0 || prsp.BufSize != 1024 || prsp.BufCount != 2 {
		t.Errorf("unexpected response: %+v", prsp)
	}
}

func TestDecodeRspBodyRoundTrip(t *testing.T) {
	src := NewImageStateRsp()
	src.Rc = 0
	src.SplitStatus = MATCHING
	src.Images = []ImageStateEntry{
		{
			Image:     0,
			Slot:      1,
			Version:   "1.2.3",
			Hash:      []byte{0xde, 0xad},
			Bootable:  true,
			Confirmed: true,
		},
	}

	body, err := BodyBytes(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hdr := &NmpHdr{
		Op:    NMP_OP_READ_RSP,
		Group: NMP_GROUP_IMAGE,
		Id:    NMP_ID_IMAGE_STATE,
	}
	rsp, err := DecodeRspBody(hdr, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := rsp.(*ImageStateRsp)
	if !ok {
		t.Fatalf("got type %T, want *ImageStateRsp", rsp)
	}
	if got.SplitStatus != MATCHING || len(got.Images) != 1 {
		t.Fatalf("unexpected rsp %+v", got)
	}
	e := got.Images[0]
	if e.Slot != 1 || e.Version != "1.2.3" ||
		!bytes.Equal(e.Hash, []byte{0xde, 0xad}) ||
		!e.Bootable || !e.Confirmed || e.Active {

		t.Fatalf("unexpected image entry %+v", e)
	}
}

func TestDecodeRspBodyUnknown(t *testing.T) {
	body, _ := BodyBytes(map[string]interface{}{"rc": 0})

	hdrs := []NmpHdr{
		{Op: NMP_OP_READ_RSP, Group: NMP_GROUP_PERUSER, Id: 0},
		{Op: NMP_OP_READ_RSP, Group: NMP_GROUP_DEFAULT, Id: 0xff},
		// Echo is only defined for write.
		{Op: NMP_OP_READ_RSP, Group: NMP_GROUP_DEFAULT, Id: NMP_ID_DEF_ECHO},
		// Requests never decode as responses.
		{Op: NMP_OP_WRITE, Group: NMP_GROUP_DEFAULT, Id: NMP_ID_DEF_ECHO},
	}

	for _, h := range hdrs {
		h := h
		rsp, err := DecodeRspBody(&h, body)
		if err == nil {
			t.Errorf("%+v: expected error, got %+v", h, rsp)
		}
		if rsp != nil {
			t.Errorf("%+v: expected nil rsp, got %+v", h, rsp)
		}
	}
}

func TestDecodeRspBodyInvalidCbor(t *testing.T) {
	hdr := &NmpHdr{
		Op:    NMP_OP_WRITE_RSP,
		Group: NMP_GROUP_DEFAULT,
		Id:    NMP_ID_DEF_ECHO,
	}

	bodies := [][]byte{
		// Truncated map.
		{0xa1, 0x62, 'r', 'c'},
		// Not a map.
		{0x18},
	}
	for _, b := range bodies {
		if rsp, err := DecodeRspBody(hdr, b); err == nil {
			t.Errorf("% x: expected error, got %+v", b, rsp)
		}
	}
}

func TestDecodeRsp(t *testing.T) {
	rsp := NewEchoRsp()
	rsp.Payload = "hi"
	body, err := BodyBytes(rsp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hdr := NmpHdr{
		Op:    NMP_OP_WRITE_RSP,
		Group: NMP_GROUP_DEFAULT,
		Id:    NMP_ID_DEF_ECHO,
		Seq:   9,
		Len:   uint16(len(body)),
	}
	pkt := append(hdr.Bytes(), body...)

	got, err := decodeRsp(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	er, ok := got.(*EchoRsp)
	if !ok {
		t.Fatalf("got type %T, want *EchoRsp", got)
	}
	if er.Payload != "hi" || er.Hdr().Seq != 9 {
		t.Fatalf("unexpected rsp %+v hdr %+v", er, *er.Hdr())
	}
}

// Devices that echo received requests (e.g., over serial) must not cause a
// crash or an error; requests are silently ignored.  Regression test for
// "nmxact; fix crash when sent requests are being echoed".
func TestDecodeRspIgnoresRequests(t *testing.T) {
	for _, op := range []uint8{NMP_OP_READ, NMP_OP_WRITE} {
		req := NewEchoReq()
		req.Payload = "echoed"
		msg := req.Msg()
		msg.Hdr.Op = op

		pkt, err := EncodeNmpPlain(msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rsp, err := decodeRsp(pkt)
		if err != nil || rsp != nil {
			t.Fatalf("op=%d: decodeRsp() = (%v, %v), want (nil, nil)",
				op, rsp, err)
		}
	}
}

func TestDecodeRspShort(t *testing.T) {
	if rsp, err := decodeRsp([]byte{1, 2, 3}); err == nil {
		t.Fatalf("expected error, got %+v", rsp)
	}
}

func TestRegisterResponseHandler(t *testing.T) {
	ogi := Ogi{NMP_OP_WRITE_RSP, NMP_GROUP_PERUSER + 1, 0x77}
	defer delete(rspCtorMap, ogi)

	ctor := func() NmpRsp { return NewEchoRsp() }
	RegisterResponseHandler(ogi, ctor)

	body, _ := BodyBytes(map[string]interface{}{"r": "x"})
	hdr := &NmpHdr{Op: ogi.Op, Group: ogi.Group, Id: ogi.Id}
	rsp, err := DecodeRspBody(hdr, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if er, ok := rsp.(*EchoRsp); !ok || er.Payload != "x" {
		t.Fatalf("unexpected rsp %#v", rsp)
	}

	// An existing handler is not replaced.
	RegisterResponseHandler(ogi,
		func() NmpRsp { return NewResetRsp() })
	rsp, err = DecodeRspBody(hdr, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := rsp.(*EchoRsp); !ok {
		t.Fatalf("handler was replaced; got %T", rsp)
	}
}
