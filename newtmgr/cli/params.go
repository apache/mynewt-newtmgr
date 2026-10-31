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

package cli

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func readMcumgrParams(s sesn.Sesn) (*nmp.McumgrParamsRsp, error) {
	c := xact.NewMcumgrParamsCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		return nil, err
	}

	return res.(*xact.McumgrParamsResult).Rsp, nil
}

// Reads the request buffer parameters of the device.  Returns nil if the
// session cannot send requests larger than its MTU or if the device does not
// report its parameters.
func sesnMcumgrParams(s sesn.Sesn) *nmp.McumgrParamsRsp {
	// Only sessions which split a request over several transport packets
	// can make use of a buffer larger than the MTU.
	if s.MgmtProto() != sesn.MGMT_PROTO_NMP || !s.CoapIsTcp() {
		return nil
	}

	rsp, err := readMcumgrParams(s)
	if err != nil || rsp.Rc != 0 || int(rsp.BufSize) <= s.MtuOut() {
		log.Debugf("Not using mcumgr params; rsp=%+v err=%v", rsp, err)
		return nil
	}

	log.Debugf("Using mcumgr params; buf_size=%d buf_count=%d",
		rsp.BufSize, rsp.BufCount)

	return rsp
}

func paramsRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	rsp, err := readMcumgrParams(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	if rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", rsp.Rc)
		return
	}

	fmt.Printf("buf_size: %d\n", rsp.BufSize)
	fmt.Printf("buf_count: %d\n", rsp.BufCount)
}

func paramsCmd() *cobra.Command {
	paramsCmd := &cobra.Command{
		Use:   "params -c <conn_profile>",
		Short: "Read mcumgr buffer parameters from a device",
		Run:   paramsRunCmd,
	}

	return paramsCmd
}
