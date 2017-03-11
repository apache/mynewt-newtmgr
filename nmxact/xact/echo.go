package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

type EchoCmd struct {
	CmdBase
	Payload string
}

func NewEchoCmd() *EchoCmd {
	return &EchoCmd{}
}

type EchoResult struct {
	Rsp *nmp.EchoRsp
}

func newEchoResult() *EchoResult {
	return &EchoResult{}
}

func (r *EchoResult) Status() int {
	return r.Rsp.Rc
}

func (c *EchoCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewEchoReq()
	r.Payload = c.Payload

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.EchoRsp)

	res := newEchoResult()
	res.Rsp = srsp
	return res, nil
}
