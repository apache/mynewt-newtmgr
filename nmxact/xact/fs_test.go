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
	"fmt"
	"sync"
	"testing"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

func TestNextFsUploadReqMtuSweep(t *testing.T) {
	data := testImage(3000)
	name := "/cfg/some/file.bin"

	for _, pv := range protoVariants {
		for mtu := 0; mtu <= 1024; mtu++ {
			if mtu > 300 && mtu%5 != 0 {
				continue
			}
			s := newProtoSesn(pv, mtu, nil)

			for _, off := range []int{0, 1, 1000, len(data) - 3,
				len(data) - 1} {

				pfx := fmt.Sprintf("%s mtu=%d off=%d", pv.name, mtu, off)

				r, err := nextFsUploadReq(s, name, data, off)
				if err != nil {
					if mtu > 100 {
						t.Errorf("%s: unexpected error: %v", pfx, err)
					}
					continue
				}

				if l := encLen(t, s, r.Msg()); l > mtu {
					t.Errorf("%s: request len %d > mtu", pfx, l)
				}
				if len(r.Data) == 0 {
					t.Errorf("%s: empty chunk", pfx)
				}
				if int(r.Off) != off {
					t.Errorf("%s: off %d", pfx, r.Off)
				}
				if off+len(r.Data) > len(data) ||
					!bytes.Equal(r.Data, data[off:off+len(r.Data)]) {

					t.Errorf("%s: wrong chunk data", pfx)
				}

				// Name and length are mandatory (89f487a).
				if r.Name != name || int(r.Len) != len(data) {
					t.Errorf("%s: name %q len %d", pfx, r.Name, r.Len)
				}
			}
		}
	}
}

// The final chunk must not be padded (cdc971c).
func TestNextFsUploadReqFinalChunk(t *testing.T) {
	data := testImage(1000)

	for _, pv := range protoVariants {
		s := newProtoSesn(pv, 512, nil)

		r, err := nextFsUploadReq(s, "f", data, len(data)-7)
		if err != nil {
			t.Fatalf("%s: %v", pv.name, err)
		}
		if !bytes.Equal(r.Data, data[len(data)-7:]) {
			t.Errorf("%s: final chunk %d bytes, want 7", pv.name,
				len(r.Data))
		}

		// Whole file fits a single request.
		r, err = nextFsUploadReq(s, "f", data[:100], 0)
		if err != nil {
			t.Fatalf("%s: %v", pv.name, err)
		}
		if !bytes.Equal(r.Data, data[:100]) {
			t.Errorf("%s: single chunk %d bytes, want 100", pv.name,
				len(r.Data))
		}
	}
}

func TestNextFsUploadReqFields(t *testing.T) {
	data := testImage(1000)
	s := newFakeSesn(nil)

	for _, off := range []int{0, 500} {
		r, err := nextFsUploadReq(s, "/a/b", data, off)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := nmp.EncodeNmpPlain(r.Msg())
		if err != nil {
			t.Fatal(err)
		}
		fields, err := decodeCborMap(plain[nmp.NMP_HDR_SIZE:])
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"name", "len", "off", "data"} {
			if _, ok := fields[k]; !ok {
				t.Errorf("off %d: field %q missing: %v", off, k, fields)
			}
		}
	}
}

// fakeFsDev emulates the file system handlers of a device.
type fakeFsDev struct {
	mtx sync.Mutex

	files map[string][]byte

	// Upload state.
	upName string
	upLen  int

	// Maximum amount of data per download response.
	chunk int

	// If non-zero, requests with an offset of at least failOff are
	// answered with this rc.
	failRc  int
	failOff uint32

	// Number of requests handled.
	reqs int
}

func newFakeFsDev() *fakeFsDev {
	return &fakeFsDev{
		files: map[string][]byte{},
		chunk: 100,
	}
}

func (d *fakeFsDev) handle(req fakeReq) (nmp.NmpRsp, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.reqs++

	switch r := req.Body.(type) {
	case *nmp.FsUploadReq:
		if d.failRc != 0 && r.Off >= d.failOff {
			return &nmp.FsUploadRsp{Rc: d.failRc}, nil
		}
		if r.Off == 0 {
			d.upName = r.Name
			d.upLen = int(r.Len)
			d.files[r.Name] = nil
		}
		f := d.files[r.Name]
		if int(r.Off) == len(f) {
			f = append(f, r.Data...)
			d.files[r.Name] = f
		}
		return &nmp.FsUploadRsp{Off: uint32(len(f))}, nil

	case *nmp.FsDownloadReq:
		if d.failRc != 0 && r.Off >= d.failOff {
			return &nmp.FsDownloadRsp{Rc: d.failRc}, nil
		}
		f, ok := d.files[r.Name]
		if !ok {
			return &nmp.FsDownloadRsp{Rc: nmp.NMP_ERR_ENOENT}, nil
		}
		off := int(r.Off)
		if off > len(f) {
			return &nmp.FsDownloadRsp{Rc: nmp.NMP_ERR_EINVAL}, nil
		}
		end := off + d.chunk
		if end > len(f) {
			end = len(f)
		}
		rsp := &nmp.FsDownloadRsp{
			Off:  r.Off,
			Data: append([]byte{}, f[off:end]...),
		}
		if off == 0 {
			rsp.Len = uint32(len(f))
		}
		return rsp, nil

	default:
		return nil, fmt.Errorf("unexpected request %T", req.Body)
	}
}

