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

package bll

import (
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type XportCfg struct {
	CtlrName    string
	OwnAddrType bledefs.BleAddrType
}

func NewXportCfg() XportCfg {
	return XportCfg{
		CtlrName: "default",
	}
}

type BllXport struct {
	cfg XportCfg
	hciIdx int
}

func NewBllXport(cfg XportCfg, hciIdx int) *BllXport {
	return &BllXport{
		cfg: cfg,
		hciIdx: hciIdx,
	}
}

func (bx *BllXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return nil, fmt.Errorf("Not Supported On Windows")
}

func (bx *BllXport) BuildBllSesn(cfg BllSesnCfg) (sesn.Sesn, error) {
	return nil, fmt.Errorf("Not Supported On Windows")
}

func (bx *BllXport) Start() error {
	return fmt.Errorf("Not Supported On Windows")
}

func (bx *BllXport) Stop() error {
	return fmt.Errorf("Not Supported On Windows")
}

func (bx *BllXport) Tx(data []byte) error {
	return fmt.Errorf("BllXport.Tx() not supported")
}
