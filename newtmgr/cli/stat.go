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

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func statsListRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewStatListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.StatListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else if len(sres.Rsp.List) == 0 {
		fmt.Printf("stat groups: none\n")
	} else {
		groups := make([]string, len(sres.Rsp.List))
		for i, g := range sres.Rsp.List {
			groups[i] = g
		}
		sort.Strings(groups)

		fmt.Printf("stat groups:\n")
		for _, g := range groups {
			fmt.Printf("    %s\n", g)
		}
	}
}

func statsRunCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewStatReadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = args[0]

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.StatReadResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("stat group: %s\n", sres.Rsp.Name)
		if len(sres.Rsp.Fields) == 0 {
			fmt.Printf("    (empty)\n")
		} else {
			names := make([]string, 0, len(sres.Rsp.Fields))
			for k, _ := range sres.Rsp.Fields {
				names = append(names, k)
			}
			sort.Strings(names)

			for _, n := range names {
				fmt.Printf("%10d %s\n", sres.Rsp.Fields[n], n)
			}
		}
	}
}

func statsCmd() *cobra.Command {
	statsHelpText := "Read statistics for the specified stats_name from a device"
	statsCmd := &cobra.Command{
		Use:   "stat <stats_name> -c <conn_profile>",
		Short: "Read statistics from a device",
		Long:  statsHelpText,
		Run:   statsRunCmd,
	}

	ListCmd := &cobra.Command{
		Use:   "list -c <conn_profile>",
		Short: "Read the list of Stats names from a device",
		Run:   statsListRunCmd,
	}

	statsCmd.AddCommand(ListCmd)

	return statsCmd
}