func (d *fakeFsDev) file(name string) []byte {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	return d.files[name]
}

func TestFsUploadRun(t *testing.T) {
	for _, pv := range protoVariants {
		for _, sz := range []int{1, 100, 3000} {
			data := testImage(sz)
			dev := newFakeFsDev()
			s := newProtoSesn(pv, 128, dev.handle)

			var offs []uint32
			c := NewFsUploadCmd()
			c.Name = "/f"
			c.Data = data
			c.ProgressCb = func(_ *FsUploadCmd, r *nmp.FsUploadRsp) {
				offs = append(offs, r.Off)
			}

			res, err := runCmd(t, c, s)
			if err != nil {
				t.Fatalf("%s sz %d: %v", pv.name, sz, err)
			}
			if st := res.Status(); st != 0 {
				t.Errorf("%s sz %d: status %d", pv.name, sz, st)
			}
			if !bytes.Equal(dev.file("/f"), data) || dev.upLen != sz {
				t.Errorf("%s sz %d: device has %d bytes, len %d", pv.name,
					sz, len(dev.file("/f")), dev.upLen)
			}

			reqs := s.Requests()
			if len(offs) != len(reqs) ||
				int(offs[len(offs)-1]) != len(data) {

				t.Errorf("%s sz %d: progress %v for %d requests",
					pv.name, sz, offs, len(reqs))
			}
			for i, r := range reqs {
				if r.EncLen > s.mtu {
					t.Errorf("%s sz %d: req %d len %d > mtu", pv.name, sz,
						i, r.EncLen)
				}
			}
		}
	}
}

// The next request uses the offset reported by the device.
func TestFsUploadRunFollowsDeviceOff(t *testing.T) {
	data := testImage(1000)
	dev := newFakeFsDev()
	s := newFakeSesn(nil)
	s.mtu = 128

	first := true
	s.handler = func(req fakeReq) (nmp.NmpRsp, error) {
		r := req.Body.(*nmp.FsUploadReq)
		if first && r.Off > 0 {
			// Pretend the device lost the previous chunk once.
			first = false
			return &nmp.FsUploadRsp{Off: 0}, nil
		}
		return dev.handle(req)
	}

	c := NewFsUploadCmd()
	c.Name = "/f"
	c.Data = data
	if _, err := runCmd(t, c, s); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dev.file("/f"), data) {
		t.Errorf("device has %d bytes", len(dev.file("/f")))
	}

	reqs := s.Requests()
	if len(reqs) < 3 {
		t.Fatalf("%d requests", len(reqs))
	}
	if reqs[2].Body.(*nmp.FsUploadReq).Off != 0 {
		t.Errorf("request after rewind has off %d",
			reqs[2].Body.(*nmp.FsUploadReq).Off)
	}
}

func TestFsUploadRunDeviceError(t *testing.T) {
	dev := newFakeFsDev()
	dev.failRc = nmp.NMP_ERR_ENOMEM
	dev.failOff = 1
	s := newFakeSesn(dev.handle)
	s.mtu = 128

	c := NewFsUploadCmd()
	c.Name = "/f"
	c.Data = testImage(1000)

	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Status(); st != nmp.NMP_ERR_ENOMEM {
		t.Errorf("status %d", st)
	}
	if n := len(s.Requests()); n != 2 {
		t.Errorf("%d requests, want 2", n)
	}
}

func TestFsUploadRunTxError(t *testing.T) {
	s := newFakeSesn(func(fakeReq) (nmp.NmpRsp, error) {
		return nil, fmt.Errorf("tx failed")
	})

	c := NewFsUploadCmd()
	c.Name = "/f"
	c.Data = testImage(10)
	if _, err := runCmd(t, c, s); err == nil {
		t.Errorf("no error")
	}
}

func TestFsUploadRunMtuTooSmall(t *testing.T) {
	dev := newFakeFsDev()
	s := newFakeSesn(dev.handle)
	s.mtu = 10

	c := NewFsUploadCmd()
	c.Name = "/f"
	c.Data = testImage(10)
	if _, err := runCmd(t, c, s); err == nil {
		t.Errorf("no error")
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d requests", n)
	}
}

