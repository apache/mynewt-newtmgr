// +build !windows

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

package bll

import (
	"time"

	"github.com/rigado/ble"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllSesnCfg struct {
	MgmtProto    sesn.MgmtProto
	AdvFilter    ble.AdvFilter
	PreferredMtu uint16
	ConnTimeout  time.Duration
	ConnTries    int
	WriteRsp     bool
	TxFilterCb   nmcoap.MsgFilter
	RxFilterCb   nmcoap.MsgFilter
}

func NewBllSesnCfg() BllSesnCfg {
	return BllSesnCfg{
		PreferredMtu: 512,
		ConnTimeout:  10 * time.Second,
		ConnTries:    3,
		WriteRsp:     false,
	}
}
