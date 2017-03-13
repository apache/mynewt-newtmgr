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

package bledefs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type BleAddrType int

const (
	BLE_ADDR_TYPE_PUBLIC  BleAddrType = 0
	BLE_ADDR_TYPE_RANDOM              = 1
	BLE_ADDR_TYPE_RPA_PUB             = 2
	BLE_ADDR_TYPE_RPA_RND             = 3
)

var BleAddrTypeStringMap = map[BleAddrType]string{
	BLE_ADDR_TYPE_PUBLIC:  "public",
	BLE_ADDR_TYPE_RANDOM:  "random",
	BLE_ADDR_TYPE_RPA_PUB: "rpa_pub",
	BLE_ADDR_TYPE_RPA_RND: "rpa_rnd",
}

type BleAddr struct {
	Bytes [6]byte
}

type BleDev struct {
	AddrType BleAddrType
	Addr     BleAddr
}

func (bd *BleDev) String() string {
	return fmt.Sprintf("%s,%s",
		BleAddrTypeToString(bd.AddrType),
		bd.Addr.String())
}

func BleAddrTypeToString(addrType BleAddrType) string {
	s := BleAddrTypeStringMap[addrType]
	if s == "" {
		panic(fmt.Sprintf("Invalid BleAddrType: %d", int(addrType)))
	}

	return s
}

func BleAddrTypeFromString(s string) (BleAddrType, error) {
	for addrType, name := range BleAddrTypeStringMap {
		if s == name {
			return addrType, nil
		}
	}

	return BleAddrType(0), fmt.Errorf("Invalid BleAddrType string: %s", s)
}

func (a BleAddrType) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAddrTypeToString(a))
}

func (a *BleAddrType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAddrTypeFromString(s)
	return err
}

func ParseBleAddr(s string) (BleAddr, error) {
	ba := BleAddr{}

	toks := strings.Split(strings.ToLower(s), ":")
	if len(toks) != 6 {
		return ba, fmt.Errorf("invalid BLE addr string: %s", s)
	}

	for i, t := range toks {
		u64, err := strconv.ParseUint(t, 16, 8)
		if err != nil {
			return ba, err
		}
		ba.Bytes[i] = byte(u64)
	}

	return ba, nil
}

func (ba *BleAddr) String() string {
	var buf bytes.Buffer
	buf.Grow(len(ba.Bytes) * 3)

	for i, b := range ba.Bytes {
		if i != 0 {
			buf.WriteString(":")
		}
		fmt.Fprintf(&buf, "%02x", b)
	}

	return buf.String()
}

func (ba *BleAddr) MarshalJSON() ([]byte, error) {
	return json.Marshal(ba.String())
}

func (ba *BleAddr) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var err error
	*ba, err = ParseBleAddr(s)
	if err != nil {
		return err
	}

	return nil
}
