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
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// fakeImgDev emulates the image upload handler of a device.
type fakeImgDev struct {
	mtx sync.Mutex

	data    []byte
	expLen  int
	sha     []byte
	upgrade bool
	img     int

	// If non-zero, upload requests with an offset of at least failOff
	// are answered with this rc.
	failRc  int
	failOff uint32

	// Offsets of requests answered with a timeout error, once each.
	dropOffs map[uint32]bool
}

func (d *fakeImgDev) handle(req fakeReq) (nmp.NmpRsp, error) {
	ur, ok := req.Body.(*nmp.ImageUploadReq)
	if !ok {
		return nil, fmt.Errorf("unexpected request %T", req.Body)
	}

	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.dropOffs[ur.Off] {
		delete(d.dropOffs, ur.Off)
		return nil, nmxutil.NewRspTimeoutError("NMP timeout")
	}

	if d.failRc != 0 && ur.Off >= d.failOff {
		return &nmp.ImageUploadRsp{Rc: d.failRc}, nil
	}

	if ur.Off == 0 {
		d.data = nil
		d.expLen = int(ur.Len)
		d.sha = ur.DataSha
		d.upgrade = ur.Upgrade
		d.img = int(ur.ImageNum)
	}

	// Like mcumgr: a request with an unexpected offset is answered with
	// the offset the device expects.
	if int(ur.Off) == len(d.data) {
		d.data = append(d.data, ur.Data...)
	}

	return &nmp.ImageUploadRsp{Rc: 0, Off: uint32(len(d.data))}, nil
}

func (d *fakeImgDev) Data() []byte {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	return append([]byte(nil), d.data...)
}

func uploadReqs(t *testing.T, s *fakeSesn) []*nmp.ImageUploadReq {
	t.Helper()

	var reqs []*nmp.ImageUploadReq
	for _, r := range s.Requests() {
		ur, ok := r.Body.(*nmp.ImageUploadReq)
		if !ok {
			continue
		}
		reqs = append(reqs, ur)
	}
	return reqs
}

func checkUploaded(t *testing.T, dev *fakeImgDev, data []byte) {
	t.Helper()

	if got := dev.Data(); !bytes.Equal(got, data) {
		t.Errorf("device got %d bytes, want %d", len(got), len(data))
	}
}

func TestImageUploadRun(t *testing.T) {
	for _, pv := range protoVariants {
		// Size of the first chunk at an MTU of 256.
		r, err := nextImageUploadReq(newProtoSesn(pv, 256, nil), false,
			testImage(1000), 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		oneChunk := len(r.Data)

		tests := []struct {
			name  string
			sz    int
			winSz int
		}{
			{"1 byte", 1, 1},
			{"32 bytes", 32, 1},
			{"one chunk", oneChunk, 1},
			{"one chunk + 1", oneChunk + 1, 1},
			{"many win 1", 10000, 1},
			{"many win 2", 10000, 2},
			{"many win 5", 10000, 5},
		}

		for _, tt := range tests {
			name := pv.name + "/" + tt.name
			data := testImage(tt.sz)
			dev := &fakeImgDev{}
			s := newProtoSesn(pv, 256, dev.handle)

			c := NewImageUploadCmd()
			c.Data = data
			c.MaxWinSz = tt.winSz
			c.ImageNum = 1

			res, err := runCmd(t, c, s)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			checkUploaded(t, dev, data)

			ures := res.(*ImageUploadResult)
			if ures.Status() != 0 {
				t.Errorf("%s: status %d", name, ures.Status())
			}

			reqs := uploadReqs(t, s)
			if len(reqs) == 0 {
				t.Fatalf("%s: no requests", name)
			}
			// XXX: Images shorter than IMAGE_UPLOAD_MIN_1ST_CHUNK are
			// sent without hash; only check the hash for larger ones.
			sha := sha256.Sum256(data)
			if dev.expLen != len(data) || (tt.sz >=
				IMAGE_UPLOAD_MIN_1ST_CHUNK && !bytes.Equal(dev.sha, sha[:])) {

				t.Errorf("%s: wrong sha/len in first chunk", name)
			}
			if dev.img != 1 {
				t.Errorf("%s: image num %d", name, dev.img)
			}
			for i, r := range s.Requests() {
				if r.EncLen > s.mtu {
					t.Errorf("%s: req %d len %d > mtu", name, i, r.EncLen)
				}
				if i > 0 {
					if _, ok := r.Fields["len"]; ok {
						t.Errorf("%s: req %d has len", name, i)
					}
					if _, ok := r.Fields["sha"]; ok {
						t.Errorf("%s: req %d has sha", name, i)
					}
				}
			}
			if tt.sz <= oneChunk && len(reqs) != 1 {
				t.Errorf("%s: %d requests, want 1", name, len(reqs))
			}
		}
	}
}

func TestImageUploadRunEmpty(t *testing.T) {
	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)

	c := NewImageUploadCmd()
	c.MaxWinSz = 1

	if _, err := runCmd(t, c, s); err != nil {
		t.Fatal(err)
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d requests for empty image", n)
	}
}

