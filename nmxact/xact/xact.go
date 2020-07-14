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
	log "github.com/sirupsen/logrus"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func txReq(s sesn.Sesn, m *nmp.NmpMsg, c *CmdBase) (
	nmp.NmpRsp, error) {

	if c.abortErr != nil {
		return nil, c.abortErr
	}

	c.curNmpSeq = m.Hdr.Seq
	c.curSesn = s
	defer func() {
		c.curNmpSeq = 0
		c.curSesn = nil
	}()

	rsp, err := sesn.TxRxMgmt(s, m, c.TxOptions())
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

func txReqAsync(s sesn.Sesn, m *nmp.NmpMsg, c *CmdBase, ch chan nmp.NmpRsp, errc chan error) error {

	if c.abortErr != nil {
		return c.abortErr
	}

	c.curNmpSeq = m.Hdr.Seq
	c.curSesn = s
	defer func() {
		c.curNmpSeq = 0
		c.curSesn = nil
	}()

	err := sesn.TxRxMgmtAsync(s, m, c.TxOptions(), ch, errc)
	if err != nil {
		log.Debugf("error %v TxRxMgmtAsync sesn %v seq %d",
			err, c.curSesn, c.curNmpSeq)
		return err
	} else {
		return nil
	}
}
