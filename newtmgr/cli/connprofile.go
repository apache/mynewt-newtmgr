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

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/config"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"

	"github.com/spf13/cobra"
)

func connProfileAddCmd(cmd *cobra.Command, args []string) {
	cpm := config.GlobalConnProfileMgr()

	// Connection Profile name required
	if len(args) == 0 {
		nmUsage(cmd, util.NewNewtError("Need connection profile name"))
	}

	name := args[0]
	cp := config.NewConnProfile()
	cp.Name = name
	cp.Type = config.CONN_TYPE_NONE

	for _, vdef := range args[1:] {
		s := strings.SplitN(vdef, "=", 2)
		switch s[0] {
		case "type":
			var err error
			cp.Type, err = config.ConnTypeFromString(s[1])
			if err != nil {
				nmUsage(cmd, err)
			}
		case "connstring":
			cp.ConnString = s[1]
		default:
			nmUsage(cmd, util.NewNewtError("Unknown variable "+s[0]))
		}
	}

	// Check that a type is specified.

	if cp.Type == config.CONN_TYPE_NONE {
		nmUsage(cmd, util.NewNewtError("Must specify a connection type"))
	}

	if err := cpm.AddConnProfile(cp); err != nil {
		nmUsage(cmd, err)
	}

	fmt.Printf("Connection profile %s successfully added\n", name)
}

func connProfileShowCmd(cmd *cobra.Command, args []string) {
	cpm := config.GlobalConnProfileMgr()

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	cpList, err := cpm.GetConnProfileList()
	if err != nil {
		nmUsage(cmd, err)
	}

	found := false
	for _, cp := range cpList {
		// Print out the connection profile, if name is "" or name
		// matches cp.Name
		if name != "" && cp.Name != name {
			continue
		}

		if !found {
			found = true
			fmt.Printf("Connection profiles: \n")
		}
		fmt.Printf("  %s: type=%s, connstring='%s'\n",
			cp.Name, config.ConnTypeToString(cp.Type), cp.ConnString)
	}

	if !found {
		if name == "" {
			fmt.Printf("No connection profiles found!\n")
		} else {
			fmt.Printf("No connection profiles found matching %s\n", name)
		}
	}
}

func connProfileDelCmd(cmd *cobra.Command, args []string) {
	cpm := config.GlobalConnProfileMgr()

	// Connection Profile name required
	if len(args) == 0 {
		nmUsage(cmd, util.NewNewtError("Need connection profile name"))
	}

	name := args[0]
	if err := cpm.DeleteConnProfile(name); err != nil {
		nmUsage(cmd, err)
	}

	fmt.Printf("Connection profile %s successfully deleted.\n", name)
}

func connProfileCmd() *cobra.Command {
	cpCmd := &cobra.Command{
		Use:   "conn",
		Short: "Manage " + nmutil.ToolInfo.ShortName + " connection profiles",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	addCmd := &cobra.Command{
		Use:   "add <conn_profile> <varname=value ...> ",
		Short: "Add a " + nmutil.ToolInfo.ShortName + " connection profile",
		Run:   connProfileAddCmd,
	}
	cpCmd.AddCommand(addCmd)

	deleCmd := &cobra.Command{
		Use:   "delete <conn_profile>",
		Short: "Delete a " + nmutil.ToolInfo.ShortName + " connection profile",
		Run:   connProfileDelCmd,
	}
	cpCmd.AddCommand(deleCmd)

	connShowHelpText := "Show information for the conn_profile connection "
	connShowHelpText += "profile or for all\nconnection profiles "
	connShowHelpText += "if conn_profile is not specified.\n"

	showCmd := &cobra.Command{
		Use:   "show [conn_profile]",
		Short: "Show " + nmutil.ToolInfo.ShortName + " connection profiles",
		Long:  connShowHelpText,
		Run:   connProfileShowCmd,
	}
	cpCmd.AddCommand(showCmd)

	return cpCmd
}
