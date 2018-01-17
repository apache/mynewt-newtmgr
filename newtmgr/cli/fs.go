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
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func fsDownloadRunCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	file, err := os.OpenFile(args[1], os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		nmUsage(cmd, util.FmtNewtError(
			"Cannot open file %s - %s", args[1], err.Error()))
	}
	defer file.Close()

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewFsDownloadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = args[0]
	c.ProgressCb = func(c *xact.FsDownloadCmd, rsp *nmp.FsDownloadRsp) {
		fmt.Printf("%d\n", rsp.Off)
		if _, err := file.Write(rsp.Data); err != nil {
			nmUsage(nil, util.ChildNewtError(err))
		}
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.FsDownloadResult)
	rsp := sres.Rsps[len(sres.Rsps)-1]
	if rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", rsp.Rc)
		return
	}

	fmt.Printf("Done\n")
}

func fsUploadRunCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	data, err := ioutil.ReadFile(args[0])
	if err != nil {
		nmUsage(cmd, util.ChildNewtError(err))
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewFsUploadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = args[1]
	c.Data = data
	c.ProgressCb = func(c *xact.FsUploadCmd, rsp *nmp.FsUploadRsp) {
		fmt.Printf("%d\n", rsp.Off)
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.FsUploadResult)
	rsp := sres.Rsps[len(sres.Rsps)-1]
	if rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", rsp.Rc)
		return
	}

	fmt.Printf("Done\n")
}

func fsCmd() *cobra.Command {
	fsCmd := &cobra.Command{
		Use:   "fs",
		Short: "Access files on a device",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	uploadEx := "  " + nmutil.ToolInfo.ExeName +
		" -c olimex fs upload sample.lua /sample.lua\n"

	uploadCmd := &cobra.Command{
		Use:     "upload <src-filename> <dst-filename> -c <conn_profile>",
		Short:   "Upload file to a device",
		Example: uploadEx,
		Run:     fsUploadRunCmd,
	}
	fsCmd.AddCommand(uploadCmd)

	downloadEx := "  " + nmutil.ToolInfo.ExeName +
		" -c olimex image download /cfg/mfg mfg.txt\n"

	downloadCmd := &cobra.Command{
		Use:     "download <src-filename> <dst-filename> -c <conn_profile>",
		Short:   "Download file from a device",
		Example: downloadEx,
		Run:     fsDownloadRunCmd,
	}
	fsCmd.AddCommand(downloadCmd)

	return fsCmd
}
