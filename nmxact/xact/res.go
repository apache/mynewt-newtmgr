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
	Path       string
	Observe    int
	NotifyFunc sesn.GetNotifyCb
	StopSignal chan int
	Token      []byte
	Typ        sesn.ResourceType
}

func NewGetResCmd() *GetResCmd {
	return &GetResCmd{
		CmdBase: NewCmdBase(),
		Observe: -1,
	}
}

type GetResResult struct {
	Code  coap.COAPCode
	Value []byte
	Token []byte
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
	var status coap.COAPCode
	var val []byte
	var token []byte
	var err error

	if c.Observe != -1 {
		status, val, token, err = sesn.GetResourceObserve(s, c.Typ, c.Path, c.TxOptions(),
			c.NotifyFunc, c.StopSignal, c.Observe, c.Token)
	} else {
		status, val, err = sesn.GetResource(s, c.Typ, c.Path, c.TxOptions())
	}

	if err != nil {
		return nil, err
	}

	res := newGetResResult()
	res.Code = status
	res.Value = val
	res.Token = token

	return res, nil
}

type PutResCmd struct {
	CmdBase
	Path  string
	Typ   sesn.ResourceType
	Value []byte
}

func NewPutResCmd() *PutResCmd {
	return &PutResCmd{
		CmdBase: NewCmdBase(),
	}
}

type PutResResult struct {
	Code  coap.COAPCode
	Value []byte
}

func newPutResResult() *PutResResult {
	return &PutResResult{}
}

func (r *PutResResult) Status() int {
	if r.Code == coap.Created ||
		r.Code == coap.Changed ||
		r.Code == coap.Content {

		return 0
	} else {
		return int(r.Code)
	}
}

func (c *PutResCmd) Run(s sesn.Sesn) (Result, error) {
	status, r, err := sesn.PutResource(s, c.Typ, c.Path, c.Value, c.TxOptions())
	if err != nil {
		return nil, err
	}

	res := newPutResResult()
	res.Code = status
	res.Value = r
	return res, nil
}

type PostResCmd struct {
	CmdBase
	Path  string
	Typ   sesn.ResourceType
	Value []byte
}

func NewPostResCmd() *PostResCmd {
	return &PostResCmd{
		CmdBase: NewCmdBase(),
	}
}

type PostResResult struct {
	Code  coap.COAPCode
	Value []byte
}

func newPostResResult() *PostResResult {
	return &PostResResult{}
}

func (r *PostResResult) Status() int {
	if r.Code == coap.Created ||
		r.Code == coap.Changed ||
		r.Code == coap.Content {

		return 0
	} else {
		return int(r.Code)
	}
}

func (c *PostResCmd) Run(s sesn.Sesn) (Result, error) {
	status, r, err := sesn.PostResource(s, c.Typ, c.Path, c.Value, c.TxOptions())
	if err != nil {
		return nil, err
	}

	res := newPostResResult()
	res.Code = status
	res.Value = r
	return res, nil
}

type DeleteResCmd struct {
	CmdBase
	Path string
	Typ  sesn.ResourceType
	Value []byte
}

func NewDeleteResCmd() *DeleteResCmd {
	return &DeleteResCmd{
		CmdBase: NewCmdBase(),
	}
}

type DeleteResResult struct {
	Code  coap.COAPCode
	Value []byte
}

func newDeleteResResult() *DeleteResResult {
	return &DeleteResResult{}
}

func (r *DeleteResResult) Status() int {
	if r.Code == coap.Deleted {
		return 0
	} else {
		return int(r.Code)
	}
}

func (c *DeleteResCmd) Run(s sesn.Sesn) (Result, error) {
	status, val, err := sesn.DeleteResource(s, c.Typ, c.Path, c.Value, c.TxOptions())
	if err != nil {
		return nil, err
	}

	res := newDeleteResResult()
	res.Code = status
	res.Value = val
	return res, nil
}
