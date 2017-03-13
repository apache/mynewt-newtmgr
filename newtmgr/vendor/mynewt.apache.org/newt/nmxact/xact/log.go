package xact

import (
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/sesn"
)

//////////////////////////////////////////////////////////////////////////////
// $read                                                                    //
//////////////////////////////////////////////////////////////////////////////

type LogShowCmd struct {
	CmdBase
	Name      string
	Timestamp int64
	Index     uint32
}

func NewLogShowCmd() *LogShowCmd {
	return &LogShowCmd{}
}

type LogShowResult struct {
	Rsp *nmp.LogShowRsp
}

func newLogShowResult() *LogShowResult {
	return &LogShowResult{}
}

func (r *LogShowResult) Status() int {
	return r.Rsp.Rc
}

func (c *LogShowCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewLogShowReq()
	r.Name = c.Name
	r.Timestamp = c.Timestamp
	r.Index = c.Index

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.LogShowRsp)

	res := newLogShowResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type LogListCmd struct {
	CmdBase
}

func NewLogListCmd() *LogListCmd {
	return &LogListCmd{}
}

type LogListResult struct {
	Rsp *nmp.LogListRsp
}

func newLogListResult() *LogListResult {
	return &LogListResult{}
}

func (r *LogListResult) Status() int {
	return r.Rsp.Rc
}

func (c *LogListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewLogListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.LogListRsp)

	res := newLogListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $module list                                                             //
//////////////////////////////////////////////////////////////////////////////

type LogModuleListCmd struct {
	CmdBase
}

func NewLogModuleListCmd() *LogModuleListCmd {
	return &LogModuleListCmd{}
}

type LogModuleListResult struct {
	Rsp *nmp.LogModuleListRsp
}

func newLogModuleListResult() *LogModuleListResult {
	return &LogModuleListResult{}
}

func (r *LogModuleListResult) Status() int {
	return r.Rsp.Rc
}

func (c *LogModuleListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewLogModuleListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.LogModuleListRsp)

	res := newLogModuleListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $level list                                                              //
//////////////////////////////////////////////////////////////////////////////

type LogLevelListCmd struct {
	CmdBase
}

func NewLogLevelListCmd() *LogLevelListCmd {
	return &LogLevelListCmd{}
}

type LogLevelListResult struct {
	Rsp *nmp.LogLevelListRsp
}

func newLogLevelListResult() *LogLevelListResult {
	return &LogLevelListResult{}
}

func (r *LogLevelListResult) Status() int {
	return r.Rsp.Rc
}

func (c *LogLevelListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewLogLevelListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.LogLevelListRsp)

	res := newLogLevelListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $clear                                                                   //
//////////////////////////////////////////////////////////////////////////////

type LogClearCmd struct {
	CmdBase
}

func NewLogClearCmd() *LogClearCmd {
	return &LogClearCmd{}
}

type LogClearResult struct {
	Rsp *nmp.LogClearRsp
}

func newLogClearResult() *LogClearResult {
	return &LogClearResult{}
}

func (r *LogClearResult) Status() int {
	return r.Rsp.Rc
}

func (c *LogClearCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewLogClearReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.LogClearRsp)

	res := newLogClearResult()
	res.Rsp = srsp
	return res, nil
}
