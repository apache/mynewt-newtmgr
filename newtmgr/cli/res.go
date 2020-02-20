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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/runtimeco/go-coap"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

var details bool
var resJson bool
var resInt bool
var resJsonFilename string
var resRawFilename string
var resBinFilename string

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
	class := (code & 0xE0) >> 5
	d1 := (code & 0x18) >> 3
	d2 := code & 0x07
	s += fmt.Sprintf("CoAP Response Code: %d.%d%d %s\n", class, d1, d2, code)
	return s
}

func printDetails(msg coap.Message) string {
	var s string
	s += printCode(msg.Code())
	if msg.Token() != nil {
		s += fmt.Sprintf(
			"CoAP Response Token: %v\n", hex.EncodeToString(msg.Token()))
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

func parsePayloadMap(args []string) (map[string]interface{}, error) {
	if len(args) == 0 {
		return nil, nil
	}

	m, err := extractResKv(args)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// removeFloats processes an unmarshalled JSON value, converting float64 values
// to int64 values where possible.  For arrays, it is recursively called on
// each element.  For objects (maps), it is recursively called on each value.
// A float is converted to an int if conversion does not result in a change in
// value.  For example, `2.0` is converted, but `1.5` is not.
func removeFloats(itf interface{}) interface{} {
	tryToInt64 := func(f float64) interface{} {
		i := int64(f)
		if float64(i) == f {
			return i
		} else {
			return f
		}
	}

	switch actual := itf.(type) {
	case float32:
		return tryToInt64(float64(actual))

	case float64:
		return tryToInt64(actual)

	case []interface{}:
		for i, val := range actual {
			actual[i] = removeFloats(val)
		}
		return actual

	case map[string]interface{}:
		for k, v := range actual {
			actual[k] = removeFloats(v)
		}

		return actual

	default:
		return actual
	}
}

func parsePayloadJson(arg string) (interface{}, error) {
	var val interface{}

	if err := json.Unmarshal([]byte(arg), &val); err != nil {
		return nil, util.ChildNewtError(err)
	}

	// If the user specified `--int`, convert floats to ints where it does not
	// result in truncation.
	if resInt {
		val = removeFloats(val)
	}

	return val, nil
}

func parsePayload(args []string) ([]byte, error) {
	var val interface{}
	var err error

	if len(args) == 0 {
		return nil, nil
	}

	if resJson {
		val, err = parsePayloadJson(args[0])
		if err != nil {
			return nil, err
		}
		// Check for zero payload.  Need to return nil explicitly; don't wrap
		// in interface{}.
		if val == nil {
			return nil, nil
		}
	} else {
		val, err = parsePayloadMap(args)
		if err != nil {
			return nil, err
		}
		// Check for zero payload.  Need to return nil explicitly; don't wrap
		// in interface{}.
		if val == nil {
			return nil, nil
		}
	}

	b, err := nmxutil.EncodeCbor(val)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func calcCborPayload(args []string) ([]byte, error) {
	if resRawFilename != "" {
		c, err := ioutil.ReadFile(resRawFilename)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}

		return c, nil
	}

	if resJsonFilename != "" {
		j, err := ioutil.ReadFile(resJsonFilename)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}

		val, err := parsePayloadJson(string(j))
		if err != nil {
			return nil, err
		}

		c, err := nmxutil.EncodeCbor(val)
		if err != nil {
			return nil, err
		}

		return c, nil
	}

	if resBinFilename != "" {
		b, err := ioutil.ReadFile(resBinFilename)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}

		c, err := nmxutil.EncodeCbor(b)
		if err != nil {
			return nil, err
		}

		return c, nil
	}

	c, err := parsePayload(args)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func runResCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	numFileArgs := 0
	if resRawFilename != "" {
		numFileArgs++
	}
	if resJsonFilename != "" {
		numFileArgs++
	}
	if resBinFilename != "" {
		numFileArgs++
	}
	if numFileArgs > 1 {
		nmUsage(cmd, util.FmtNewtError(
			"too many payload files specified: have=%d want<=1", numFileArgs))
	}

	op, err := nmcoap.ParseOp(args[0])
	if err != nil {
		nmUsage(nil, err)
	}

	path := args[1]

	b, err := calcCborPayload(args[2:])
	if err != nil {
		nmUsage(nil, err)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.MsgParams = nmcoap.MsgParams{
		Code:    op,
		Uri:     path,
		Payload: b,
	}

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.ResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n", sres.Rsp.Code(), sres.Rsp.Code())
		return
	}

	if sres.Rsp.Payload() != nil {
		fmt.Printf("%s\n", resResponseStr(path, sres.Rsp.Payload()))
	}

	if details {
		fmt.Printf(printDetails(sres.Rsp))
	}
}

func resCmd() *cobra.Command {
	resCmd := &cobra.Command{
		Use:   "res <op> <path> <k=v> [k=v] [k=v]",
		Short: "Access a CoAP resource on a device",
		Run:   runResCmd,
	}

	resCmd.PersistentFlags().BoolVarP(&details, "details", "d", false,
		"Show more details about the CoAP response")
	resCmd.PersistentFlags().BoolVarP(&resJson, "json", "j", false,
		"Accept a JSON string for the CoAP message body (not `k=v` pairs)")
	resCmd.PersistentFlags().StringVarP(&resJsonFilename, "jsonfile", "J", "",
		"Name of file containing JSON for the CoAP message body")
	resCmd.PersistentFlags().StringVarP(&resRawFilename, "rawfile", "R", "",
		"Name of file containing the raw CoAP message body")
	resCmd.PersistentFlags().StringVarP(&resBinFilename, "binfile", "B", "",
		"Name of file containing bytes to encode as a byte string for the "+
			"CoAP message body")
	resCmd.PersistentFlags().BoolVarP(&resInt, "int", "", false,
		"Parse all numbers as integer values where possible "+
			"(only applicable when combined with -j or -J)")

	return resCmd
}
