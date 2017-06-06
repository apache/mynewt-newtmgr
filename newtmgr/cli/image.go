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
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/core"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

var (
	coreElfify   bool
	coreOffset   uint32
	coreNumBytes uint32
)

func imageFlagsStr(image nmp.ImageStateEntry) string {
	strs := []string{}

	if image.Active {
		strs = append(strs, "active")
	}
	if image.Confirmed {
		strs = append(strs, "confirmed")
	}
	if image.Pending {
		strs = append(strs, "pending")
	}
	if image.Permanent {
		strs = append(strs, "permanent")
	}

	return strings.Join(strs, " ")
}

func imageStatePrintRsp(rsp *nmp.ImageStateRsp) error {
	if rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", rsp.Rc)
		return nil
	}
	fmt.Println("Images:")
	for _, img := range rsp.Images {
		fmt.Printf(" slot=%d\n", img.Slot)
		fmt.Printf("    version: %s\n", img.Version)
		fmt.Printf("    bootable: %v\n", img.Bootable)
		fmt.Printf("    flags: %s\n", imageFlagsStr(img))
		if len(img.Hash) == 0 {
			fmt.Printf("    hash: Unavailable\n")
		} else {
			fmt.Printf("    hash: %x\n", img.Hash)
		}
	}

	fmt.Printf("Split status: %s (%d)\n", rsp.SplitStatus.String(),
		rsp.SplitStatus)
	return nil
}

func imageStateListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewImageStateReadCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.ImageStateReadResult)

	if err := imageStatePrintRsp(ires.Rsp); err != nil {
		nmUsage(nil, err)
	}
}

func imageStateTestCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, nil)
	}

	hexBytes, err := hex.DecodeString(args[0])
	if err != nil {
		nmUsage(cmd, util.ChildNewtError(err))
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewImageStateWriteCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Hash = hexBytes
	c.Confirm = false

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.ImageStateWriteResult)

	if err := imageStatePrintRsp(ires.Rsp); err != nil {
		nmUsage(nil, err)
	}
}

func imageStateConfirmCmd(cmd *cobra.Command, args []string) {
	var hexBytes []byte
	if len(args) >= 1 {
		var err error
		hexBytes, err = hex.DecodeString(args[0])
		if err != nil {
			nmUsage(cmd, util.ChildNewtError(err))
		}
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewImageStateWriteCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Hash = hexBytes
	c.Confirm = true

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.ImageStateWriteResult)

	if err := imageStatePrintRsp(ires.Rsp); err != nil {
		nmUsage(nil, err)
	}
}

func imageUploadCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, util.NewNewtError("Need to specify image to upload"))
	}

	imageFile, err := ioutil.ReadFile(args[0])
	if err != nil {
		nmUsage(cmd, util.NewNewtError(err.Error()))
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewImageUploadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Data = imageFile
	c.ProgressCb = func(c *xact.ImageUploadCmd, rsp *nmp.ImageUploadRsp) {
		fmt.Printf("%d\n", rsp.Off)
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.ImageUploadResult)

	if ires.Status() != 0 {
		fmt.Printf("Error: %d\n", ires.Status())
		return
	}

	fmt.Printf("Done\n")
}

func coreListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewCoreListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.CoreListResult)

	switch ires.Status() {
	case 0:
		fmt.Printf("Corefile present\n")
	case nmp.NMP_ERR_ENOENT:
		fmt.Printf("No corefiles\n")
	default:
		fmt.Printf("Error: %d\n", ires.Status())
	}
}

func coreDownloadCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, nil)
	}

	tmpName := args[0] + ".tmp"
	file, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		nmUsage(cmd, util.NewNewtError(fmt.Sprintf(
			"Cannot open file %s - %s", tmpName, err.Error())))
	}
	defer os.Remove(tmpName)
	defer file.Close()

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewCoreLoadCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.ProgressCb = func(c *xact.CoreLoadCmd, rsp *nmp.CoreLoadRsp) {
		fmt.Printf("%d\n", rsp.Off)
		if _, err := file.Write(rsp.Data); err != nil {
			nmUsage(nil, util.ChildNewtError(err))
		}
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.CoreLoadResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %d\n", sres.Status())
		return
	}

	if !coreElfify {
		os.Rename(tmpName, args[0])
		fmt.Printf("Done writing core file to %s\n", args[0])
	} else {
		coreConvert, err := core.ConvertFilenames(tmpName, args[0])
		if err != nil {
			nmUsage(nil, err)
			return
		}

		fmt.Printf("Done writing core file to %s; hash=%x\n", args[0],
			coreConvert.ImageHash)
	}
}

func coreEraseCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewCoreEraseCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.CoreEraseResult)

	if ires.Status() != 0 {
		fmt.Printf("Error: %d\n", ires.Status())
		return
	}

	fmt.Printf("Done\n")
}

func imageEraseCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewImageEraseCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}
	ires := res.(*xact.ImageEraseResult)

	if ires.Status() != 0 {
		fmt.Printf("Error: %d\n", ires.Status())
		return
	}

	fmt.Printf("Done\n")
}

func coreConvertCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
		return
	}

	coreConvert, err := core.ConvertFilenames(args[0], args[1])
	if err != nil {
		nmUsage(nil, err)
		return
	}

	fmt.Printf("Corefile created for\n   %x\n", coreConvert.ImageHash)
}

func imageCmd() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Manage images on remote instance",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show target images",
		Run:   imageStateListCmd,
	}
	imageCmd.AddCommand(listCmd)

	testCmd := &cobra.Command{
		Use:   "test <hex-image-hash>",
		Short: "Test an image on next reboot",
		Run:   imageStateTestCmd,
	}
	imageCmd.AddCommand(testCmd)

	confirmCmd := &cobra.Command{
		Use:   "confirm [hex-image-hash]",
		Short: "Permanently run image",
		Long: "If a hash is specified, permanently switch to the " +
			"corresponding image.  If no hash is specified, the current " +
			"image setup is made permanent.",
		Run: imageStateConfirmCmd,
	}
	imageCmd.AddCommand(confirmCmd)

	uploadEx := "  newtmgr -c olimex image upload <image_file>\n"
	uploadEx +=
		"  newtmgr -c olimex image upload bin/slinky_zero/apps/slinky.img\n"

	uploadCmd := &cobra.Command{
		Use:     "upload",
		Short:   "Upload image to target",
		Example: uploadEx,
		Run:     imageUploadCmd,
	}
	imageCmd.AddCommand(uploadCmd)

	coreListCmd := &cobra.Command{
		Use:     "corelist",
		Short:   "List core(s) on target",
		Example: "  newtmgr -c olimex image corelist\n",
		Run:     coreListCmd,
	}
	imageCmd.AddCommand(coreListCmd)

	coreEx := "  newtmgr -c olimex image coredownload -e <filename>\n"
	coreEx += "  newtmgr -c olimex image coredownload -e core\n"
	coreEx += "  newtmgr -c olimex image coredownload --offset 10 -n 10 core\n"

	coreDownloadCmd := &cobra.Command{
		Use:     "coredownload <dst-filename>",
		Short:   "Download core from target",
		Example: coreEx,
		Run:     coreDownloadCmd,
	}
	coreDownloadCmd.Flags().BoolVarP(&coreElfify, "elfify", "e", false,
		"Create an elf file")
	coreDownloadCmd.Flags().Uint32Var(&coreOffset, "offset", 0, "Start offset")
	coreDownloadCmd.Flags().Uint32VarP(&coreNumBytes, "bytes", "n", 0,
		"Number of bytes of the core to download")
	imageCmd.AddCommand(coreDownloadCmd)

	coreEraseEx := "  newtmgr -c olimex image coreerase\n"

	coreEraseCmd := &cobra.Command{
		Use:     "coreerase",
		Short:   "Erase core on target",
		Example: coreEraseEx,
		Run:     coreEraseCmd,
	}
	imageCmd.AddCommand(coreEraseCmd)

	imageEraseEx := "  newtmgr -c olimex image erase\n"

	imageEraseCmd := &cobra.Command{
		Use:     "erase",
		Short:   "Erase unused image on target",
		Example: imageEraseEx,
		Run:     imageEraseCmd,
	}
	imageCmd.AddCommand(imageEraseCmd)

	coreConvertCmd := &cobra.Command{
		Use:   "coreconvert <core-filename> <elf-filename>",
		Short: "Convert core to elf",
		Run:   coreConvertCmd,
	}
	imageCmd.AddCommand(coreConvertCmd)

	return imageCmd
}
