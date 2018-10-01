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
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func configRead(s sesn.Sesn, args []string) {
	c := xact.NewConfigReadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = args[0]

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.ConfigReadResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("Value: %s\n", sres.Rsp.Val)
	}
}

func configWrite(s sesn.Sesn, args []string) {
	c := xact.NewConfigWriteCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = args[0]
	c.Val = args[1]

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.ConfigWriteResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("Done\n")
	}
}

func configSave(s sesn.Sesn, args []string) {
	c := xact.NewConfigWriteCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Save = true

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.ConfigWriteResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("Done\n")
	}
}

func configRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	if len(args) == 1 {
		if args[0] == "save" {
			configSave(s, args)
		} else {
			configRead(s, args)
		}
	} else if len(args) >= 2 {
		configWrite(s, args)
	} else {
		nmUsage(cmd, nil)
	}
}

func configCmd() *cobra.Command {
	configCmdLongHelp := "Read or write a config value for <var-name> variable on " +
		"a device.\nSpecify a var-value to write a value to a device.\n" +
		"To persist existing configuration use 'save' as the var-name.\n"
	configEx := "    " + nmutil.ToolInfo.ExeName + " -c olimex config test/8\n"
	configEx += "    " + nmutil.ToolInfo.ExeName + " -c olimex config test/8 1\n"
	configEx += "    " + nmutil.ToolInfo.ExeName + " -c olimex config save\n"
	configCmd := &cobra.Command{
		Use:     "config <var-name> [var-value] -c <conn_profile>",
		Short:   "Read or write a config value on a device",
		Long:    configCmdLongHelp,
		Example: configEx,
		Run:     configRunCmd,
	}

	return configCmd
}
