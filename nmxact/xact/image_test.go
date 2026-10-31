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

package xact

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sync"
	"testing"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func testImage(sz int) []byte {
	data := make([]byte, sz)
	for i := range data {
		data[i] = byte(i*7 + 3)
	}
	return data
}

type protoVariant struct {
	name  string
	proto sesn.MgmtProto
	isTcp bool
}

var protoVariants = []protoVariant{
	{"nmp", sesn.MGMT_PROTO_NMP, false},
	{"omp-dgram", sesn.MGMT_PROTO_OMP, false},
	{"omp-tcp", sesn.MGMT_PROTO_OMP, true},
}

func newProtoSesn(pv protoVariant, mtu int, h fakeHandler) *fakeSesn {
	s := newFakeSesn(h)
	s.proto = pv.proto
	s.isTcp = pv.isTcp
	s.mtu = mtu
	return s
}

func encLen(t *testing.T, s sesn.Sesn, m *nmp.NmpMsg) int {
	t.Helper()

	enc, err := mgmt.EncodeMgmt(s, m)
	if err != nil {
		t.Fatalf("EncodeMgmt: %v", err)
	}
	return len(enc)
}

func TestBuildImageUploadReq(t *testing.T) {
	hash := []byte{1, 2, 3}
	chunk := []byte{4, 5}

	r := buildImageUploadReq(1000, hash, true, chunk, 0, 1, 42)
	if r.Len != 1000 || !bytes.Equal(r.DataSha, hash) || !r.Upgrade {
		t.Errorf("off 0: len/sha/upgrade not set: %+v", r)
	}
	if r.Off != 0 || !bytes.Equal(r.Data, chunk) || r.ImageNum != 1 {
		t.Errorf("off 0: wrong off/data/image: %+v", r)
	}
	if r.Hdr().Seq != 42 {
		t.Errorf("seq: got %d want 42", r.Hdr().Seq)
	}

	// Only the first chunk carries len, sha and upgrade (733feb6).
	r = buildImageUploadReq(1000, hash, true, chunk, 10, 1, 43)
	if r.Len != 0 || r.DataSha != nil || r.Upgrade {
		t.Errorf("off 10: len/sha/upgrade set: %+v", r)
	}
	if r.Off != 10 || !bytes.Equal(r.Data, chunk) {
		t.Errorf("off 10: wrong off/data: %+v", r)
	}
}

func TestNextImageUploadReqMaxReqLen(t *testing.T) {
	data := testImage(3000)

	// The MTU is too small for any image data; a larger request length
	// (peer reassembles requests split over several packets) must be used
	// instead of the MTU.
	s := newProtoSesn(protoVariant{"nmp-tcp", sesn.MGMT_PROTO_NMP, true},
		20, nil)

	if _, err := nextImageUploadReq(s, false, data, 0, 0, 0); err == nil {
		t.Errorf("mtu=20: no error")
	}

	for _, reqLen := range []int{100, 300, 1024} {
		for _, off := range []int{0, 600} {
			r, err := nextImageUploadReq(s, false, data, off, 0, reqLen)
			if err != nil {
				t.Fatalf("reqLen=%d off=%d: unexpected error: %v", reqLen,
					off, err)
			}

			pfx := fmt.Sprintf("reqLen=%d off=%d", reqLen, off)
			l := encLen(t, s, r.Msg())
			if l > reqLen {
				t.Errorf("%s: request len %d > reqLen", pfx, l)
			}
			if len(r.Data) < IMAGE_UPLOAD_MAX_CHUNK && l < reqLen-20 {
				t.Errorf("%s: request len %d; chunk not maximal", pfx, l)
			}
		}
	}
}

