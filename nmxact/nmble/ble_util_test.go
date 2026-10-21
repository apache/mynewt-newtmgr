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

package nmble

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func TestNextSeqWrap(t *testing.T) {
	seqMtx.Lock()
	saved := nextSeq
	nextSeq = BLE_SEQ_EVT_MIN - 2
	seqMtx.Unlock()

	defer func() {
		seqMtx.Lock()
		nextSeq = saved
		seqMtx.Unlock()
	}()

	// Sequence numbers never enter the event range.
	want := []BleSeq{BLE_SEQ_EVT_MIN - 2, BLE_SEQ_EVT_MIN - 1, BLE_SEQ_MIN,
		BLE_SEQ_MIN + 1}
	for _, w := range want {
		if got := NextSeq(); got != w {
			t.Fatalf("NextSeq() = %#x, want %#x", got, w)
		}
	}
}

func TestBleUtilNewReqs(t *testing.T) {
	tests := []struct {
		req interface{}
		typ MsgType
	}{
		{NewBleConnectReq(), MSG_TYPE_CONNECT},
		{NewBleTerminateReq(), MSG_TYPE_TERMINATE},
		{NewBleConnCancelReq(), MSG_TYPE_CONN_CANCEL},
		{NewBleDiscAllSvcsReq(), MSG_TYPE_DISC_ALL_SVCS},
		{NewBleDiscSvcUuidReq(), MSG_TYPE_DISC_SVC_UUID},
		{NewBleDiscAllChrsReq(), MSG_TYPE_DISC_ALL_CHRS},
		{NewBleDiscAllDscsReq(), MSG_TYPE_DISC_ALL_DSCS},
		{NewBleExchangeMtuReq(), MSG_TYPE_EXCHANGE_MTU},
		{NewBleGenRandAddrReq(), MSG_TYPE_GEN_RAND_ADDR},
		{NewBleSetRandAddrReq(), MSG_TYPE_SET_RAND_ADDR},
		{NewBleWriteCmdReq(), MSG_TYPE_WRITE_CMD},
		{NewBleWriteReq(), MSG_TYPE_WRITE},
		{NewBleScanReq(), MSG_TYPE_SCAN},
		{NewBleScanCancelReq(), MSG_TYPE_SCAN_CANCEL},
		{NewBleSetPreferredMtuReq(), MSG_TYPE_SET_PREFERRED_MTU},
		{NewBleConnFindReq(), MSG_TYPE_CONN_FIND},
		{NewResetReq(), MSG_TYPE_RESET},
		{NewBleSecurityInitiateReq(), MSG_TYPE_SECURITY_INITIATE},
		{NewBleAdvFieldsReq(), MSG_TYPE_ADV_FIELDS},
		{NewBleAdvSetDataReq(), MSG_TYPE_ADV_SET_DATA},
		{NewBleAdvRspSetDataReq(), MSG_TYPE_ADV_RSP_SET_DATA},
		{NewBleAdvStartReq(), MSG_TYPE_ADV_START},
		{NewBleAdvStopReq(), MSG_TYPE_ADV_STOP},
		{NewBleClearSvcsReq(), MSG_TYPE_CLEAR_SVCS},
		{NewBleAddSvcsReq(), MSG_TYPE_ADD_SVCS},
		{NewBleCommitSvcsReq(), MSG_TYPE_COMMIT_SVCS},
		{NewAccessStatusReq(), MSG_TYPE_ACCESS_STATUS},
		{NewNotifyReq(), MSG_TYPE_NOTIFY},
		{NewFindChrReq(), MSG_TYPE_FIND_CHR},
		{NewSyncReq(), MSG_TYPE_SYNC},
		{NewBleSmInjectIoReq(), MSG_TYPE_SM_INJECT_IO},
	}

	for _, tt := range tests {
		v := reflect.ValueOf(tt.req).Elem()
		op := MsgOp(v.FieldByName("Op").Int())
		typ := MsgType(v.FieldByName("Type").Int())
		seq := BleSeq(v.FieldByName("Seq").Uint())

		if op != MSG_OP_REQ {
			t.Errorf("%T: op = %s, want request", tt.req, MsgOpToString(op))
		}
		if typ != tt.typ {
			t.Errorf("%T: type = %s, want %s", tt.req,
				MsgTypeToString(typ), MsgTypeToString(tt.typ))
		}
		if seq >= BLE_SEQ_EVT_MIN {
			t.Errorf("%T: seq %#x in event range", tt.req, seq)
		}
	}
}

