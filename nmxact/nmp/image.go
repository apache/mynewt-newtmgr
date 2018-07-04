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
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////

type ImageUploadReq struct {
	NmpBase         `codec:"-"`
	Off      uint32 `codec:"off"`
	Len      uint32 `codec:"len,omitempty"`
	DataSha  []byte `codec:"sha,omitempty"`
	Data     []byte `codec:"data"`
}

type ImageUploadRsp struct {
	NmpBase
	Rc  int    `codec:"rc"`
	Off uint32 `codec:"off"`
}

func NewImageUploadReq() *ImageUploadReq {
	r := &ImageUploadReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_UPLOAD)
	return r
}

func (r *ImageUploadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewImageUploadRsp() *ImageUploadRsp {
	return &ImageUploadRsp{}
}

func (r *ImageUploadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $state                                                                   //
//////////////////////////////////////////////////////////////////////////////

type SplitStatus int

const (
	NOT_APPLICABLE SplitStatus = iota
	NOT_MATCHING
	MATCHING
)

/* returns the enum as a string */
func (sm SplitStatus) String() string {
	names := map[SplitStatus]string{
		NOT_APPLICABLE: "N/A",
		NOT_MATCHING:   "non-matching",
		MATCHING:       "matching",
	}

	str := names[sm]
	if str == "" {
		return "Unknown!"
	}
	return str
}

type ImageStateEntry struct {
	NmpBase
	Slot      int    `codec:"slot"`
	Version   string `codec:"version"`
	Hash      []byte `codec:"hash"`
	Bootable  bool   `codec:"bootable"`
	Pending   bool   `codec:"pending"`
	Confirmed bool   `codec:"confirmed"`
	Active    bool   `codec:"active"`
	Permanent bool   `codec:"permanent"`
}

type ImageStateReadReq struct {
	NmpBase `codec:"-"`
}

type ImageStateWriteReq struct {
	NmpBase        `codec:"-"`
	Hash    []byte `codec:"hash"`
	Confirm bool   `codec:"confirm"`
}

type ImageStateRsp struct {
	NmpBase
	Rc          int               `codec:"rc"`
	Images      []ImageStateEntry `codec:"images"`
	SplitStatus SplitStatus       `codec:"splitStatus"`
}

func NewImageStateReadReq() *ImageStateReadReq {
	r := &ImageStateReadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_STATE)
	return r
}

func (r *ImageStateReadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewImageStateWriteReq() *ImageStateWriteReq {
	r := &ImageStateWriteReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_STATE)
	return r
}

func (r *ImageStateWriteReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewImageStateRsp() *ImageStateRsp {
	return &ImageStateRsp{}
}

func (r *ImageStateRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $corelist                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreListReq struct {
	NmpBase `codec:"-"`
}

type CoreListRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewCoreListReq() *CoreListReq {
	r := &CoreListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELIST)
	return r
}

func (r *CoreListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewCoreListRsp() *CoreListRsp {
	return &CoreListRsp{}
}

func (r *CoreListRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $coreload                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreLoadReq struct {
	NmpBase    `codec:"-"`
	Off uint32 `codec:"off"`
}

type CoreLoadRsp struct {
	NmpBase
	Rc   int    `codec:"rc"`
	Off  uint32 `codec:"off"`
	Len  uint32 `codec:"len"`
	Data []byte `codec:"data"`
}

func NewCoreLoadReq() *CoreLoadReq {
	r := &CoreLoadReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELOAD)
	return r
}

func (r *CoreLoadReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewCoreLoadRsp() *CoreLoadRsp {
	return &CoreLoadRsp{}
}

func (r *CoreLoadRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $coreerase                                                               //
//////////////////////////////////////////////////////////////////////////////

type CoreEraseReq struct {
	NmpBase `codec:"-"`
}

type CoreEraseRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewCoreEraseReq() *CoreEraseReq {
	r := &CoreEraseReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_CORELOAD)
	return r
}

func (r *CoreEraseReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewCoreEraseRsp() *CoreEraseRsp {
	return &CoreEraseRsp{}
}

func (r *CoreEraseRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $erase                                                                   //
//////////////////////////////////////////////////////////////////////////////

type ImageEraseReq struct {
	NmpBase `codec:"-"`
}

type ImageEraseRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewImageEraseReq() *ImageEraseReq {
	r := &ImageEraseReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_IMAGE, NMP_ID_IMAGE_ERASE)
	return r
}

func (r *ImageEraseReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewImageEraseRsp() *ImageEraseRsp {
	return &ImageEraseRsp{}
}

func (r *ImageEraseRsp) Msg() *NmpMsg { return MsgFromReq(r) }
