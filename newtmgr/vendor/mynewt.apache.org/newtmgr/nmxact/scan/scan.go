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

package scan

import (
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type ScanPeer struct {
	HwId     string
	PeerSpec sesn.PeerSpec
}

func (p *ScanPeer) String() string {
	return fmt.Sprintf("%s, %s", p.PeerSpec.Ble.String(), p.HwId)
}

type ScanFn func(peer ScanPeer)

type CfgBle struct {
	ScanPred bledefs.BleAdvPredicate
}

type Cfg struct {
	// General configuration.
	ScanCb  ScanFn
	SesnCfg sesn.SesnCfg

	// Transport-specific configuration.
	Ble CfgBle
}

type Scanner interface {
	Start(cfg Cfg) error
	Stop() error

	// @return                      true if the specified device was found and
	//                                  forgetten;
	//                              false if the specified device is unknown.
	ForgetDevice(hwId string) bool

	ForgetAllDevices()
}

// Constructs a scan configuration suitable for discovery of OMP
// (Newtmgr-over-CoAP) Mynewt devices.
func BleOmpScanCfg(ScanCb ScanFn) Cfg {
	sc := sesn.NewSesnCfg()
	sc.MgmtProto = sesn.MGMT_PROTO_OMP
	sc.Ble.EncryptWhen = bledefs.BLE_ENCRYPT_AS_REQD
	sc.Ble.OwnAddrType = bledefs.BLE_ADDR_TYPE_RANDOM

	cfg := Cfg{
		ScanCb:  ScanCb,
		SesnCfg: sc,
		Ble: CfgBle{
			ScanPred: func(adv bledefs.BleAdvReport) bool {
				for _, u := range adv.Fields.Uuids16 {
					if u == bledefs.OmpSecSvcUuid {
						return true
					}
				}

				return false
			},
		},
	}
	return cfg
}