func TestStatusError(t *testing.T) {
	err := StatusError(MSG_OP_RSP, MSG_TYPE_CONNECT, ERR_CODE_EBUSY)

	bhe := nmxutil.ToBleHost(err)
	if bhe == nil {
		t.Fatalf("StatusError() returned %T, want *BleHostError", err)
	}
	if bhe.Status != ERR_CODE_EBUSY {
		t.Fatalf("Status = %d, want %d", bhe.Status, ERR_CODE_EBUSY)
	}
	for _, s := range []string{"response", "connect", "ebusy"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not contain %q", err.Error(), s)
		}
	}
}

func TestBhdTimeoutError(t *testing.T) {
	err := BhdTimeoutError(MSG_TYPE_SCAN, 17)
	if !nmxutil.IsXport(err) {
		t.Fatalf("BhdTimeoutError() returned %T, want *XportError", err)
	}
	if !strings.Contains(err.Error(), "scan") ||
		!strings.Contains(err.Error(), "seq=17") {

		t.Fatalf("unexpected error text %q", err.Error())
	}
}

func TestBleAdvReportFromScanEvt(t *testing.T) {
	u8 := uint8(6)
	i8 := int8(-4)
	u16a := uint16(10)
	u16b := uint16(20)
	u16c := uint16(0x0340)
	u16d := uint16(160)
	name := "dev"
	uri := "http://x"
	addr, _ := ParseBleAddr("01:02:03:04:05:06")
	u128, _ := ParseUuid128(NmpPlainSvcUuid)

	e := &BleScanEvt{
		EventType:              BLE_ADV_EVENT_IND,
		AddrType:               BLE_ADDR_TYPE_RANDOM,
		Addr:                   addr,
		Rssi:                   -70,
		Data:                   BleBytes{[]byte{1, 2, 3}},
		DataFlags:              &u8,
		DataUuids16:            []BleUuid16{0x1800, 0xfe18},
		DataUuids16IsComplete:  true,
		DataUuids32:            []uint32{0x12345678},
		DataUuids32IsComplete:  true,
		DataUuids128:           []BleUuid128{u128},
		DataUuids128IsComplete: true,
		DataName:               &name,
		DataNameIsComplete:     true,
		DataTxPwrLvl:           &i8,
		DataSlaveItvlMin:       &u16a,
		DataSlaveItvlMax:       &u16b,
		DataSvcDataUuid16:      BleBytes{[]byte{4}},
		DataPublicTgtAddrs:     []BleAddr{addr},
		DataAppearance:         &u16c,
		DataAdvItvl:            &u16d,
		DataSvcDataUuid32:      BleBytes{[]byte{5}},
		DataSvcDataUuid128:     BleBytes{[]byte{6}},
		DataUri:                &uri,
		DataMfgData:            BleBytes{[]byte{7, 8}},
	}

	want := BleAdvReport{
		EventType: BLE_ADV_EVENT_IND,
		Sender:    BleDev{AddrType: BLE_ADDR_TYPE_RANDOM, Addr: addr},
		Rssi:      -70,
		Fields: BleAdvFields{
			Data:               []byte{1, 2, 3},
			Flags:              &u8,
			Uuids16:            []BleUuid16{0x1800, 0xfe18},
			Uuids16IsComplete:  true,
			Uuids32:            []uint32{0x12345678},
			Uuids32IsComplete:  true,
			Uuids128:           []BleUuid128{u128},
			Uuids128IsComplete: true,
			Name:               &name,
			NameIsComplete:     true,
			TxPwrLvl:           &i8,
			SlaveItvlMin:       &u16a,
			SlaveItvlMax:       &u16b,
			SvcDataUuid16:      []byte{4},
			PublicTgtAddrs:     []BleAddr{addr},
			Appearance:         &u16c,
			AdvItvl:            &u16d,
			SvcDataUuid32:      []byte{5},
			SvcDataUuid128:     []byte{6},
			Uri:                &uri,
			MfgData:            []byte{7, 8},
		},
	}

	got := BleAdvReportFromScanEvt(e)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BleAdvReportFromScanEvt() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestBleAdvFieldsToReq(t *testing.T) {
	u8 := uint8(6)
	i8 := int8(3)
	u16a := uint16(10)
	u16b := uint16(20)
	u16c := uint16(0x0340)
	u16d := uint16(160)
	name := "dev"
	uri := "http://x"
	addr, _ := ParseBleAddr("01:02:03:04:05:06")
	u128, _ := ParseUuid128(NmpPlainSvcUuid)

	f := BleAdvFields{
		// Raw data is not part of the request.
		Data:               []byte{0xee},
		Flags:              &u8,
		Uuids16:            []BleUuid16{0x1800},
		Uuids16IsComplete:  true,
		Uuids32:            []uint32{1},
		Uuids32IsComplete:  true,
		Uuids128:           []BleUuid128{u128},
		Uuids128IsComplete: true,
		Name:               &name,
		NameIsComplete:     true,
		TxPwrLvl:           &i8,
		SlaveItvlMin:       &u16a,
		SlaveItvlMax:       &u16b,
		SvcDataUuid16:      []byte{4},
		PublicTgtAddrs:     []BleAddr{addr},
		Appearance:         &u16c,
		AdvItvl:            &u16d,
		SvcDataUuid32:      []byte{5},
		SvcDataUuid128:     []byte{6},
		Uri:                &uri,
		MfgData:            []byte{7, 8},
	}

	r := BleAdvFieldsToReq(f)

	if r.Op != MSG_OP_REQ || r.Type != MSG_TYPE_ADV_FIELDS {
		t.Fatalf("unexpected header op=%d type=%d", r.Op, r.Type)
	}

	want := *NewBleAdvFieldsReq()
	want.Seq = r.Seq
	want.Flags = &u8
	want.Uuids16 = []BleUuid16{0x1800}
	want.Uuids16IsComplete = true
	want.Uuids32 = []uint32{1}
	want.Uuids32IsComplete = true
	want.Uuids128 = []BleUuid128{u128}
	want.Uuids128IsComplete = true
	want.Name = &name
	want.NameIsComplete = true
	want.TxPwrLvl = &i8
	want.SlaveItvlMin = &u16a
	want.SlaveItvlMax = &u16b
	want.SvcDataUuid16 = BleBytes{[]byte{4}}
	want.PublicTgtAddrs = []BleAddr{addr}
	want.Appearance = &u16c
	want.AdvItvl = &u16d
	want.SvcDataUuid32 = BleBytes{[]byte{5}}
	want.SvcDataUuid128 = BleBytes{[]byte{6}}
	want.Uri = &uri
	want.MfgData = BleBytes{[]byte{7, 8}}

	if !reflect.DeepEqual(*r, want) {
		t.Fatalf("BleAdvFieldsToReq() =\n%+v\nwant\n%+v", *r, want)
	}
}

func TestBleSvcToAddSvc(t *testing.T) {
	svc := BleSvc{
		Uuid:    NewBleUuid16(0x1234),
		SvcType: BLE_SVC_TYPE_SECONDARY,
		Chrs: []BleChr{
			{
				Uuid:       NewBleUuid16(0x2a00),
				Flags:      BLE_GATT_F_READ | BLE_GATT_F_NOTIFY,
				MinKeySize: 16,
				AccessCb: func(a BleGattAccess) (uint8, []byte) {
					return 0, nil
				},
				Dscs: []BleDsc{
					{
						Uuid:       NewBleUuid16(0x2901),
						AttFlags:   BLE_ATT_F_READ,
						MinKeySize: 7,
					},
				},
			},
			{
				Uuid:  NewBleUuid16(0x2a01),
				Flags: BLE_GATT_F_WRITE,
			},
		},
	}

	want := BleAddSvc{
		Uuid:    NewBleUuid16(0x1234),
		SvcType: BLE_SVC_TYPE_SECONDARY,
		Chrs: []BleAddChr{
			{
				Uuid:       NewBleUuid16(0x2a00),
				Flags:      BLE_GATT_F_READ | BLE_GATT_F_NOTIFY,
				MinKeySize: 16,
				Dscs: []BleAddDsc{
					{
						Uuid:       NewBleUuid16(0x2901),
						AttFlags:   BLE_ATT_F_READ,
						MinKeySize: 7,
					},
				},
			},
			{
				Uuid:  NewBleUuid16(0x2a01),
				Flags: BLE_GATT_F_WRITE,
			},
		},
	}

	got := BleSvcToAddSvc(svc)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BleSvcToAddSvc() =\n%+v\nwant\n%+v", got, want)
	}

	// A service without characteristics.
	got = BleSvcToAddSvc(BleSvc{Uuid: NewBleUuid16(1)})
	if got.Chrs != nil || CompareUuids(got.Uuid, NewBleUuid16(1)) != 0 {
		t.Fatalf("unexpected result %+v", got)
	}
}

