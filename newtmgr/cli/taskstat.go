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

func taskStatRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewTaskStatCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.TaskStatResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
		return
	}

	names := make([]string, 0, len(sres.Rsp.Tasks))
	for k, _ := range sres.Rsp.Tasks {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Printf("  %8s %3s %3s %8s %8s %8s %8s %8s %8s\n",
		"task", "pri", "tid", "runtime", "csw", "stksz",
		"stkuse", "last_checkin", "next_checkin")
	for _, n := range names {
		t := sres.Rsp.Tasks[n]
		fmt.Printf("  %8s %3d %3d %8d %8d %8d %8d %8d %8d\n",
			n,
			t["prio"],
			t["tid"],
			t["runtime"],
			t["cswcnt"],
			t["stksiz"],
			t["stkuse"],
			t["last_checkin"],
			t["next_checkin"])
	}
}

func taskStatCmd() *cobra.Command {
	taskStatCmd := &cobra.Command{
		Use:   "taskstat -c <conn_profile>",
		Short: "Read task statistics from a device",
		Run:   taskStatRunCmd,
	}

	return taskStatCmd
}
