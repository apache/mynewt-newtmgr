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
	"sort"
	"testing"

	"github.com/ugorji/go/codec"
)

func decodeCborMap(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()

	m := map[string]interface{}{}
	dec := codec.NewDecoderBytes(b, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("failed to decode CBOR body: %v", err)
	}
	return m
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestNmpHdrBytes(t *testing.T) {
	hdr := NmpHdr{
		Op:    NMP_OP_WRITE,
		Flags: 0xa5,
		Len:   0x1234,
		Group: 0xabcd,
		Seq:   0x42,
		Id:    0x07,
	}

	want := []byte{0x02, 0xa5, 0x12, 0x34, 0xab, 0xcd, 0x42, 0x07}
	got := hdr.Bytes()
	if !bytes.Equal(got, want) {
		t.Fatalf("Bytes() = % x, want % x", got, want)
	}
	if len(got) != NMP_HDR_SIZE {
		t.Fatalf("len(Bytes()) = %d, want %d", len(got), NMP_HDR_SIZE)
	}
}

func TestDecodeNmpHdr(t *testing.T) {
	data := []byte{0x03, 0x01, 0x00, 0x10, 0x00, 0x40, 0xff, 0x05}
	hdr, err := DecodeNmpHdr(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := NmpHdr{
		Op:    NMP_OP_WRITE_RSP,
		Flags: 0x01,
		Len:   0x0010,
		Group: NMP_GROUP_PERUSER,
		Seq:   0xff,
		Id:    0x05,
	}
	if *hdr != want {
		t.Fatalf("DecodeNmpHdr() = %+v, want %+v", *hdr, want)
	}

	// Trailing body bytes are ignored.
	hdr2, err := DecodeNmpHdr(append(data, 0xaa, 0xbb))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *hdr2 != want {
		t.Fatalf("DecodeNmpHdr() = %+v, want %+v", *hdr2, want)
	}
}

func TestDecodeNmpHdrShort(t *testing.T) {
	for n := 0; n < NMP_HDR_SIZE; n++ {
		hdr, err := DecodeNmpHdr(make([]byte, n))
		if err == nil {
			t.Errorf("len=%d: expected error, got hdr %+v", n, hdr)
		}
		if hdr != nil {
			t.Errorf("len=%d: expected nil hdr, got %+v", n, hdr)
		}
	}
}

func TestNmpHdrRoundTrip(t *testing.T) {
	hdrs := []NmpHdr{
		{},
		{Op: 3, Flags: 0xff, Len: 0xffff, Group: 0xffff, Seq: 0xff, Id: 0xff},
		{Op: 1, Flags: 0, Len: 1, Group: 256, Seq: 1, Id: 1},
	}

	for _, h := range hdrs {
		got, err := DecodeNmpHdr(h.Bytes())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != h {
			t.Errorf("round trip: got %+v, want %+v", *got, h)
		}
	}
}

func TestEncodeNmpPlain(t *testing.T) {
	req := NewEchoReq()
	req.Payload = "hello"
	msg := req.Msg()

	// Garbage length must be overwritten by the encoder.
	msg.Hdr.Len = 0xbeef

	data, err := EncodeNmpPlain(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hdr, err := DecodeNmpHdr(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := data[NMP_HDR_SIZE:]
	if int(hdr.Len) != len(body) {
		t.Fatalf("hdr.Len=%d, body len=%d", hdr.Len, len(body))
	}
	if msg.Hdr.Len != hdr.Len {
		t.Fatalf("msg.Hdr.Len=%d not updated, want %d", msg.Hdr.Len, hdr.Len)
	}
	if hdr.Op != NMP_OP_WRITE || hdr.Group != NMP_GROUP_DEFAULT ||
		hdr.Id != NMP_ID_DEF_ECHO || hdr.Seq != req.Hdr().Seq {
		t.Fatalf("unexpected header: %+v", *hdr)
	}

	m := decodeCborMap(t, body)
	if !reflect.DeepEqual(sortedKeys(m), []string{"d"}) {
		t.Fatalf("unexpected body keys: %v", sortedKeys(m))
	}
	if m["d"] != "hello" {
		t.Fatalf("d=%v, want hello", m["d"])
	}
}

func TestFillNmpReqWithSeq(t *testing.T) {
	r := NewImageUploadReqWithSeq(0x42)
	want := NmpHdr{
		Op:    NMP_OP_WRITE,
		Group: NMP_GROUP_IMAGE,
		Id:    NMP_ID_IMAGE_UPLOAD,
		Seq:   0x42,
	}
	if *r.Hdr() != want {
		t.Fatalf("hdr = %+v, want %+v", *r.Hdr(), want)
	}
}

func TestNmpBaseSetHdr(t *testing.T) {
	var b NmpBase
	h := NmpHdr{Op: 1, Group: 2, Id: 3, Seq: 4}
	b.SetHdr(&h)
	h.Seq = 5
	if b.Hdr().Seq != 4 {
		t.Fatalf("SetHdr must copy the header")
	}
}

// Every request constructor must set the correct op/group/id, and encoding
// a request must only produce the request's own CBOR fields: the NmpBase
// header must never leak into the body, and "omitempty" fields must be
// left out when empty.
func TestNewReqs(t *testing.T) {
	tests := []struct {
		name  string
		req   NmpReq
		op    uint8
		group uint16
		id    uint8
		keys  []string
	}{
		{"echo", NewEchoReq(),
			NMP_OP_WRITE, NMP_GROUP_DEFAULT, NMP_ID_DEF_ECHO,
			[]string{"d"}},
		{"taskstat", NewTaskStatReq(),
			NMP_OP_READ, NMP_GROUP_DEFAULT, NMP_ID_DEF_TASKSTAT,
			[]string{}},
		{"mpstat", NewMempoolStatReq(),
			NMP_OP_READ, NMP_GROUP_DEFAULT, NMP_ID_DEF_MPSTAT,
			[]string{}},
		{"datetime read", NewDateTimeReadReq(),
			NMP_OP_READ, NMP_GROUP_DEFAULT, NMP_ID_DEF_DATETIME_STR,
			[]string{}},
		{"datetime write", NewDateTimeWriteReq(),
			NMP_OP_WRITE, NMP_GROUP_DEFAULT, NMP_ID_DEF_DATETIME_STR,
			[]string{"datetime"}},
		{"reset", NewResetReq(),
			NMP_OP_WRITE, NMP_GROUP_DEFAULT, NMP_ID_DEF_RESET,
			[]string{}},
		{"mcumgr params", NewMcumgrParamsReq(),
			NMP_OP_READ, NMP_GROUP_DEFAULT, NMP_ID_DEF_PARAMS,
			[]string{}},
		{"image upload", NewImageUploadReq(),
			NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_UPLOAD,
			[]string{"data", "image", "off"}},
		{"image state read", NewImageStateReadReq(),
			NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_STATE,
			[]string{}},
		{"image state write", NewImageStateWriteReq(),
			NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_STATE,
			[]string{"confirm", "hash"}},
		{"core list", NewCoreListReq(),
			NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELIST,
			[]string{}},
		{"core load", NewCoreLoadReq(),
			NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELOAD,
			[]string{"off"}},
		{"core erase", NewCoreEraseReq(),
			NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELOAD,
			[]string{}},
		{"image erase", NewImageEraseReq(),
			NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_ERASE,
			[]string{}},
		{"stat read", NewStatReadReq(),
			NMP_OP_READ, NMP_GROUP_STAT, NMP_ID_STAT_READ,
			[]string{"name"}},
		{"stat list", NewStatListReq(),
			NMP_OP_READ, NMP_GROUP_STAT, NMP_ID_STAT_LIST,
			[]string{}},
		{"log show", NewLogShowReq(),
			NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_SHOW,
			[]string{"index", "log_name", "ts"}},
		{"log list", NewLogListReq(),
			NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_LIST,
			[]string{}},
		{"log module list", NewLogModuleListReq(),
			NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_MODULE_LIST,
			[]string{}},
		{"log level list", NewLogLevelListReq(),
			NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_LEVEL_LIST,
			[]string{}},
		{"log clear", NewLogClearReq(),
			NMP_OP_WRITE, NMP_GROUP_LOG, NMP_ID_LOG_CLEAR,
			[]string{}},
		{"crash", NewCrashReq(),
			NMP_OP_WRITE, NMP_GROUP_CRASH, NMP_ID_CRASH_TRIGGER,
			[]string{"t"}},
		{"run test", NewRunTestReq(),
			NMP_OP_WRITE, NMP_GROUP_RUN, NMP_ID_RUN_TEST,
			[]string{"testname", "token"}},
		{"run list", NewRunListReq(),
			NMP_OP_READ, NMP_GROUP_RUN, NMP_ID_RUN_LIST,
			[]string{}},
		{"fs download", NewFsDownloadReq(),
			NMP_OP_READ, NMP_GROUP_FS, NMP_ID_FS_FILE,
			[]string{"name", "off"}},
		{"fs upload", NewFsUploadReq(),
			NMP_OP_WRITE, NMP_GROUP_FS, NMP_ID_FS_FILE,
			[]string{"data", "len", "name", "off"}},
		{"config read", NewConfigReadReq(),
			NMP_OP_READ, NMP_GROUP_CONFIG, NMP_ID_CONFIG_VAL,
			[]string{"name"}},
		{"config write", NewConfigWriteReq(),
			NMP_OP_WRITE, NMP_GROUP_CONFIG, NMP_ID_CONFIG_VAL,
			[]string{}},
		{"shell exec", NewShellExecReq(),
			NMP_OP_WRITE, NMP_GROUP_SHELL, NMP_ID_SHELL_EXEC,
			[]string{"argv"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := tt.req.Hdr()
			if hdr.Op != tt.op || hdr.Group != tt.group || hdr.Id != tt.id {
				t.Fatalf("hdr = %+v, want op=%d group=%d id=%d",
					*hdr, tt.op, tt.group, tt.id)
			}
			if hdr.Flags != 0 || hdr.Len != 0 {
				t.Fatalf("hdr = %+v, want flags=0 len=0", *hdr)
			}

			msg := tt.req.Msg()
			if msg.Hdr != *hdr {
				t.Fatalf("Msg().Hdr = %+v, want %+v", msg.Hdr, *hdr)
			}

			data, err := EncodeNmpPlain(msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			m := decodeCborMap(t, data[NMP_HDR_SIZE:])
			if got := sortedKeys(m); !reflect.DeepEqual(got, tt.keys) {
				t.Fatalf("body keys = %v, want %v", got, tt.keys)
			}
		})
	}
}

func TestImageUploadReqOmitEmpty(t *testing.T) {
	// First chunk: len, sha and upgrade are present.
	r := NewImageUploadReq()
	r.Off = 0
	r.Len = 1000
	r.DataSha = []byte{1, 2, 3}
	r.Upgrade = true
	r.Data = []byte{0xaa}

	data, err := EncodeNmpPlain(r.Msg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := decodeCborMap(t, data[NMP_HDR_SIZE:])
	want := []string{"data", "image", "len", "off", "sha", "upgrade"}
	if got := sortedKeys(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("body keys = %v, want %v", got, want)
	}

	// Subsequent chunk: only image, off and data.
	r = NewImageUploadReq()
	r.Off = 500
	r.Data = []byte{0xbb}

	data, err = EncodeNmpPlain(r.Msg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m = decodeCborMap(t, data[NMP_HDR_SIZE:])
	want = []string{"data", "image", "off"}
	if got := sortedKeys(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("body keys = %v, want %v", got, want)
	}
}

func TestConfigWriteReqOmitEmpty(t *testing.T) {
	tests := []struct {
		name string
		val  string
		save bool
		keys []string
	}{
		{"", "", true, []string{"save"}},
		{"foo/bar", "1", false, []string{"name", "val"}},
		{"foo/bar", "1", true, []string{"name", "save", "val"}},
	}

	for _, tt := range tests {
		r := NewConfigWriteReq()
		r.Name = tt.name
		r.Val = tt.val
		r.Save = tt.save

		data, err := EncodeNmpPlain(r.Msg())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := decodeCborMap(t, data[NMP_HDR_SIZE:])
		if got := sortedKeys(m); !reflect.DeepEqual(got, tt.keys) {
			t.Errorf("%+v: body keys = %v, want %v", tt, got, tt.keys)
		}
	}
}

func TestSplitStatusString(t *testing.T) {
	tests := []struct {
		s    SplitStatus
		want string
	}{
		{NOT_APPLICABLE, "N/A"},
		{NOT_MATCHING, "non-matching"},
		{MATCHING, "matching"},
		{SplitStatus(99), "Unknown!"},
	}

	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("SplitStatus(%d).String() = %q, want %q",
				int(tt.s), got, tt.want)
		}
	}
}
