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
	"fmt"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"

	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type XportCfg struct {
	CtlrName string
}

func NewXportCfg() XportCfg {
	return XportCfg{
		CtlrName: "default",
	}
}

type BllXport struct {
	cfg XportCfg
}

func NewBllXport(cfg XportCfg) *BllXport {
	return &BllXport{
		cfg: cfg,
	}
}

func (bx *BllXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return nil, fmt.Errorf("BllXport.BuildSesn() not supported; " +
		"use BllXport.BuildBllSesn instead")
}

func (bx *BllXport) BuildBllSesn(cfg BllSesnCfg) (sesn.Sesn, error) {
	return NewBllSesn(cfg), nil
}

func (bx *BllXport) Start() error {
	d, err := dev.NewDevice(bx.cfg.CtlrName)
	if err != nil {
		return err
	}

	ble.SetDefaultDevice(d)

	return nil
}

func (bx *BllXport) Stop() error {
	if err := ble.Stop(); err != nil {
		return err
	}

	return nil
}

func (bx *BllXport) Tx(data []byte) error {
	return fmt.Errorf("BllXport.Tx() not supported")
}
