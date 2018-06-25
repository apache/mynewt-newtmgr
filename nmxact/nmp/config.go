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

type ConfigReadReq struct {
	NmpBase        `codec:"-"`
	Name    string `codec:"name"`
}

type ConfigReadRsp struct {
	NmpBase
	Rc  int    `codec:"rc"`
	Val string `codec:"val"`
}

func NewConfigReadReq() *ConfigReadReq {
	r := &ConfigReadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_CONFIG, NMP_ID_CONFIG_VAL)
	return r
}

func (r *ConfigReadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewConfigReadRsp() *ConfigReadRsp {
	return &ConfigReadRsp{}
}

func (r *ConfigReadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $write                                                                   //
//////////////////////////////////////////////////////////////////////////////

type ConfigWriteReq struct {
	NmpBase     `codec:"-"`
	Name string `codec:"name,omitempty"`
	Val  string `codec:"val,omitempty"`
	Save bool   `codec:"save,omitempty"`
}

type ConfigWriteRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewConfigWriteReq() *ConfigWriteReq {
	r := &ConfigWriteReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_CONFIG, NMP_ID_CONFIG_VAL)
	return r
}

func (r *ConfigWriteReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewConfigWriteRsp() *ConfigWriteRsp {
	return &ConfigWriteRsp{}
}

func (r *ConfigWriteRsp) Msg() *NmpMsg { return MsgFromReq(r) }
