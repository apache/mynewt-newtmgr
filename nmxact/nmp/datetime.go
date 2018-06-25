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

///////////////////////////////////////////////////////////////////////////////
// $read                                                                     //
///////////////////////////////////////////////////////////////////////////////

type DateTimeReadReq struct {
	NmpBase `codec:"-"`
}

type DateTimeReadRsp struct {
	NmpBase
	DateTime string `codec:"datetime"`
	Rc       int    `codec:"rc"`
}

func NewDateTimeReadReq() *DateTimeReadReq {
	r := &DateTimeReadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_DEFAULT, NMP_ID_DEF_DATETIME_STR)
	return r
}

func (r *DateTimeReadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewDateTimeReadRsp() *DateTimeReadRsp {
	return &DateTimeReadRsp{}
}

func (r *DateTimeReadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

///////////////////////////////////////////////////////////////////////////////
// $write                                                                    //
///////////////////////////////////////////////////////////////////////////////

type DateTimeWriteReq struct {
	NmpBase         `codec:"-"`
	DateTime string `codec:"datetime"`
}

type DateTimeWriteRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewDateTimeWriteReq() *DateTimeWriteReq {
	r := &DateTimeWriteReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_DEFAULT, NMP_ID_DEF_DATETIME_STR)
	return r
}

func (r *DateTimeWriteReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewDateTimeWriteRsp() *DateTimeWriteRsp {
	return &DateTimeWriteRsp{}
}

func (r *DateTimeWriteRsp) Msg() *NmpMsg { return MsgFromReq(r) }
