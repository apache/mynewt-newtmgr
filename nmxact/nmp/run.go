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

package nmp

import ()

//////////////////////////////////////////////////////////////////////////////
// $test                                                                    //
//////////////////////////////////////////////////////////////////////////////

type RunTestReq struct {
	NmpBase         `codec:"-"`
	Testname string `codec:"testname"`
	Token    string `codec:"token"`
}

type RunTestRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewRunTestReq() *RunTestReq {
	r := &RunTestReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_RUN, NMP_ID_RUN_TEST)
	return r
}

func (r *RunTestReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewRunTestRsp() *RunTestRsp {
	return &RunTestRsp{}
}

func (r *RunTestRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type RunListReq struct {
	NmpBase `codec:"-"`
}

type RunListRsp struct {
	NmpBase
	Rc   int      `codec:"rc"`
	List []string `codec:"run_list"`
}

func NewRunListReq() *RunListReq {
	r := &RunListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_RUN, NMP_ID_RUN_LIST)
	return r
}

func (r *RunListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewRunListRsp() *RunListRsp {
	return &RunListRsp{}
}

func (r *RunListRsp) Msg() *NmpMsg { return MsgFromReq(r) }
