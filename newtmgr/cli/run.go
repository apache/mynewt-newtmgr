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
	"sort"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"mynewt.apache.org/newt/util"
)

func runTestCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewRunTestCmd()
	c.SetTxOptions(nmutil.TxOptions())

	if len(args) == 0 {
		c.Testname = "all"
	} else {
		c.Testname = args[0]
		if len(args) > 1 {
			c.Token = args[1]
		}
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.RunTestResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
		return
	}

	fmt.Printf("Done\n")
}

func runListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewRunListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.RunListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
		return
	}

	sort.Strings(sres.Rsp.List)
	fmt.Printf("available tests:\n")
	for _, n := range sres.Rsp.List {
		fmt.Printf("    %s\n", n)
	}
}

func runCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run procedures on remote device",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	runtestEx := "  newtmgr -c conn run test all 201612161220"

	runTestCmd := &cobra.Command{
		Use: "test [all | testname] [token]",
		Short: "Run commands on remote device - \"token\" output on log " +
			"messages",
		Example: runtestEx,
		Run:     runTestCmd,
	}
	runCmd.AddCommand(runTestCmd)

	runListCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered commands on remote device",
		Run:   runListCmd,
	}
	runCmd.AddCommand(runListCmd)

	return runCmd
}
