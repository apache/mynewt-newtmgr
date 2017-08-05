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
	"strings"

	"github.com/runtimeco/go-coap"
	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func extractResKv(params []string) (map[string]interface{}, error) {
	m := map[string]interface{}{}

	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			return nil, util.FmtNewtError("invalid resource specifier: %s",
				param)
		}

		// XXX: For now, assume all values are strings.
		m[parts[0]] = parts[1]
	}

	return m, nil
}

func resGetRunCmd(s sesn.Sesn, resType sesn.ResourceType, uri string) {
	c := xact.NewGetResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Uri = uri
	c.Typ = resType

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

	var valstr string

	m, err := nmxutil.DecodeCborMap(sres.Value)
	if err != nil {
		valstr = hex.Dump(sres.Value)
	} else if len(m) == 0 {
		valstr = "<empty>"
	} else if len(m) == 1 {
		for k, v := range m {
			valstr = fmt.Sprintf("%s=%+v", k, v)
		}
	} else {
		for k, v := range m {
			valstr += fmt.Sprintf("\n    %s=%+v", k, v)
		}
	}

	fmt.Printf("%s: %s\n", uri, valstr)
}

func resPutRunCmd(s sesn.Sesn, resType sesn.ResourceType, uri string,
	value map[string]interface{}) {

	b, err := nmxutil.EncodeCborMap(value)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	c := xact.NewPutResCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Uri = uri
	c.Typ = resType
	c.Value = b

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.PutResResult)
	if sres.Status() != 0 {
		fmt.Printf("Error: %s (%d)\n",
			coap.COAPCode(sres.Status()), sres.Status())
	} else {
		fmt.Printf("Done\n")
	}
}

func resRunCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		nmUsage(cmd, nil)
	}

	rt, err := sesn.ParseResType(args[0])
	if err != nil {
		nmUsage(cmd, err)
	}

	uri := args[1]

	var m map[string]interface{}
	if len(args) >= 3 {
		m, err = extractResKv(args[2:])
		if err != nil {
			nmUsage(cmd, err)
		}
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	if m == nil {
		resGetRunCmd(s, rt, uri)
	} else {
		resPutRunCmd(s, rt, uri, m)
	}
}

func resCmd() *cobra.Command {
	resEx := "   newtmgr -c olimex res public mynewt.value.0\n"

	resCmd := &cobra.Command{
		Use:     "res <type> <uri> [k=v] [k=v] [...]",
		Short:   "Read or write a CoAP resource on a device",
		Example: resEx,
		Run:     resRunCmd,
	}

	return resCmd
}