func TestImageUploadRunSeq(t *testing.T) {
	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)
	s.mtu = 128

	c := NewImageUploadCmd()
	c.Data = testImage(5000)
	c.MaxWinSz = 1

	if _, err := runCmd(t, c, s); err != nil {
		t.Fatal(err)
	}

	// Each transmitted request consumes exactly one sequence number
	// (d9f7558).
	reqs := s.Requests()
	for i := 1; i < len(reqs); i++ {
		if reqs[i].Hdr.Seq != reqs[i-1].Hdr.Seq+1 {
			t.Fatalf("req %d seq %d after %d", i, reqs[i].Hdr.Seq,
				reqs[i-1].Hdr.Seq)
		}
	}
}

func TestImageUploadRunStartOff(t *testing.T) {
	data := testImage(3000)
	dev := &fakeImgDev{data: append([]byte(nil), data[:1000]...)}
	s := newFakeSesn(dev.handle)

	c := NewImageUploadCmd()
	c.Data = data
	c.StartOff = 1000
	c.MaxWinSz = 1

	if _, err := runCmd(t, c, s); err != nil {
		t.Fatal(err)
	}
	checkUploaded(t, dev, data)

	reqs := uploadReqs(t, s)
	if reqs[0].Off != 1000 {
		t.Errorf("first req off %d want 1000", reqs[0].Off)
	}
	for _, r := range reqs {
		if r.Off < 1000 || r.Len != 0 || r.DataSha != nil {
			t.Errorf("unexpected req: off=%d len=%d sha=%x", r.Off, r.Len,
				r.DataSha)
		}
	}
}

func TestImageUploadRunProgress(t *testing.T) {
	data := testImage(5000)
	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)
	s.mtu = 200

	var mtx sync.Mutex
	var offs []uint32

	c := NewImageUploadCmd()
	c.Data = data
	c.MaxWinSz = 1
	c.ProgressCb = func(_ *ImageUploadCmd, r *nmp.ImageUploadRsp) {
		mtx.Lock()
		defer mtx.Unlock()
		offs = append(offs, r.Off)
	}

	if _, err := runCmd(t, c, s); err != nil {
		t.Fatal(err)
	}

	mtx.Lock()
	defer mtx.Unlock()

	if len(offs) != len(s.Requests()) {
		t.Errorf("%d progress callbacks for %d requests", len(offs),
			len(s.Requests()))
	}
	for i := 1; i < len(offs); i++ {
		if offs[i] <= offs[i-1] {
			t.Errorf("progress not increasing: %v", offs)
			break
		}
	}
}

func TestImageUploadRunRetransmit(t *testing.T) {
	data := testImage(5000)

	for _, winSz := range []int{1, 5} {
		s := newFakeSesn(nil)
		s.mtu = 200

		// Learn the chunk offsets from a clean upload first.
		clean := &fakeImgDev{}
		s.handler = clean.handle
		c := NewImageUploadCmd()
		c.Data = data
		c.MaxWinSz = winSz
		if _, err := runCmd(t, c, s); err != nil {
			t.Fatal(err)
		}
		reqs := uploadReqs(t, s)
		if len(reqs) < 4 {
			t.Fatalf("only %d chunks", len(reqs))
		}

		for _, drop := range []int{0, 1, len(reqs) / 2} {
			dev := &fakeImgDev{
				dropOffs: map[uint32]bool{reqs[drop].Off: true},
			}
			s := newFakeSesn(dev.handle)
			s.mtu = 200

			c := NewImageUploadCmd()
			c.Data = data
			c.MaxWinSz = winSz
			if _, err := runCmd(t, c, s); err != nil {
				t.Fatalf("win %d drop %d: %v", winSz, drop, err)
			}
			checkUploaded(t, dev, data)
		}
	}
}

// Window based uploads used to deadlock occasionally: a response
// handler could signal a free slot while the main loop was not waiting,
// leaving a stale signal behind which let the loop overfill the window.
// With the window then growing while the count shrank, no handler
// observed a full window anymore and the main loop waited forever.
// The deadlock depends on goroutine scheduling, so run many uploads.
func TestImageUploadRunWindowStress(t *testing.T) {
	data := testImage(2000)

	for _, winSz := range []int{2, 5} {
		for i := 0; i < 50; i++ {
			dev := &fakeImgDev{}
			s := newFakeSesn(dev.handle)
			s.mtu = 128

			c := NewImageUploadCmd()
			c.Data = data
			c.MaxWinSz = winSz
			if _, err := runCmd(t, c, s); err != nil {
				t.Fatalf("win %d iter %d: %v", winSz, i, err)
			}
			checkUploaded(t, dev, data)
		}
	}
}