func TestNextImageUploadReqMtuSweep(t *testing.T) {
	data := testImage(3000)
	sha := sha256.Sum256(data)

	for _, pv := range protoVariants {
		for mtu := 20; mtu <= 1024; mtu++ {
			// Every MTU around the interesting boundaries; sample the
			// rest to keep the test fast.
			if mtu > 300 && mtu%5 != 0 {
				continue
			}
			s := newProtoSesn(pv, mtu, nil)

			for _, off := range []int{0, 1, 600, len(data) - 1} {
				r, err := nextImageUploadReq(s, false, data, off, 0, 0)
				if err != nil {
					// Too small MTU must be reported, not panic.
					if mtu > 100 {
						t.Errorf("%s mtu=%d off=%d: unexpected error: %v",
							pv.name, mtu, off, err)
					}
					continue
				}

				pfx := fmt.Sprintf("%s mtu=%d off=%d", pv.name, mtu, off)
				if l := encLen(t, s, r.Msg()); l > mtu {
					t.Errorf("%s: request len %d > mtu", pfx, l)
				}
				if len(r.Data) == 0 {
					t.Errorf("%s: empty chunk", pfx)
				}
				if len(r.Data) > IMAGE_UPLOAD_MAX_CHUNK {
					t.Errorf("%s: chunk %d > %d", pfx, len(r.Data),
						IMAGE_UPLOAD_MAX_CHUNK)
				}
				if int(r.Off) != off {
					t.Errorf("%s: req off %d", pfx, r.Off)
				}
				if !bytes.Equal(r.Data, data[off:off+len(r.Data)]) {
					t.Errorf("%s: wrong chunk data", pfx)
				}

				if off == 0 {
					if int(r.Len) != len(data) {
						t.Errorf("%s: len %d", pfx, r.Len)
					}
					if r.DataSha != nil &&
						!bytes.Equal(r.DataSha, sha[:]) {
						t.Errorf("%s: wrong sha", pfx)
					}
					// The hash is only dropped if it does not leave
					// room for a minimal first chunk (aba21b8).  The
					// chunk length calculation may leave up to two
					// bytes unused (see TestNextImageUploadReqUsesMtu).
					if r.DataSha == nil &&
						len(data) >= IMAGE_UPLOAD_MIN_1ST_CHUNK {

						withSha := buildImageUploadReq(len(data), sha[:],
							false, data[:IMAGE_UPLOAD_MIN_1ST_CHUNK+2], 0,
							0, r.Hdr().Seq)
						if encLen(t, s, withSha.Msg()) <= mtu {
							t.Errorf("%s: sha dropped needlessly", pfx)
						}
					}
				} else if r.Len != 0 || r.DataSha != nil {
					t.Errorf("%s: len/sha in non-first chunk", pfx)
				}
			}
		}
	}
}

// The chunk length is reduced in multiple rounds until the request fits
// (c953418); the result must still use the MTU well.  The CBOR length
// prefix of the data shrinks along with the chunk, so up to two bytes may
// remain unused.
func TestNextImageUploadReqUsesMtu(t *testing.T) {
	data := testImage(3000)

	for _, pv := range protoVariants {
		for mtu := 100; mtu <= 600; mtu++ {
			s := newProtoSesn(pv, mtu, nil)
			r, err := nextImageUploadReq(s, false, data, 1000, 0, 0)
			if err != nil {
				t.Fatalf("%s mtu=%d: %v", pv.name, mtu, err)
			}
			if len(r.Data) == IMAGE_UPLOAD_MAX_CHUNK {
				continue
			}

			// Three more bytes of data must not fit.
			bigger := buildImageUploadReq(len(data), nil, false,
				data[1000:1000+len(r.Data)+3], 1000, 0, r.Hdr().Seq)
			if l := encLen(t, s, bigger.Msg()); l <= mtu {
				t.Errorf("%s mtu=%d: chunk %d not maximal (%d <= %d)",
					pv.name, mtu, len(r.Data), l, mtu)
			}
		}
	}
}

func TestNextImageUploadReqSmallMtuDropsSha(t *testing.T) {
	data := testImage(1000)

	for _, pv := range protoVariants {
		// Find an MTU at which the first chunk only fits without the hash
		// (aba21b8).
		found := false
		for mtu := 20; mtu <= 200; mtu++ {
			s := newProtoSesn(pv, mtu, nil)
			r, err := nextImageUploadReq(s, false, data, 0, 0, 0)
			if err != nil {
				continue
			}
			if r.DataSha == nil {
				found = true
				if len(r.Data) == 0 {
					t.Errorf("%s mtu=%d: no data", pv.name, mtu)
				}
				if int(r.Len) != len(data) {
					t.Errorf("%s mtu=%d: len missing", pv.name, mtu)
				}
			}
		}
		if !found {
			t.Errorf("%s: sha never dropped", pv.name)
		}
	}
}

func TestNextImageUploadReqMtuTooSmall(t *testing.T) {
	data := testImage(1000)

	for _, pv := range protoVariants {
		for mtu := 0; mtu < 20; mtu++ {
			s := newProtoSesn(pv, mtu, nil)
			for _, off := range []int{0, 500} {
				if _, err := nextImageUploadReq(s, false, data, off,
					0, 0); err == nil {

					t.Errorf("%s mtu=%d off=%d: no error", pv.name, mtu,
						off)
				}
			}
		}
	}
}

