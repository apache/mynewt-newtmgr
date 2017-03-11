package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type ResetCmd struct {
	CmdBase
	Payload string
}

func NewResetCmd() *ResetCmd {
	return &ResetCmd{}
}

type ResetResult struct {
	Rsp *nmp.ResetRsp
}

func newResetResult() *ResetResult {
	return &ResetResult{}
}

func (r *ResetResult) Status() int {
	return 0
}

func (c *ResetCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewResetReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ResetRsp)

	res := newResetResult()
	res.Rsp = srsp
	return res, nil
}