func TestImageUploadRunMtuTooSmall(t *testing.T) {
	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)
	s.mtu = 10

	c := NewImageUploadCmd()
	c.Data = testImage(100)
	c.MaxWinSz = 1

	if _, err := runCmd(t, c, s); err == nil {
		t.Errorf("no error with tiny MTU")
	}
}

func TestImageUploadRunAborted(t *testing.T) {
	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)

	c := NewImageUploadCmd()
	c.Data = testImage(100)
	c.MaxWinSz = 1
	if err := c.Abort(); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, c, s); err == nil {
		t.Errorf("no error after abort")
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d requests after abort", n)
	}
}

// imgUpgradeDev emulates a device handling image erase and upload.
type imgUpgradeDev struct {
	fakeImgDev

	s *fakeSesn

	eraseRc  int
	eraseErr error

	// Close the session when the erase request fails.
	eraseDisconnect bool

	erases int
}

func (d *imgUpgradeDev) handle(req fakeReq) (nmp.NmpRsp, error) {
	if req.Hdr.Group == nmp.NMP_GROUP_IMAGE &&
		req.Hdr.Id == nmp.NMP_ID_IMAGE_ERASE {

		d.mtx.Lock()
		d.erases++
		d.mtx.Unlock()

		if d.eraseErr != nil {
			if d.eraseDisconnect {
				d.s.setOpen(false)
			}
			return nil, d.eraseErr
		}
		return rspFor(req, map[string]interface{}{"rc": d.eraseRc})
	}

	return d.fakeImgDev.handle(req)
}

func TestImageUpgradeRun(t *testing.T) {
	tests := []struct {
		name       string
		noErase    bool
		eraseRc    int
		eraseErr   error
		disconnect bool
		openErr    error
		wantErr    bool
		wantErases int
		wantOpens  int
	}{
		{name: "erase ok", wantErases: 1},
		{name: "no erase", noErase: true},
		{name: "erase rc", eraseRc: nmp.NMP_ERR_ENOMEM, wantErases: 1},
		{
			name:       "erase error while connected",
			eraseErr:   fmt.Errorf("erase failed"),
			wantErr:    true,
			wantErases: 1,
		},
		{
			name:       "erase disconnect reconnect ok",
			eraseErr:   fmt.Errorf("disconnected"),
			disconnect: true,
			wantErases: 1,
			wantOpens:  1,
		},
		{
			name:       "erase disconnect reconnect fails",
			eraseErr:   fmt.Errorf("disconnected"),
			disconnect: true,
			openErr:    fmt.Errorf("cannot connect"),
			wantErr:    true,
			wantErases: 1,
			wantOpens:  1,
		},
	}

	for _, tt := range tests {
		data := testImage(2000)
		dev := &imgUpgradeDev{
			eraseRc:         tt.eraseRc,
			eraseErr:        tt.eraseErr,
			eraseDisconnect: tt.disconnect,
		}
		s := newFakeSesn(dev.handle)
		s.openErr = tt.openErr
		dev.s = s

		var mtx sync.Mutex
		var last uint32
		c := NewImageUpgradeCmd()
		c.Data = data
		c.NoErase = tt.noErase
		c.MaxWinSz = 1
		c.ProgressCb = func(_ *ImageUploadCmd, r *nmp.ImageUploadRsp) {
			mtx.Lock()
			defer mtx.Unlock()
			last = r.Off
		}

		res, err := runCmd(t, c, s)
		if dev.erases != tt.wantErases {
			t.Errorf("%s: %d erases, want %d", tt.name, dev.erases,
				tt.wantErases)
		}
		if s.openCount != tt.wantOpens {
			t.Errorf("%s: %d opens, want %d", tt.name, s.openCount,
				tt.wantOpens)
		}
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: no error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}

		checkUploaded(t, &dev.fakeImgDev, data)
		ures := res.(*ImageUpgradeResult)
		if ures.Status() != 0 || ures.UploadRes == nil {
			t.Errorf("%s: status %d", tt.name, ures.Status())
		}
		if tt.noErase != (ures.EraseRes == nil) {
			t.Errorf("%s: erase result %+v", tt.name, ures.EraseRes)
		}
		if !tt.noErase && !tt.disconnect &&
			ures.EraseRes.Status() != tt.eraseRc {

			t.Errorf("%s: erase status %d", tt.name,
				ures.EraseRes.Status())
		}

		mtx.Lock()
		if int(last) != len(data) {
			t.Errorf("%s: last progress off %d", tt.name, last)
		}
		mtx.Unlock()
	}
}