func TestNextImageUploadReqFields(t *testing.T) {
	data := testImage(1000)

	for _, pv := range protoVariants {
		s := newProtoSesn(pv, 512, nil)

		// Last chunk.
		r, err := nextImageUploadReq(s, false, data, len(data)-5, 2, 0)
		if err != nil {
			t.Fatalf("%s: %v", pv.name, err)
		}
		if !bytes.Equal(r.Data, data[len(data)-5:]) {
			t.Errorf("%s: wrong last chunk: %d bytes", pv.name, len(r.Data))
		}
		if r.ImageNum != 2 {
			t.Errorf("%s: image num %d", pv.name, r.ImageNum)
		}
		if r.Upgrade {
			t.Errorf("%s: upgrade set in non-first chunk", pv.name)
		}

		// Upgrade flag is only sent in the first chunk, when requested.
		for _, upgrade := range []bool{false, true} {
			r, err = nextImageUploadReq(s, upgrade, data, 0, 3, 0)
			if err != nil {
				t.Fatalf("%s: %v", pv.name, err)
			}
			if r.Upgrade != upgrade {
				t.Errorf("%s: upgrade %v want %v", pv.name, r.Upgrade,
					upgrade)
			}
			if r.ImageNum != 3 {
				t.Errorf("%s: image num %d", pv.name, r.ImageNum)
			}
		}

		r, err = nextImageUploadReq(s, true, data, 100, 0, 0)
		if err != nil {
			t.Fatalf("%s: %v", pv.name, err)
		}
		if r.Upgrade {
			t.Errorf("%s: upgrade set at off 100", pv.name)
		}
	}
}

// growFilter is a CoAP transmit filter which pads every message with a
// different amount of data unless frozen, similar to an encrypting filter
// which uses new parameters for each message.
type growFilter struct {
	mtx      sync.Mutex
	frozen   bool
	extra    int
	freezes  int
	unfrozen int
}

func (f *growFilter) Filter(m coap.Message) (coap.Message, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if !f.frozen {
		f.extra++
		f.unfrozen++
	}
	pl := append([]byte{}, m.Payload()...)
	m.SetPayload(append(pl, make([]byte, f.extra)...))
	return m, nil
}

func (f *growFilter) Freeze() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.frozen = true
	f.freezes++
	f.extra += 3
}

func (f *growFilter) Unfreeze() {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	f.frozen = false
}

func TestNextImageUploadReqFreezesFilter(t *testing.T) {
	data := testImage(3000)

	for _, pv := range protoVariants[1:] {
		for _, off := range []int{0, 1000} {
			for mtu := 100; mtu <= 600; mtu += 7 {
				f := &growFilter{}
				s := newProtoSesn(pv, mtu, nil)
				s.txFilter = f

				r, err := nextImageUploadReq(s, false, data, off, 0, 0)
				if err != nil {
					t.Fatalf("%s mtu=%d: %v", pv.name, mtu, err)
				}
				if f.frozen {
					t.Errorf("%s mtu=%d: filter left frozen", pv.name, mtu)
				}
				if f.freezes != 1 || f.unfrozen != 0 {
					t.Errorf("%s mtu=%d: freezes=%d unfrozen=%d",
						pv.name, mtu, f.freezes, f.unfrozen)
				}

				// Encoding with the parameters used during the chunk
				// size calculation must fit the MTU (2452835).
				f.Freeze()
				f.extra -= 3
				l := encLen(t, s, r.Msg())
				f.Unfreeze()
				if l > mtu {
					t.Errorf("%s mtu=%d: len %d > mtu", pv.name, mtu, l)
				}
			}
		}
	}
}

func TestImageUploadTrackerUpdate(t *testing.T) {
	tr := ImageUploadIntTracker{RspMap: map[int]int{}}

	tr.UpdateTracker(100, IMAGE_UPLOAD_STATUS_EXPECTED)
	if tr.RspMap[100] != 1 {
		t.Errorf("expected: got %d", tr.RspMap[100])
	}
	tr.UpdateTracker(100, IMAGE_UPLOAD_STATUS_RQ)
	if tr.RspMap[100] != 0 {
		t.Errorf("rq: got %d", tr.RspMap[100])
	}
	tr.UpdateTracker(100, IMAGE_UPLOAD_STATUS_RQ)
	if tr.RspMap[100] != -1 {
		t.Errorf("repeated rq: got %d", tr.RspMap[100])
	}
	tr.UpdateTracker(200, IMAGE_UPLOAD_STATUS_MISSED)
	if tr.RspMap[200] != IMAGE_UPLOAD_CHUNK_MISSED_WM {
		t.Errorf("missed: got %d", tr.RspMap[200])
	}
}

