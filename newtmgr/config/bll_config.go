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
	"strings"

	"github.com/go-ble/ble"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/bll"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
)

type BllConfig struct {
	CtlrName string
	PeerId   string
	PeerName string
}

func NewBllConfig() *BllConfig {
	return &BllConfig{}
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
		case "peer_id":
			bc.PeerId = v
		case "peer_name":
			bc.PeerName = v
		default:
			return nil, einvalBllConnString("Unrecognized key: %s", k)
		}
	}

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

	return sc, nil
}
