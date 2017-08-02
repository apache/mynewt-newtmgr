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

func getResRunCmd(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		nmUsage(cmd, nil)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	uri := args[1]

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewGetResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Uri = uri
	c.Typ = rt

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.GetResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %d\n", sres.Status())
	} else {
		fmt.Printf("%s: %+v\n", uri, sres.Value)
	}
}

func getResCmd() *cobra.Command {
	getResEx := "   newtmgr -c olimex getres public mynewt.value.0\n"

	getResCmd := &cobra.Command{
		Use:     "getres <type> <uri>",
		Short:   "Read a CoAP resource on a device",
		Example: getResEx,
		Run:     getResRunCmd,
	}

	return getResCmd
}
