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
// $test                                                                    //
//////////////////////////////////////////////////////////////////////////////

type RunTestCmd struct {
	CmdBase
	Testname string
	Token    string
}

func NewRunTestCmd() *RunTestCmd {
	return &RunTestCmd{
		CmdBase: NewCmdBase(),
	}
}

type RunTestResult struct {
	Rsp *nmp.RunTestRsp
}

func newRunTestResult() *RunTestResult {
	return &RunTestResult{}
}

func (r *RunTestResult) Status() int {
	return r.Rsp.Rc
}

func (c *RunTestCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewRunTestReq()
	r.Testname = c.Testname
	r.Token = c.Token

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.RunTestRsp)

	res := newRunTestResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type RunListCmd struct {
	CmdBase
}

func NewRunListCmd() *RunListCmd {
	return &RunListCmd{
		CmdBase: NewCmdBase(),
	}
}

type RunListResult struct {
	Rsp *nmp.RunListRsp
}

func newRunListResult() *RunListResult {
	return &RunListResult{}
}

func (r *RunListResult) Status() int {
	return r.Rsp.Rc
}

func (c *RunListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewRunListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.RunListRsp)

	res := newRunListResult()
	res.Rsp = srsp
	return res, nil
}
