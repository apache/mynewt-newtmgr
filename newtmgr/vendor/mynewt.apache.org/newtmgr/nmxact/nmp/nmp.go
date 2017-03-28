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

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newt/util"
)

const NMP_HDR_SIZE = 8

type NmpHdr struct {
	Op    uint8 /* 3 bits of opcode */
	Flags uint8
	Len   uint16
	Group uint16
	Seq   uint8
	Id    uint8
}

type NmpMsg struct {
	Hdr  NmpHdr
	Body interface{}
}

// Combine req + rsp.
type NmpReq interface {
	Hdr() *NmpHdr
	SetHdr(hdr *NmpHdr)

	Msg() *NmpMsg
}

type NmpRsp interface {
	Hdr() *NmpHdr
	SetHdr(msg *NmpHdr)

	Msg() *NmpMsg
}

type NmpBase struct {
	hdr NmpHdr `codec:"-"`
}

func (b *NmpBase) Hdr() *NmpHdr {
	return &b.hdr
}

func (b *NmpBase) SetHdr(h *NmpHdr) {
	b.hdr = *h
}

func MsgFromReq(r NmpReq) *NmpMsg {
	return &NmpMsg{
		*r.Hdr(),
		r,
	}
}

func NewNmpMsg() *NmpMsg {
	return &NmpMsg{}
}

func DecodeNmpHdr(data []byte) (*NmpHdr, error) {
	if len(data) < NMP_HDR_SIZE {
		return nil, util.NewNewtError(fmt.Sprintf(
			"Newtmgr request buffer too small %d bytes", len(data)))
	}

	hdr := &NmpHdr{}

	hdr.Op = uint8(data[0])
	hdr.Flags = uint8(data[1])
	hdr.Len = binary.BigEndian.Uint16(data[2:4])
	hdr.Group = binary.BigEndian.Uint16(data[4:6])
	hdr.Seq = uint8(data[6])
	hdr.Id = uint8(data[7])

	return hdr, nil
}

func (hdr *NmpHdr) Bytes() []byte {
	buf := make([]byte, 0, NMP_HDR_SIZE)

	buf = append(buf, byte(hdr.Op))
	buf = append(buf, byte(hdr.Flags))

	u16b := make([]byte, 2)
	binary.BigEndian.PutUint16(u16b, hdr.Len)
	buf = append(buf, u16b...)

	binary.BigEndian.PutUint16(u16b, hdr.Group)
	buf = append(buf, u16b...)

	buf = append(buf, byte(hdr.Seq))
	buf = append(buf, byte(hdr.Id))

	return buf
}

func BodyBytes(body interface{}) ([]byte, error) {
	data := make([]byte, 0)

	enc := codec.NewEncoderBytes(&data, new(codec.CborHandle))
	if err := enc.Encode(body); err != nil {
		return nil, fmt.Errorf("Failed to encode message %s", err.Error())
	}

	log.Debugf("Encoded %+v to:\n%s", body, hex.Dump(data))

	return data, nil
}

func EncodeNmpPlain(nmr *NmpMsg) ([]byte, error) {
	bb, err := BodyBytes(nmr.Body)
	if err != nil {
		return nil, err
	}

	nmr.Hdr.Len = uint16(len(bb))

	hb := nmr.Hdr.Bytes()
	data := append(hb, bb...)

	log.Debugf("Encoded:\n%s", hex.Dump(data))

	return data, nil
}

func fillNmpReq(req NmpReq, op uint8, group uint16, id uint8) {
	hdr := NmpHdr{
		Op:    op,
		Flags: 0,
		Len:   0,
		Group: group,
		Seq:   nmxutil.NextNmpSeq(),
		Id:    id,
	}

	req.SetHdr(&hdr)
}