// A non-zero rc from the device must end the upload and be reported via
// the result's status instead of making Run() loop forever.
func TestImageUploadRunDeviceError(t *testing.T) {
	data := testImage(5000)

	for _, winSz := range []int{1, 5} {
		for _, failOff := range []uint32{0, 1} {
			dev := &fakeImgDev{
				failRc:  nmp.NMP_ERR_ENOMEM,
				failOff: failOff,
			}
			s := newFakeSesn(dev.handle)
			s.mtu = 200

			var mtx sync.Mutex
			var cbRcs []int
			c := NewImageUploadCmd()
			c.Data = data
			c.MaxWinSz = winSz
			c.ProgressCb = func(_ *ImageUploadCmd, r *nmp.ImageUploadRsp) {
				mtx.Lock()
				defer mtx.Unlock()
				cbRcs = append(cbRcs, r.Rc)
			}

			res, err := runCmd(t, c, s)
			if err != nil {
				t.Fatalf("win %d fail at %d: %v", winSz, failOff, err)
			}
			if st := res.Status(); st != nmp.NMP_ERR_ENOMEM {
				t.Errorf("win %d fail at %d: status %d", winSz, failOff,
					st)
			}

			mtx.Lock()
			if len(cbRcs) == 0 || cbRcs[len(cbRcs)-1] != nmp.NMP_ERR_ENOMEM {
				t.Errorf("win %d fail at %d: progress rcs %v", winSz,
					failOff, cbRcs)
			}
			mtx.Unlock()
		}
	}
}

func TestImageUpgradeRunDeviceError(t *testing.T) {
	dev := &imgUpgradeDev{}
	dev.failRc = nmp.NMP_ERR_ENOMEM
	dev.failOff = 1
	s := newFakeSesn(dev.handle)
	dev.s = s

	c := NewImageUpgradeCmd()
	c.Data = testImage(3000)
	c.MaxWinSz = 1
	c.ProgressCb = func(*ImageUploadCmd, *nmp.ImageUploadRsp) {}

	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Status(); st != nmp.NMP_ERR_ENOMEM {
		t.Errorf("status %d", st)
	}
}

// Commands created by the constructors must work without setting
// MaxWinSz explicitly.
func TestImageUploadRunDefaultWindow(t *testing.T) {
	data := testImage(3000)

	dev := &fakeImgDev{}
	s := newFakeSesn(dev.handle)
	uc := NewImageUploadCmd()
	uc.Data = data
	if _, err := runCmd(t, uc, s); err != nil {
		t.Fatal(err)
	}
	checkUploaded(t, dev, data)

	udev := &imgUpgradeDev{}
	s = newFakeSesn(udev.handle)
	udev.s = s
	gc := NewImageUpgradeCmd()
	gc.Data = data
	gc.ProgressCb = func(*ImageUploadCmd, *nmp.ImageUploadRsp) {}
	if _, err := runCmd(t, gc, s); err != nil {
		t.Fatal(err)
	}
	checkUploaded(t, &udev.fakeImgDev, data)
}

func TestImageUpgradeRunNoProgressCb(t *testing.T) {
	data := testImage(3000)
	dev := &imgUpgradeDev{}
	s := newFakeSesn(dev.handle)
	dev.s = s

	c := NewImageUpgradeCmd()
	c.Data = data
	c.MaxWinSz = 1

	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Status(); st != 0 {
		t.Errorf("status %d", st)
	}
	checkUploaded(t, &dev.fakeImgDev, data)
}

// Responses delivered asynchronously, as with the transceiver based
// sessions: after the last chunk has been sent, Run() waits for the
// outstanding responses.
func TestImageUploadRunAsyncSession(t *testing.T) {
	data := testImage(3000)

	for _, winSz := range []int{1, 2, 5} {
		for _, drop := range []bool{false, true} {
			dev := &fakeImgDev{}
			if drop {
				dev.dropOffs = map[uint32]bool{0: true}
			}
			s := newFakeSesn(dev.handle)
			s.mtu = 200
			s.asyncDelay = time.Millisecond

			c := NewImageUploadCmd()
			c.Data = data
			c.MaxWinSz = winSz

			res, err := runCmd(t, c, s)
			if err != nil {
				t.Fatalf("win %d drop %v: %v", winSz, drop, err)
			}
			if st := res.Status(); st != 0 {
				t.Errorf("win %d drop %v: status %d", winSz, drop, st)
			}
			checkUploaded(t, dev, data)
		}
	}
}
