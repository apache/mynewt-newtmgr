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
	"strconv"
	"strings"

	"github.com/runtimeco/go-coap"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func indent(s string, numSpaces int) string {
	b := make([]byte, numSpaces)
	for i, _ := range b {
		b[i] = ' '
	}
	tab := string(b)

	nltab := "\n" + tab
	return tab + strings.Replace(s, "\n", nltab, -1)
}

func cborValStr(itf interface{}) string {
	switch v := itf.(type) {
	case string:
		return v

	case []byte:
		return strings.TrimSuffix(hex.Dump(v), "\n")

	default:
		return fmt.Sprintf("%#v", v)
	}
}

func extractResKv(params []string) (map[string]interface{}, error) {
	m := map[string]interface{}{}

	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, util.FmtNewtError("invalid resource specifier: %s",
				param)
		}

		var val interface{}

		// If value is quoted, parse it as a string.
		if strings.HasPrefix(parts[1], "\"") &&
			strings.HasSuffix(parts[1], "\"") {

			val = parts[1][1 : len(parts[1])-1]
		} else {
			// Try to parse value as an integer.
			num, err := strconv.Atoi(parts[1])
			if err == nil {
				val = num
			} else {
				val = parts[1]
			}
		}

		m[parts[0]] = val
	}

	return m, nil
}

func resResponseStr(path string, cbor []byte) string {
	s := path

	if len(cbor) > 0 {
		m, err := nmxutil.DecodeCbor(cbor)
		if err != nil {
			s += fmt.Sprintf("\n    invalid incoming cbor:%v\n%s",
				err, hex.Dump(cbor))
		}
		s += fmt.Sprintf("\n%v", m)
	} else {
		s += "\n    <empty>"
	}
	return s
}

func resGetCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	path := args[1]

	c := xact.NewGetResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Path = path
	c.Typ = rt

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.GetResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(c.Path, sres.Value))
	}
}

func resPutCmd(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	path := args[1]

	var m map[string]interface{}
	m, err = extractResKv(args[2:])
	if err != nil {
		nmUsage(cmd, err)
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	c := xact.NewPutResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Path = path
	c.Typ = rt
	c.Value = b

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.PutResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(c.Path, sres.Value))
	}
}

func resPostCmd(cmd *cobra.Command, args []string) {
	if len(args) < 3 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	path := args[1]

	var m map[string]interface{}
	m, err = extractResKv(args[2:])
	if err != nil {
		nmUsage(cmd, err)
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	c := xact.NewPostResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Path = path
	c.Typ = rt
	c.Value = b

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.PostResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(c.Path, sres.Value))
	}
}

func resDeleteCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	path := args[1]

	var m map[string]interface{}
	m, err = extractResKv(args[2:])
	if err != nil {
		nmUsage(cmd, err)
	}

	b, err := nmxutil.EncodeCborMap(m)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	c := xact.NewDeleteResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Path = path
	c.Typ = rt
	c.Value = b

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.DeleteResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
		return
	}

	if sres.Value != nil {
		fmt.Printf("%s\n", resResponseStr(c.Path, sres.Value))
	}
}

func resCmd() *cobra.Command {
	resCmd := &cobra.Command{
		Use:   "res",
		Short: "Access a CoAP resource on a device",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	resCmd.AddCommand(&cobra.Command{
		Use:   "get <type> <path>",
		Short: "Send a CoAP GET request",
		Run:   resGetCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "put <type> <path> <k=v> [k=v] [k=v]",
		Short: "Send a CoAP PUT request",
		Run:   resPutCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "post <type> <path> <k=v> [k=v] [k=v]",
		Short: "Send a CoAP POST request",
		Run:   resPostCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "delete <type> <path>",
		Short: "Send a CoAP DELETE request",
		Run:   resDeleteCmd,
	})

	return resCmd
}
