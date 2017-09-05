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

package adv

import (
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type CfgBle struct {
	// Mandatory
	OwnAddrType   bledefs.BleAddrType
	ConnMode      bledefs.BleAdvConnMode
	DiscMode      bledefs.BleAdvDiscMode
	ItvlMin       uint16
	ItvlMax       uint16
	ChannelMap    uint8
	FilterPolicy  bledefs.BleAdvFilterPolicy
	HighDutyCycle bool
	AdvFields     bledefs.BleAdvFields
	RspFields     bledefs.BleAdvFields
	SesnCfg       sesn.SesnCfg

	// Only required for direct advertisements
	PeerAddr *bledefs.BleAddr
}

type Cfg struct {
	Ble CfgBle
}

type Advertiser interface {
	Start(cfg Cfg) (sesn.Sesn, error)
	Stop() error
}

func NewCfg() Cfg {
	return Cfg{
		Ble: CfgBle{
			OwnAddrType: bledefs.BLE_ADDR_TYPE_RANDOM,
			ConnMode:    bledefs.BLE_ADV_CONN_MODE_UND,
			DiscMode:    bledefs.BLE_ADV_DISC_MODE_GEN,
		},
	}
}
