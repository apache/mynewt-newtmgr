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

package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rigado/ble"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/bll"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type BllConfig struct {
	CtlrName    string
	OwnAddrType bledefs.BleAddrType
	PeerId      string
	PeerName    string

	// Connection timeout, in seconds.
	ConnTimeout float64

	HciIdx int
}

func NewBllConfig() *BllConfig {
	return &BllConfig{
		ConnTimeout: nmutil.Timeout,
	}
}

func einvalBllConnString(f string, args ...interface{}) error {
	suffix := fmt.Sprintf(f, args)
	return util.FmtNewtError("Invalid BLE connstring; %s", suffix)
}

func ParseBllConnString(cs string) (*BllConfig, error) {
	bc := NewBllConfig()

	if strings.TrimSpace(cs) == "" {
		return bc, nil
	}

	parts := strings.Split(cs, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, einvalBllConnString("expected comma-separated "+
				"key=value pairs; no '=' in: %s", p)
		}

		k := kv[0]
		v := kv[1]

		switch k {
		case "ctlr_name":
			bc.CtlrName = v
		case "own_addr_type":
			var err error
			bc.OwnAddrType, err = bledefs.BleAddrTypeFromString(v)
			if err != nil {
				return nil, einvalBleConnString("Invalid own_addr_type: %s", v)
			}
		case "peer_id":
			bc.PeerId = v
		case "peer_name":
			bc.PeerName = v
		case "conn_timeout":
			var err error
			bc.ConnTimeout, err = strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, einvalBleConnString("Invalid conn_timeout: %s", v)
			}

		default:
			return nil, einvalBllConnString("Unrecognized key: %s", k)
		}
	}

	bc.HciIdx = nmutil.HciIdx

	return bc, nil
}

func BuildBllSesnCfg(bc *BllConfig) (bll.BllSesnCfg, error) {
	if nmutil.DeviceName != "" {
		bc.PeerName = nmutil.DeviceName
	}

	sc := bll.NewBllSesnCfg()

	if bc.PeerName != "" {
		sc.AdvFilter = func(a ble.Advertisement) bool {
			return a.LocalName() == bc.PeerName
		}
	} else if bc.PeerId != "" {
		sc.AdvFilter = func(a ble.Advertisement) bool {
			return a.Addr().String() == bc.PeerId
		}
	} else {
		return sc, util.NewNewtError("bll session lacks a peer specifier")
	}

	sc.WriteRsp = nmutil.BleWriteRsp
	sc.ConnTimeout = time.Duration(bc.ConnTimeout*1000000000) * time.Nanosecond

	return sc, nil
}
