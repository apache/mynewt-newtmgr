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

type StatReadCmd struct {
	CmdBase
	Name string
}

func NewStatReadCmd() *StatReadCmd {
	return &StatReadCmd{
		CmdBase: NewCmdBase(),
	}
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
	return &StatListCmd{
		CmdBase: NewCmdBase(),
	}
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
