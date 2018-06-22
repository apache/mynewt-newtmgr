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
// $read                                                                    //
//////////////////////////////////////////////////////////////////////////////

type StatReadReq struct {
	NmpBase     `codec:"-"`
	Name string `codec:"name"`
}

type StatReadRsp struct {
	NmpBase
	Rc     int                    `codec:"rc"`
	Name   string                 `codec:"name"`
	Group  string                 `codec:"group"`
	Fields map[string]interface{} `codec:"fields"`
}

func NewStatReadReq() *StatReadReq {
	r := &StatReadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_STAT, NMP_ID_STAT_READ)
	return r
}

func (r *StatReadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewStatReadRsp() *StatReadRsp {
	return &StatReadRsp{}
}

func (r *StatReadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type StatListReq struct {
	NmpBase `codec:"-"`
}

type StatListRsp struct {
	NmpBase
	Rc   int      `codec:"rc"`
	List []string `codec:"stat_list"`
}

func NewStatListReq() *StatListReq {
	r := &StatListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_STAT, NMP_ID_STAT_LIST)
	return r
}

func (r *StatListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewStatListRsp() *StatListRsp {
	return &StatListRsp{}
}

func (r *StatListRsp) Msg() *NmpMsg { return MsgFromReq(r) }
