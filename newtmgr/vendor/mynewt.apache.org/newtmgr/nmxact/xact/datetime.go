package xact

import (
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

///////////////////////////////////////////////////////////////////////////////
// $read                                                                     //
///////////////////////////////////////////////////////////////////////////////

type DateTimeReadCmd struct {
	CmdBase
}

func NewDateTimeReadCmd() *DateTimeReadCmd {
	return &DateTimeReadCmd{
		CmdBase: NewCmdBase(),
	}
}

type DateTimeReadResult struct {
	Rsp *nmp.DateTimeReadRsp
}

func newDateTimeReadResult() *DateTimeReadResult {
	return &DateTimeReadResult{}
}

func (r *DateTimeReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *DateTimeReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewDateTimeReadReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.DateTimeReadRsp)

	res := newDateTimeReadResult()
	res.Rsp = srsp
	return res, nil
}

///////////////////////////////////////////////////////////////////////////////
// $write                                                                    //
///////////////////////////////////////////////////////////////////////////////

type DateTimeWriteCmd struct {
	CmdBase
	DateTime string
}

func NewDateTimeWriteCmd() *DateTimeWriteCmd {
	return &DateTimeWriteCmd{
		CmdBase: NewCmdBase(),
	}
}

type DateTimeWriteResult struct {
	Rsp *nmp.DateTimeWriteRsp
}

func newDateTimeWriteResult() *DateTimeWriteResult {
	return &DateTimeWriteResult{}
}

func (r *DateTimeWriteResult) Status() int {
	return r.Rsp.Rc
}

func (c *DateTimeWriteCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewDateTimeWriteReq()
	r.DateTime = c.DateTime

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.DateTimeWriteRsp)

	res := newDateTimeWriteResult()
	res.Rsp = srsp
	return res, nil
}
