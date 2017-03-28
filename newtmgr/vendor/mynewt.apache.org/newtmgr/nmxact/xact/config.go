package xact

import (
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $read                                                                    //
//////////////////////////////////////////////////////////////////////////////

type ConfigReadCmd struct {
	CmdBase
	Name string
}

func NewConfigReadCmd() *ConfigReadCmd {
	return &ConfigReadCmd{}
}

type ConfigReadResult struct {
	Rsp *nmp.ConfigReadRsp
}

func newConfigReadResult() *ConfigReadResult {
	return &ConfigReadResult{}
}

func (r *ConfigReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *ConfigReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewConfigReadReq()
	r.Name = c.Name

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ConfigReadRsp)

	res := newConfigReadResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $write                                                                   //
//////////////////////////////////////////////////////////////////////////////

type ConfigWriteCmd struct {
	CmdBase
	Name string
	Val  string
}

func NewConfigWriteCmd() *ConfigWriteCmd {
	return &ConfigWriteCmd{}
}

type ConfigWriteResult struct {
	Rsp *nmp.ConfigWriteRsp
}

func newConfigWriteResult() *ConfigWriteResult {
	return &ConfigWriteResult{}
}

func (r *ConfigWriteResult) Status() int {
	return r.Rsp.Rc
}

func (c *ConfigWriteCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewConfigWriteReq()
	r.Name = c.Name
	r.Val = c.Val

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ConfigWriteRsp)

	res := newConfigWriteResult()
	res.Rsp = srsp
	return res, nil
}
