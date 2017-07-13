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

func mempoolStatRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewMempoolStatCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.MempoolStatResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
		return
	}

	names := make([]string, 0, len(sres.Rsp.Mpools))
	for k, _ := range sres.Rsp.Mpools {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Printf("%32s %5s %4s %4s %4s\n", "name", "blksz", "cnt", "free", "min")
	for _, n := range names {
		mp := sres.Rsp.Mpools[n]
		fmt.Printf("%32s %5d %4d %4d %4d\n",
			n,
			mp["blksiz"],
			mp["nblks"],
			mp["nfree"],
			mp["min"])
	}
}

func mempoolStatCmd() *cobra.Command {
	mempoolStatCmd := &cobra.Command{
		Use:   "mpstat -c <conn_profile>",
		Short: "Read mempool statistics from a device",
		Run:   mempoolStatRunCmd,
	}

	return mempoolStatCmd
}
