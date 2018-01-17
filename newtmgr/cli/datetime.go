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

func dateTimeRead(s sesn.Sesn) error {
	c := xact.NewDateTimeReadCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		return util.ChildNewtError(err)
	}

	sres := res.(*xact.DateTimeReadResult)
	fmt.Println("Datetime(RFC 3339 format):", sres.Rsp.DateTime)

	return nil
}

func dateTimeWrite(s sesn.Sesn, args []string) error {
	c := xact.NewDateTimeWriteCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.DateTime = args[0]

	res, err := c.Run(s)
	if err != nil {
		return util.ChildNewtError(err)
	}

	sres := res.(*xact.DateTimeWriteResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("Error: %d\n", sres.Rsp.Rc)
	} else {
		fmt.Printf("Done\n")
	}

	return nil
}

func dateTimeRunCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	if len(args) == 0 {
		if err := dateTimeRead(s); err != nil {
			nmUsage(nil, err)
		}
	} else {
		if err := dateTimeWrite(s, args); err != nil {
			nmUsage(nil, err)
		}
	}
}

func dateTimeCmd() *cobra.Command {
	dateTimeHelpText := "Display or set datetime on a device. "
	dateTimeHelpText += "Specify a datetime-value\n"
	dateTimeHelpText += "to set the datetime on the device.\n\n"
	dateTimeHelpText += "Must specify datetime-value in RFC 3339 format.\n"

	dateTimeEx := nmutil.ToolInfo.ExeName + " datetime -c myserial\n"
	dateTimeEx += nmutil.ToolInfo.ExeName +
		" datetime 2016-03-02T22:44:00 -c myserial" +
		"             (implicit UTC) \n"
	dateTimeEx += nmutil.ToolInfo.ExeName +
		" datetime 2016-03-02T22:44:00Z -c myserial" +
		"            (explicit UTC)\n"
	dateTimeEx += nmutil.ToolInfo.ExeName +
		" datetime 2016-03-02T22:44:00-08:00 -c myserial" +
		"       (PST)\n"
	dateTimeEx += nmutil.ToolInfo.ExeName +
		" datetime 2016-03-02T22:44:00.1 -c myserial" +
		"           (fractional secs)\n"
	dateTimeEx += nmutil.ToolInfo.ExeName +
		" datetime 2016-03-02T22:44:00.101+05:30 -c myserial" +
		"   (fractional secs + timezone)\n"

	dateTimeCmd := &cobra.Command{
		Use:     "datetime [rfc-3339-date-string] -c <conn_profile>",
		Short:   "Manage datetime on a device",
		Long:    dateTimeHelpText,
		Example: dateTimeEx,
		Run:     dateTimeRunCmd,
	}

	return dateTimeCmd
}
