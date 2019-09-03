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

package xact

import (
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type ResCmd struct {
	CmdBase
	MsgParams nmcoap.MsgParams
}

func NewResCmd() *ResCmd {
	return &ResCmd{
		CmdBase: NewCmdBase(),
	}
}

type ResResult struct {
	Rsp coap.Message
}

func newResResult() *ResResult {
	return &ResResult{}
}

func (r *ResResult) Status() int {
	switch r.Rsp.Code() {
	case coap.Created, coap.Deleted, coap.Valid, coap.Changed, coap.Content:
		return 0

	default:
		return int(r.Rsp.Code())
	}
}

func (c *ResCmd) Run(s sesn.Sesn) (Result, error) {
	var rsp coap.Message
	var err error

	rsp, err = sesn.TxRxCoap(s, c.MsgParams, c.txOptions)
	if err != nil {
		return nil, err
	}

	res := newResResult()
	res.Rsp = rsp

	return res, nil
}

type ResNoRxCmd struct {
	CmdBase
	MsgParams nmcoap.MsgParams
}

func NewResNoRxCmd() *ResNoRxCmd {
	return &ResNoRxCmd{
		CmdBase: NewCmdBase(),
	}
}

type ResNoRxResult struct {
}

func newResNoRxResult() *ResNoRxResult {
	return &ResNoRxResult{}
}

func (r *ResNoRxResult) Status() int {
	return 0
}

func (c *ResNoRxCmd) Run(s sesn.Sesn) (Result, error) {
	if err := sesn.TxCoap(s, c.MsgParams); err != nil {
		return nil, err
	}

	return newResNoRxResult(), nil
}
