package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type MempoolStatCmd struct {
	CmdBase
}

func NewMempoolStatCmd() *MempoolStatCmd {
	return &MempoolStatCmd{}
}

type MempoolStatResult struct {
	Rsp *nmp.MempoolStatRsp
}

func newMempoolStatResult() *MempoolStatResult {
	return &MempoolStatResult{}
}

func (r *MempoolStatResult) Status() int {
	return r.Rsp.Rc
}

func (c *MempoolStatCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewMempoolStatReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.MempoolStatRsp)

	res := newMempoolStatResult()
	res.Rsp = srsp
	return res, nil
}
