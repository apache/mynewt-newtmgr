package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $read                                                                    //
//////////////////////////////////////////////////////////////////////////////

type StatReadCmd struct {
	CmdBase
	Name string
}

func NewStatReadCmd() *StatReadCmd {
	return &StatReadCmd{}
}

type StatReadResult struct {
	Rsp *nmp.StatReadRsp
}

func newStatReadResult() *StatReadResult {
	return &StatReadResult{}
}

func (r *StatReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *StatReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewStatReadReq()
	r.Name = c.Name

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.StatReadRsp)

	res := newStatReadResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type StatListCmd struct {
	CmdBase
}

func NewStatListCmd() *StatListCmd {
	return &StatListCmd{}
}

type StatListResult struct {
	Rsp *nmp.StatListRsp
}

func newStatListResult() *StatListResult {
	return &StatListResult{}
}

func (r *StatListResult) Status() int {
	return r.Rsp.Rc
}

func (c *StatListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewStatListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.StatListRsp)

	res := newStatListResult()
	res.Rsp = srsp
	return res, nil
}
