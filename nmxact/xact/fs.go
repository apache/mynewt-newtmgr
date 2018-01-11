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
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $download                                                                //
//////////////////////////////////////////////////////////////////////////////

type FsDownloadProgressCb func(c *FsDownloadCmd, r *nmp.FsDownloadRsp)
type FsDownloadCmd struct {
	CmdBase
	Name       string
	ProgressCb FsDownloadProgressCb
}

func NewFsDownloadCmd() *FsDownloadCmd {
	return &FsDownloadCmd{
		CmdBase: NewCmdBase(),
	}
}

type FsDownloadResult struct {
	Rsps []*nmp.FsDownloadRsp
}

func newFsDownloadResult() *FsDownloadResult {
	return &FsDownloadResult{}
}

func (r *FsDownloadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

func (c *FsDownloadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newFsDownloadResult()
	off := 0

	for {
		r := nmp.NewFsDownloadReq()
		r.Name = c.Name
		r.Off = uint32(off)

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		frsp := rsp.(*nmp.FsDownloadRsp)
		res.Rsps = append(res.Rsps, frsp)

		if frsp.Rc != 0 {
			break
		}

		if c.ProgressCb != nil {
			c.ProgressCb(c, frsp)
		}

		if len(frsp.Data) == 0 {
			// Download complete.
			break
		}

		off = int(frsp.Off) + len(frsp.Data)
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////

type FsUploadProgressCb func(c *FsUploadCmd, r *nmp.FsUploadRsp)
type FsUploadCmd struct {
	CmdBase
	Name       string
	Data       []byte
	ProgressCb FsUploadProgressCb
}

func NewFsUploadCmd() *FsUploadCmd {
	return &FsUploadCmd{
		CmdBase: NewCmdBase(),
	}
}

type FsUploadResult struct {
	Rsps []*nmp.FsUploadRsp
}

func newFsUploadResult() *FsUploadResult {
	return &FsUploadResult{}
}

func (r *FsUploadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

func buildFsUploadReq(name string, fileSz int, chunk []byte,
	off int) *nmp.FsUploadReq {

	r := nmp.NewFsUploadReq()

	if r.Off == 0 {
		r.Len = uint32(fileSz)
	}
	r.Name = name
	r.Off = uint32(off)
	r.Data = chunk

	return r
}

func nextFsUploadReq(s sesn.Sesn, name string, data []byte, off int) (
	*nmp.FsUploadReq, error) {

	// First, build a request without data to determine how much data could
	// fit.
	empty := buildFsUploadReq(name, len(data), nil, off)
	emptyEnc, err := mgmt.EncodeMgmt(s, empty.Msg())
	if err != nil {
		return nil, err
	}

	room := s.MtuOut() - len(emptyEnc)
	if room <= 0 {
		return nil, fmt.Errorf("Cannot create file upload request; " +
			"MTU too low to fit any file data")
	}

	if off+room > len(data) {
		// Final chunk.
		room = len(data) - off
	}

	// Assume all the unused space can hold file data.  This assumption may not
	// be valid for some encodings (e.g., CBOR uses variable length fields to
	// encodes byte string lengths).
	r := buildFsUploadReq(name, len(data), data[off:off+room], off)
	enc, err := mgmt.EncodeMgmt(s, r.Msg())
	if err != nil {
		return nil, err
	}

	oversize := len(enc) - s.MtuOut()
	if oversize > 0 {
		// Request too big.  Reduce the amount of file data.
		r = buildFsUploadReq(name, len(data), data[off:off+room-oversize], off)
	}

	return r, nil
}

func (c *FsUploadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newFsUploadResult()

	for off := 0; off < len(c.Data); {
		r, err := nextFsUploadReq(s, c.Name, c.Data, off)
		if err != nil {
			return nil, err
		}

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		crsp := rsp.(*nmp.FsUploadRsp)

		off = int(crsp.Off)

		if c.ProgressCb != nil {
			c.ProgressCb(c, crsp)
		}

		res.Rsps = append(res.Rsps, crsp)
		if crsp.Rc != 0 {
			break
		}
	}

	return res, nil
}
