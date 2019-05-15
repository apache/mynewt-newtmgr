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

	"github.com/spf13/cobra"
	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func shellExecCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewShellExecCmd()
	c.SetTxOptions(nmutil.TxOptions())

	if len(args) == 0 {
		nmUsage(cmd, nil)
	}

	c.Argv = args

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.ShellExecResult)
	fmt.Printf("status=%d\n", sres.Rsp.Rc)
	if len(sres.Rsp.O) > 0 {
		fmt.Printf("%s", sres.Rsp.O)
		if sres.Rsp.O[len(sres.Rsp.O)-1] != '\n' {
			fmt.Printf("\n")
		}
	}
}

func shellCmd() *cobra.Command {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Execute shell commands remotely",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	execCmd := &cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a shell command remotely",
		Run:   shellExecCmd,
	}

	shellCmd.AddCommand(execCmd)

	return shellCmd
}
