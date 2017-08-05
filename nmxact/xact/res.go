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

	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type GetResCmd struct {
	CmdBase
	Uri string
	Typ sesn.ResourceType
}

func NewGetResCmd() *GetResCmd {
	return &GetResCmd{
		CmdBase: NewCmdBase(),
	}
}

type GetResResult struct {
	Code  coap.COAPCode
	Value []byte
}

func newGetResResult() *GetResResult {
	return &GetResResult{}
}

func (r *GetResResult) Status() int {
	if r.Code == coap.Content {
		return 0
	} else {
		return int(r.Code)
	}
}

func (c *GetResCmd) Run(s sesn.Sesn) (Result, error) {
	status, val, err := sesn.GetResource(s, c.Typ, c.Uri, c.TxOptions())
	if err != nil {
		return nil, err
	}

	res := newGetResResult()
	res.Code = status
	res.Value = val
	return res, nil
}

type PutResCmd struct {
	CmdBase
	Uri   string
	Typ   sesn.ResourceType
	Value []byte
}

func NewPutResCmd() *PutResCmd {
	return &PutResCmd{
		CmdBase: NewCmdBase(),
	}
}

type PutResResult struct {
	Code coap.COAPCode
}

func newPutResResult() *PutResResult {
	return &PutResResult{}
}

func (r *PutResResult) Status() int {
	if r.Code == coap.Created || r.Code == coap.Changed {
		return 0
	} else {
		return int(r.Code)
	}
}

func (c *PutResCmd) Run(s sesn.Sesn) (Result, error) {
	status, err := sesn.PutResource(s, c.Typ, c.Uri, c.Value, c.TxOptions())
	if err != nil {
		return nil, err
	}

	res := newPutResResult()
	res.Code = status
	return res, nil
}
