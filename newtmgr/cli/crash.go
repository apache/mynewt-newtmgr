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
	"strings"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func crashRunCmd(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		nmUsage(cmd, nil)
	}

	ct, err := xact.CrashTypeFromString(args[0])
	if err != nil {
		nmUsage(cmd, util.ChildNewtError(err))
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewCrashCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.CrashType = ct

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.CrashResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("Done\n")
	}
}

func crashCmd() *cobra.Command {
	crashEx := "   " + nmutil.ToolInfo.ExeName + " -c olimex crash div0\n"

	namesStr := strings.Join(xact.CrashTypeNames(), "|")
	crashCmd := &cobra.Command{
		Use:     "crash <" + namesStr + "> -c <conn_profiles>",
		Short:   "Send a crash command to a device",
		Example: crashEx,
		Run:     crashRunCmd,
	}

	return crashCmd
}
