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
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type Result interface {
	Status() int
}

type Cmd interface {
	// Transmits request and listens for response; blocking.
	Run(s sesn.Sesn) (Result, error)
	Abort() error

	TxOptions() sesn.TxOptions
	SetTxOptions(opt sesn.TxOptions)
}

type CmdBase struct {
	txOptions sesn.TxOptions
	curNmpSeq uint8
	curSesn   sesn.Sesn
	abortErr  error
}

func NewCmdBase() CmdBase {
	return CmdBase{
		txOptions: sesn.NewTxOptions(),
	}
}

func (c *CmdBase) TxOptions() sesn.TxOptions {
	return c.txOptions
}

func (c *CmdBase) SetTxOptions(opt sesn.TxOptions) {
	c.txOptions = opt
}

func (c *CmdBase) Abort() error {
	if c.curSesn != nil {
		if err := c.curSesn.AbortRx(c.curNmpSeq); err != nil {
			return err
		}
	}

	c.abortErr = fmt.Errorf("Command aborted")
	return nil
}
