// +build windows

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

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/bll"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type BllConfig struct {
	CtlrName    string
	OwnAddrType bledefs.BleAddrType
	PeerId      string
	PeerName    string
	HciIdx	    int
}

func NewBllConfig() *BllConfig {
	return &BllConfig{}
}

func einvalBllConnString(f string, args ...interface{}) error {
	suffix := fmt.Sprintf(f, args)
	return util.FmtNewtError("Invalid BLE connstring; %s", suffix)
}

func ParseBllConnString(cs string) (*BllConfig, error) {

	return nil, util.FmtNewtError("Not Supported on Windows")
}

func BuildBllSesnCfg(bc *BllConfig) (bll.BllSesnCfg, error) {

	sc := bll.NewBllSesnCfg()

	return sc, util.FmtNewtError("Not Supported on Windows")
}