func TestFsDownloadRun(t *testing.T) {
	tests := []struct {
		name  string
		sz    int
		chunk int
	}{
		{"empty", 0, 100},
		{"one chunk", 50, 100},
		{"exact chunks", 300, 100},
		{"many chunks", 1234, 100},
	}

	for _, tt := range tests {
		data := testImage(tt.sz)
		dev := newFakeFsDev()
		dev.chunk = tt.chunk
		dev.files["/f"] = data
		s := newFakeSesn(dev.handle)

		var got []byte
		c := NewFsDownloadCmd()
		c.Name = "/f"
		c.ProgressCb = func(_ *FsDownloadCmd, r *nmp.FsDownloadRsp) {
			got = append(got, r.Data...)
		}

		res, err := runCmd(t, c, s)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if st := res.Status(); st != 0 {
			t.Errorf("%s: status %d", tt.name, st)
		}

		// The download continues until the device returns no data
		// (e4c84d1).
		if !bytes.Equal(got, data) {
			t.Errorf("%s: got %d bytes, want %d", tt.name, len(got),
				len(data))
		}
		var all []byte
		for _, r := range res.(*FsDownloadResult).Rsps {
			all = append(all, r.Data...)
		}
		if !bytes.Equal(all, data) {
			t.Errorf("%s: result has %d bytes, want %d", tt.name,
				len(all), len(data))
		}

		wantReqs := (tt.sz+tt.chunk-1)/tt.chunk + 1
		reqs := s.Requests()
		if len(reqs) != wantReqs {
			t.Errorf("%s: %d requests, want %d", tt.name, len(reqs),
				wantReqs)
		}
		for i, r := range reqs {
			wantOff := i * tt.chunk
			if wantOff > tt.sz {
				wantOff = tt.sz
			}
			dr := r.Body.(*nmp.FsDownloadReq)
			if dr.Name != "/f" || int(dr.Off) != wantOff {
				t.Errorf("%s: req %d: name %q off %d", tt.name, i,
					dr.Name, dr.Off)
			}
			if r.Hdr.Op != nmp.NMP_OP_READ ||
				r.Hdr.Group != nmp.NMP_GROUP_FS ||
				r.Hdr.Id != nmp.NMP_ID_FS_FILE {

				t.Errorf("%s: req %d: hdr %+v", tt.name, i, r.Hdr)
			}
		}
	}
}

func TestFsDownloadRunDeviceError(t *testing.T) {
	for _, failOff := range []uint32{0, 100} {
		dev := newFakeFsDev()
		dev.files["/f"] = testImage(1000)
		dev.failRc = nmp.NMP_ERR_EINVAL
		dev.failOff = failOff
		s := newFakeSesn(dev.handle)

		var cbs int
		c := NewFsDownloadCmd()
		c.Name = "/f"
		c.ProgressCb = func(*FsDownloadCmd, *nmp.FsDownloadRsp) { cbs++ }

		res, err := runCmd(t, c, s)
		if err != nil {
			t.Fatal(err)
		}
		if st := res.Status(); st != nmp.NMP_ERR_EINVAL {
			t.Errorf("fail at %d: status %d", failOff, st)
		}
		want := int(failOff)/dev.chunk + 1
		if n := len(s.Requests()); n != want {
			t.Errorf("fail at %d: %d requests, want %d", failOff, n, want)
		}
		// No progress callback for the error response.
		if cbs != want-1 {
			t.Errorf("fail at %d: %d callbacks", failOff, cbs)
		}
	}

	// Missing file.
	s := newFakeSesn(newFakeFsDev().handle)
	c := NewFsDownloadCmd()
	c.Name = "/nope"
	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Status(); st != nmp.NMP_ERR_ENOENT {
		t.Errorf("missing file: status %d", st)
	}
}

func TestFsDownloadRunTxError(t *testing.T) {
	s := newFakeSesn(func(fakeReq) (nmp.NmpRsp, error) {
		return nil, fmt.Errorf("tx failed")
	})

	c := NewFsDownloadCmd()
	c.Name = "/f"
	if _, err := runCmd(t, c, s); err == nil {
		t.Errorf("no error")
	}
}

// Status() of a result without responses (e.g. uploading an empty file,
// which sends no request) must not panic.
func TestMultiRspResultStatusEmpty(t *testing.T) {
	results := []Result{
		&FsDownloadResult{},
		&FsUploadResult{},
		&CoreLoadResult{},
		&ImageUploadResult{},
		&LogShowFullResult{},
	}
	for _, r := range results {
		if st := r.Status(); st != nmp.NMP_ERR_EUNKNOWN {
			t.Errorf("%T: status %d", r, st)
		}
	}

	s := newFakeSesn(newFakeFsDev().handle)
	c := NewFsUploadCmd()
	c.Name = "/f"
	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if st := res.Status(); st != nmp.NMP_ERR_EUNKNOWN {
		t.Errorf("empty upload: status %d", st)
	}
}
