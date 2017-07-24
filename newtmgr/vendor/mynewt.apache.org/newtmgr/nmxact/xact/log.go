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
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
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
	return &LogShowCmd{
		CmdBase: NewCmdBase(),
	}
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
	return &LogListCmd{
		CmdBase: NewCmdBase(),
	}
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
	return &LogModuleListCmd{
		CmdBase: NewCmdBase(),
	}
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
	return &LogLevelListCmd{
		CmdBase: NewCmdBase(),
	}
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
	return &LogClearCmd{
		CmdBase: NewCmdBase(),
	}
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
