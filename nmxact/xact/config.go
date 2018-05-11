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

type ConfigReadCmd struct {
	CmdBase
	Name string
}

func NewConfigReadCmd() *ConfigReadCmd {
	return &ConfigReadCmd{
		CmdBase: NewCmdBase(),
	}
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
	Save bool
}

func NewConfigWriteCmd() *ConfigWriteCmd {
	return &ConfigWriteCmd{
		CmdBase: NewCmdBase(),
	}
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
	r.Save = c.Save

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ConfigWriteRsp)

	res := newConfigWriteResult()
	res.Rsp = srsp
	return res, nil
}
