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

package config

import (
	"fmt"
	"strings"
	"time"

	"mynewt.apache.org/newt/nmxact/bledefs"
	"mynewt.apache.org/newt/nmxact/nmble"
	"mynewt.apache.org/newt/nmxact/sesn"
	"mynewt.apache.org/newt/util"
)

type BleConfig struct {
	PeerAddrType bledefs.BleAddrType
	PeerAddr     bledefs.BleAddr
	PeerName     string

	OwnAddrType bledefs.BleAddrType
	OwnAddr     bledefs.BleAddr

	BlehostdPath   string
	ControllerPath string
}

func NewBleConfig() *BleConfig {
	return &BleConfig{}
}

func einvalBleConnString(f string, args ...interface{}) error {
	suffix := fmt.Sprintf(f, args)
	return util.FmtNewtError("Invalid BLE connstring; %s", suffix)
}

func ParseBleConnString(cs string) (*BleConfig, error) {
	bc := NewBleConfig()

	parts := strings.Split(cs, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, einvalBleConnString("expected comma-separated "+
				"key=value pairs; no '=' in: %s", p)
		}

		k := kv[0]
		v := kv[1]

		var err error
		switch k {
		case "peer_addr_type":
			bc.PeerAddrType, err = bledefs.BleAddrTypeFromString(v)
			if err != nil {
				return nil, einvalBleConnString("Invalid peer_addr_type: %s", v)
			}
		case "peer_addr":
			bc.PeerAddr, err = bledefs.ParseBleAddr(v)
			if err != nil {
				return nil, einvalBleConnString("Invalid peer_addr; %s",
					err.Error())
			}
		case "peer_name":
			bc.PeerName = v
		case "own_addr_type":
			bc.OwnAddrType, err = bledefs.BleAddrTypeFromString(v)
			if err != nil {
				return nil, einvalBleConnString("Invalid own_addr_type: %s", v)
			}
		case "own_addr":
			bc.OwnAddr, err = bledefs.ParseBleAddr(v)
			if err != nil {
				return nil, einvalBleConnString("Invalid own_addr; %s",
					err.Error())
			}
		case "bhd_path":
			bc.BlehostdPath = v
		case "ctlr_path":
			bc.ControllerPath = v
		default:
			return nil, einvalBleConnString("Unrecognized key: %s", k)
		}
	}

	return bc, nil
}

func FillSesnCfg(bc *BleConfig, sc *sesn.SesnCfg) {
	sc.Ble.OwnAddrType = bc.OwnAddrType

	if bc.PeerName != "" {
		sc.Ble.PeerSpec = sesn.BlePeerSpecName(bc.PeerName)
	} else {
		sc.Ble.PeerSpec = sesn.BlePeerSpecDev(bledefs.BleDev{
			AddrType: bc.PeerAddrType,
			Addr:     bc.PeerAddr,
		})
	}

	// We don't need to stick around until a connection closes.
	sc.Ble.CloseTimeout = 10000 * time.Millisecond
}

func BuildBleXport(bc *BleConfig) (*nmble.BleXport, error) {
	params := nmble.NewXportCfg()
	params.SockPath = "/tmp/blehostd-uds"
	params.BlehostdPath = bc.BlehostdPath
	params.DevPath = bc.ControllerPath
	params.BlehostdAcceptTimeout = 2 * time.Second
	params.BlehostdRestart = false

	bx, err := nmble.NewBleXport(params)
	if err != nil {
		return nil, util.ChildNewtError(err)
	}

	return bx, nil
}
