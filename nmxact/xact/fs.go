package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
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
	return &FsDownloadCmd{}
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

		off = int(frsp.Off) + len(frsp.Data)
		if off >= int(frsp.Len) {
			break
		}
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
	return &FsUploadCmd{}
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

// Returns nil if the final request has already been sent.
func nextUploadChunk(data []byte, off int, mtu int) []byte {
	bytesLeft := len(data) - off
	if bytesLeft == 0 {
		return nil
	}

	chunkSz := 0
	if off == 0 {
		chunkSz = 64
	} else {
		chunkSz = mtu - 64
		if chunkSz > 200 {
			chunkSz = 200
		}
	}

	if chunkSz > bytesLeft {
		chunkSz = bytesLeft
	}
	return data[off : off+chunkSz]
}

func (c *FsUploadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newFsUploadResult()
	off := 0

	for {
		r := nmp.NewFsUploadReq()

		if off == 0 {
			r.Name = c.Name
			r.Len = uint32(len(c.Data))
		}

		r.Off = uint32(off)
		r.Data = nextUploadChunk(c.Data, off, s.MtuOut())
		if r.Data == nil {
			break
		}

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		frsp := rsp.(*nmp.FsUploadRsp)

		if c.ProgressCb != nil {
			c.ProgressCb(c, frsp)
		}

		res.Rsps = append(res.Rsps, frsp)
		if frsp.Rc != 0 {
			break
		}

		off = int(frsp.Off)
	}

	return res, nil
}
