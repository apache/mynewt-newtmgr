package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////

type ImageUploadProgressFn func(c *ImageUploadCmd, r *nmp.ImageUploadRsp)
type ImageUploadCmd struct {
	CmdBase
	Data       []byte
	ProgressCb ImageUploadProgressFn
}

type ImageUploadResult struct {
	Rsps []*nmp.ImageUploadRsp
}

func NewImageUploadCmd() *ImageUploadCmd {
	return &ImageUploadCmd{}
}

func newImageUploadResult() *ImageUploadResult {
	return &ImageUploadResult{}
}

func (r *ImageUploadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

// Returns nil if the final request has already been sent.
func nextImageUploadChunk(data []byte, off int, mtu int) []byte {
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

func (c *ImageUploadCmd) Run(s sesn.Sesn) (Result, error) {
	off := 0
	res := newImageUploadResult()

	for {
		r := nmp.NewImageUploadReq()

		if off == 0 {
			r.Len = len(c.Data)
		}
		r.Off = off
		r.Data = nextImageUploadChunk(c.Data, off, s.MtuOut())
		if r.Data == nil {
			break
		}

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		irsp := rsp.(*nmp.ImageUploadRsp)

		off = irsp.Off

		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}

		res.Rsps = append(res.Rsps, irsp)
		if irsp.Rc != 0 {
			break
		}
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $state read                                                              //
//////////////////////////////////////////////////////////////////////////////

type ImageStateReadCmd struct {
	CmdBase
}

type ImageStateReadResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateReadCmd() *ImageStateReadCmd {
	return &ImageStateReadCmd{}
}

func newImageStateReadResult() *ImageStateReadResult {
	return &ImageStateReadResult{}
}

func (r *ImageStateReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateReadReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateReadResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $state write                                                             //
//////////////////////////////////////////////////////////////////////////////

type ImageStateWriteCmd struct {
	CmdBase
	Hash    []byte
	Confirm bool
}

type ImageStateWriteResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateWriteCmd() *ImageStateWriteCmd {
	return &ImageStateWriteCmd{}
}

func newImageStateWriteResult() *ImageStateWriteResult {
	return &ImageStateWriteResult{}
}

func (r *ImageStateWriteResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateWriteCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateWriteReq()
	r.Hash = c.Hash
	r.Confirm = c.Confirm

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateWriteResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $corelist                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreListCmd struct {
	CmdBase
}

type CoreListResult struct {
	Rsp *nmp.CoreListRsp
}

func NewCoreListCmd() *CoreListCmd {
	return &CoreListCmd{}
}

func newCoreListResult() *CoreListResult {
	return &CoreListResult{}
}

func (r *CoreListResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreListRsp)

	res := newCoreListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreload                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreLoadProgressFn func(c *CoreLoadCmd, r *nmp.CoreLoadRsp)
type CoreLoadCmd struct {
	CmdBase
	ProgressCb CoreLoadProgressFn
}

type CoreLoadResult struct {
	Rsps []*nmp.CoreLoadRsp
}

func NewCoreLoadCmd() *CoreLoadCmd {
	return &CoreLoadCmd{}
}

func newCoreLoadResult() *CoreLoadResult {
	return &CoreLoadResult{}
}

func (r *CoreLoadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

func (c *CoreLoadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newCoreLoadResult()
	off := 0

	for {
		r := nmp.NewCoreLoadReq()
		r.Off = uint32(off)

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		irsp := rsp.(*nmp.CoreLoadRsp)

		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}

		res.Rsps = append(res.Rsps, irsp)
		if irsp.Rc != 0 {
			break
		}

		if len(irsp.Data) == 0 {
			// Download complete.
			break
		}

		off = int(irsp.Off) + len(irsp.Data)
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreerase                                                               //
//////////////////////////////////////////////////////////////////////////////

type CoreEraseCmd struct {
	CmdBase
}

type CoreEraseResult struct {
	Rsp *nmp.CoreEraseRsp
}

func NewCoreEraseCmd() *CoreEraseCmd {
	return &CoreEraseCmd{}
}

func newCoreEraseResult() *CoreEraseResult {
	return &CoreEraseResult{}
}

func (r *CoreEraseResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreEraseCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreEraseReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreEraseRsp)

	res := newCoreEraseResult()
	res.Rsp = srsp
	return res, nil
}