func TestBuildMgmtChrs(t *testing.T) {
	mustUuid := func(s string) BleUuid {
		u, err := ParseUuid(s)
		if err != nil {
			t.Fatalf("ParseUuid(%q): %v", s, err)
		}
		return u
	}

	chrId := func(svc string, chr string) BleChrId {
		return BleChrId{SvcUuid: mustUuid(svc), ChrUuid: mustUuid(chr)}
	}

	resReq := chrId(IotivitySvcUuid, IotivityReqChrUuid)
	resRsp := chrId(IotivitySvcUuid, IotivityRspChrUuid)

	tests := []struct {
		proto  sesn.MgmtProto
		nmpReq BleChrId
		nmpRsp BleChrId
	}{
		{
			sesn.MGMT_PROTO_NMP,
			chrId(NmpPlainSvcUuid, NmpPlainChrUuid),
			chrId(NmpPlainSvcUuid, NmpPlainChrUuid),
		},
		{
			sesn.MGMT_PROTO_OMP,
			chrId(OmpUnsecSvcUuid, OmpUnsecReqChrUuid),
			chrId(OmpUnsecSvcUuid, OmpUnsecRspChrUuid),
		},
	}

	for _, tt := range tests {
		mc, err := BuildMgmtChrs(tt.proto)
		if err != nil {
			t.Fatalf("proto %d: unexpected error: %v", tt.proto, err)
		}

		checks := []struct {
			name string
			got  *BleChrId
			want BleChrId
		}{
			{"NmpReqChr", mc.NmpReqChr, tt.nmpReq},
			{"NmpRspChr", mc.NmpRspChr, tt.nmpRsp},
			{"ResReqChr", mc.ResReqChr, resReq},
			{"ResRspChr", mc.ResRspChr, resRsp},
		}
		for _, c := range checks {
			if c.got == nil {
				t.Errorf("proto %d: %s is nil", tt.proto, c.name)
				continue
			}
			if CompareChrIds(*c.got, c.want) != 0 {
				t.Errorf("proto %d: %s = %s, want %s", tt.proto, c.name,
					c.got.String(), c.want.String())
			}
		}
	}

	if _, err := BuildMgmtChrs(sesn.MGMT_PROTO_COAP_SERVER); err == nil {
		t.Errorf("COAP_SERVER: expected error")
	}
}