func TestImageUploadTrackerProcessMissedChunks(t *testing.T) {
	tr := ImageUploadIntTracker{
		Off: 500,
		RspMap: map[int]int{
			100: 0,  // done
			200: 1,  // in flight
			300: -1, // at watermark: not yet missed
			400: -2, // missed
		},
	}

	tr.ProcessMissedChunks()

	if tr.Off != 400 {
		t.Errorf("off: got %d want 400", tr.Off)
	}
	want := map[int]int{200: 1, 300: -1}
	if len(tr.RspMap) != len(want) {
		t.Errorf("map: got %v want %v", tr.RspMap, want)
	}
	for k, v := range want {
		if tr.RspMap[k] != v {
			t.Errorf("map[%d]: got %d want %d", k, tr.RspMap[k], v)
		}
	}
}

func TestImageUploadTrackerHandleResponse(t *testing.T) {
	var cbOffs []uint32
	c := NewImageUploadCmd()
	c.MaxWinSz = 2
	c.ProgressCb = func(_ *ImageUploadCmd, r *nmp.ImageUploadRsp) {
		cbOffs = append(cbOffs, r.Off)
	}
	res := newImageUploadResult()

	tr := ImageUploadIntTracker{
		TuneWS: true,
		WCount: 1,
		WCap:   1,
		RspMap: map[int]int{100: 1},
	}

	// Full window gets an open slot -> signal; window grows.
	if !tr.HandleResponse(c, &nmp.ImageUploadRsp{Off: 100}, res) {
		t.Errorf("no signal on full -> open transition")
	}
	if tr.WCap != 2 || tr.WCount != 0 || tr.MaxRxOff != 100 {
		t.Errorf("after rsp 1: cap=%d count=%d maxrx=%d", tr.WCap,
			tr.WCount, tr.MaxRxOff)
	}
	if tr.RspMap[100] != 0 {
		t.Errorf("rsp map: %v", tr.RspMap)
	}

	// Window not full -> no signal; window capped at MaxWinSz.
	tr.WCount = 1
	if tr.HandleResponse(c, &nmp.ImageUploadRsp{Off: 50}, res) {
		t.Errorf("signal without full window")
	}
	if tr.WCap != 2 || tr.WCount != 0 || tr.MaxRxOff != 100 {
		t.Errorf("after rsp 2: cap=%d count=%d maxrx=%d", tr.WCap,
			tr.WCount, tr.MaxRxOff)
	}

	// A nil response only releases the slot.
	tr.WCount = 2
	if !tr.HandleResponse(c, nil, res) {
		t.Errorf("no signal on full -> open transition (nil rsp)")
	}

	if len(res.Rsps) != 2 || res.Rsps[0].Off != 100 ||
		res.Rsps[1].Off != 50 {

		t.Errorf("result rsps: %+v", res.Rsps)
	}
	if len(cbOffs) != 2 || cbOffs[0] != 100 || cbOffs[1] != 50 {
		t.Errorf("progress offs: %v", cbOffs)
	}
}

func TestImageUploadTrackerHandleError(t *testing.T) {
	tr := ImageUploadIntTracker{
		TuneWS: true,
		WCount: 4,
		WCap:   4,
		RspMap: map[int]int{},
	}
	err := fmt.Errorf("boom")

	// Window full and shrinking along with the count: no open slot yet.
	if tr.HandleError(100, err) {
		t.Errorf("signal although window still full")
	}
	if tr.WCap != 3 || tr.WCount != 3 || tr.TuneWS {
		t.Errorf("after err 1: cap=%d count=%d tune=%v", tr.WCap,
			tr.WCount, tr.TuneWS)
	}
	if tr.RspMap[100] != IMAGE_UPLOAD_CHUNK_MISSED_WM {
		t.Errorf("rsp map: %v", tr.RspMap)
	}

	if tr.HandleError(200, err) {
		t.Errorf("signal although window still full")
	}
	if tr.WCap != 2 || tr.WCount != 2 {
		t.Errorf("after err 2: cap=%d count=%d", tr.WCap, tr.WCount)
	}

	// The window does not shrink below IMAGE_UPLOAD_START_WS + 1, so
	// the slot opens now.
	if !tr.HandleError(300, err) {
		t.Errorf("no signal on full -> open transition")
	}
	if tr.WCap != 2 || tr.WCount != 1 {
		t.Errorf("after err 3: cap=%d count=%d", tr.WCap, tr.WCount)
	}
}
