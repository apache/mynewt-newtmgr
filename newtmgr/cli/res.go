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
	"encoding/json"

	"github.com/runtimeco/go-coap"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

var details bool

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
		} else if strings.ToLower(parts[1]) == "false" ||
			strings.ToLower(parts[1]) == "true" {
			// parse value as boolean
			b, err := strconv.ParseBool(strings.ToLower(parts[1]))
			if err == nil {
				val = b
			} else {
				val = parts[1]
			}
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

func printCode(code coap.COAPCode) string {
	var s string
	class := (code & 0xE0)>>5
	d1 := (code & 0x18)>>3
	d2 := code & 0x07
	s += fmt.Sprintf("CoAP Response Code: %d.%d%d %s\n", class, d1, d2, code)
	return s
}

func printDetails(sres interface{}) string {
	var s string
	switch sres := sres.(type) {
	case *xact.GetResResult:
		s += printCode(sres.Code)
		if sres.Token != nil {
			s += fmt.Sprintf("CoAP Response Token: %v\n", hex.EncodeToString(sres.Token))
		}
	case *xact.PutResResult:
		s += printCode(sres.Code)
	case *xact.PostResResult:
		s += printCode(sres.Code)
	case *xact.DeleteResResult:
		s += printCode(sres.Code)
	}
	return s
}

/* Helper functions to convert JSON object into pretty format
   Adapted from elastic/beats/libbeat/common/mapstr.go
*/

func cleanUpInterfaceArray(in []interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, v := range in {
		result[i] = cleanUpMapValue(v)
	}
	return result
}

func cleanUpInterfaceMap(in map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range in {
		result[fmt.Sprintf("%v", k)] = cleanUpMapValue(v)
	}
	return result
}

func cleanUpMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanUpInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanUpInterfaceMap(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func resResponseStr(path string, cbor []byte) string {
	s := path

	if len(cbor) > 0 {
		m, err := nmxutil.DecodeCbor(cbor)
		if err != nil {
			s += fmt.Sprintf("\n    invalid incoming cbor:%v\n%s",
				err, hex.Dump(cbor))
		}
		j, err := json.MarshalIndent(cleanUpMapValue(m), "", "    ")
		if err != nil {
			s += fmt.Sprintf("\nerror: ", err)
		}
		s += fmt.Sprintf("\n%v", string(j))
	} else {
		s += "\n    <empty>"
	}
	return s
}

func resGetCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	path := args[0]

	c := xact.NewGetResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Path = path

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

	if details {
		fmt.Printf(printDetails(sres))
	}
}

func resPutCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	path := args[0]

	var m map[string]interface{}
	m, err = extractResKv(args[1:])
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

	if details {
		fmt.Printf(printDetails(sres))
	}
}

func resPostCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	path := args[0]

	var m map[string]interface{}
	m, err = extractResKv(args[1:])
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

	if details {
		fmt.Printf(printDetails(sres))
	}
}

func resDeleteCmd(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		nmUsage(cmd, nil)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	path := args[0]

	var m map[string]interface{}
	m, err = extractResKv(args[1:])
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

	if details {
		fmt.Printf(printDetails(sres))
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
		Use:   "get <path>",
		Short: "Send a CoAP GET request",
		Run:   resGetCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "put <path> <k=v> [k=v] [k=v]",
		Short: "Send a CoAP PUT request",
		Run:   resPutCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "post <path> <k=v> [k=v] [k=v]",
		Short: "Send a CoAP POST request",
		Run:   resPostCmd,
	})

	resCmd.AddCommand(&cobra.Command{
		Use:   "delete <path>",
		Short: "Send a CoAP DELETE request",
		Run:   resDeleteCmd,
	})

	resCmd.PersistentFlags().BoolVarP(&details, "details", "d", false, "Show more details about the CoAP response")

	return resCmd
}
