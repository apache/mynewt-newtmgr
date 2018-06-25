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
// $download                                                                //
//////////////////////////////////////////////////////////////////////////////

type FsDownloadReq struct {
	NmpBase     `codec:"-"`
	Name string `codec:"name"`
	Off  uint32 `codec:"off"`
}

type FsDownloadRsp struct {
	NmpBase
	Rc   int    `codec:"rc"`
	Off  uint32 `codec:"off"`
	Len  uint32 `codec:"len"`
	Data []byte `codec:"data"`
}

func NewFsDownloadReq() *FsDownloadReq {
	r := &FsDownloadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_FS, NMP_ID_FS_FILE)
	return r
}

func (r *FsDownloadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewFsDownloadRsp() *FsDownloadRsp {
	return &FsDownloadRsp{}
}

func (r *FsDownloadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////

type FsUploadReq struct {
	NmpBase     `codec:"-"`
	Name string `codec:"name"`
	Len  uint32 `codec:"len"`
	Off  uint32 `codec:"off"`
	Data []byte `codec:"data"`
}

type FsUploadRsp struct {
	NmpBase
	Rc  int    `codec:"rc"`
	Off uint32 `codec:"off"`
}

func NewFsUploadReq() *FsUploadReq {
	r := &FsUploadReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_FS, NMP_ID_FS_FILE)
	return r
}

func (r *FsUploadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewFsUploadRsp() *FsUploadRsp {
	return &FsUploadRsp{}
}

func (r *FsUploadRsp) Msg() *NmpMsg { return MsgFromReq(r) }