func TestIsSecErr(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("x"), false},
		{nmxutil.NewXportError("x"), false},
	}

	for code := 0; code < 0x100; code++ {
		var want bool
		switch code {
		case ERR_CODE_ATT_INSUFFICIENT_AUTHEN,
			ERR_CODE_ATT_INSUFFICIENT_AUTHOR,
			ERR_CODE_ATT_INSUFFICIENT_KEY_SZ,
			ERR_CODE_ATT_INSUFFICIENT_ENC:
			want = true
		}
		tests = append(tests, struct {
			err  error
			want bool
		}{nmxutil.NewBleHostError(ERR_CODE_ATT_BASE+code, "x"), want})
	}

	// The same low bits in other error ranges are not security errors.
	tests = append(tests, struct {
		err  error
		want bool
	}{nmxutil.NewBleHostError(
		ERR_CODE_HCI_BASE+ERR_CODE_ATT_INSUFFICIENT_AUTHEN, "x"), false})

	for _, tt := range tests {
		if got := IsSecErr(tt.err); got != tt.want {
			t.Errorf("IsSecErr(%#v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestToSecurityErr(t *testing.T) {
	secCodes := map[int]bool{
		ERR_CODE_SM_ERR_PASSKEY:          true,
		ERR_CODE_SM_ERR_OOB:              true,
		ERR_CODE_SM_ERR_CONFIRM_MISMATCH: true,
		ERR_CODE_SM_ERR_UNSPECIFIED:      true,
		ERR_CODE_SM_ERR_DHKEY:            true,
		ERR_CODE_SM_ERR_NUMCMP:           true,
	}

	for _, base := range []int{ERR_CODE_SM_US_BASE, ERR_CODE_SM_PEER_BASE} {
		for code := 0; code < 0x100; code++ {
			in := nmxutil.NewBleHostError(base+code, "pairing failed")
			got := ToSecurityErr(in)

			if !secCodes[code] {
				if got != nil {
					t.Errorf("ToSecurityErr(%#x) = %v, want nil",
						base+code, got)
				}
				continue
			}

			if !nmxutil.IsBleSecurity(got) {
				t.Errorf("ToSecurityErr(%#x) = %T, want *BleSecurityError",
					base+code, got)
				continue
			}
			if got.Error() != in.Error() {
				t.Errorf("ToSecurityErr(%#x) text = %q, want %q",
					base+code, got.Error(), in.Error())
			}
		}
	}

	others := []error{
		nil,
		errors.New("x"),
		nmxutil.NewBleSecurityError("x"),
		nmxutil.NewBleHostError(ERR_CODE_ATT_BASE+ERR_CODE_SM_ERR_PASSKEY,
			"x"),
	}
	for _, e := range others {
		if got := ToSecurityErr(e); got != nil {
			t.Errorf("ToSecurityErr(%#v) = %v, want nil", e, got)
		}
	}
}
