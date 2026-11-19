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
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

var optLogShowFull bool

// Converts the provided CBOR map to a JSON string.
func logCborMsgText(cborMap []byte) (string, error) {
	cm, err := nmxutil.DecodeCborMap(cborMap)
	if err != nil {
		return "", err
	}

	msg, err := json.Marshal(cm)
	if err != nil {
		return "", util.ChildNewtError(err)
	}

	return string(msg), nil
}

type logShowCfg struct {
	Name      string
	Last      bool
	Index     uint32
	Timestamp int64
}

type logNumEntriesCfg struct {
	Name      string
	Index     uint32
}

func logNumEntriesParseArgs(args []string) (*logNumEntriesCfg, error) {
	cfg := &logNumEntriesCfg{}

	if len(args) < 1 {
		return cfg, nil
	}
	cfg.Name = args[0]

	if len(args) < 2 {
		cfg.Index = 0
  } else {
		u64, err := strconv.ParseUint(args[1], 0, 64)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}
		if u64 > 0xffffffff {
			return nil, util.NewNewtError("index out of range")
		}
		cfg.Index = uint32(u64)
	}

	return cfg, nil
}

func logShowParseArgs(args []string) (*logShowCfg, error) {
	cfg := &logShowCfg{}

	if len(args) < 1 {
		return cfg, nil
	}
	cfg.Name = args[0]

	if len(args) < 2 {
		return cfg, nil
	}

	if args[1] == "last" {
		cfg.Index = 0
		cfg.Timestamp = -1
	} else {
		u64, err := strconv.ParseUint(args[1], 0, 64)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}
		if u64 > 0xffffffff {
			return nil, util.NewNewtError("index out of range")
		}
		cfg.Index = uint32(u64)
	}

	if len(args) < 3 || args[1] == "last" {
		return cfg, nil
	}

	ts, err := strconv.ParseInt(args[2], 0, 64)
	if err != nil {
		return nil, util.ChildNewtError(err)
	}
	cfg.Timestamp = ts

	return cfg, nil
}

func printLogShowRsp(rsp *nmp.LogShowRsp, printHdr bool) {
	if len(rsp.Logs) == 0 {
		fmt.Printf("(no logs retrieved)\n")
		return
	}

	for _, log := range rsp.Logs {
		if printHdr {
			fmt.Printf("Name: %s\n", log.Name)
			fmt.Printf("Type: %s\n", nmp.LogTypeToString(log.Type))

			fmt.Printf("%16s %10s %22s | %16s %16s %6s %8s %s\n",
				"[num_entries]", "[index]", "[timestamp]", "[module]", "[level]", "[type]",
				"[img]", "[message]")
		}

		for _, entry := range log.Entries {
			modText := fmt.Sprintf("%s (%d)",
				nmp.LogModuleToString(int(entry.Module)), entry.Module)
			levText := fmt.Sprintf("%s (%d)",
				nmp.LogLevelToString(int(entry.Level)), entry.Level)

			var err error
			msgText := ""
			switch entry.Type {
			case nmp.LOG_ENTRY_TYPE_STRING:
				msgText = string(entry.Msg)
			case nmp.LOG_ENTRY_TYPE_CBOR:
				msgText, err = logCborMsgText(entry.Msg)
				if err != nil {
					fmt.Printf("Error decoding CBOR entry: %s; "+
						"idx=%d",
						err.Error(), entry.Index)
					msgText = hex.EncodeToString(entry.Msg)
				}

			case nmp.LOG_ENTRY_TYPE_BINARY:
				msgText = hex.EncodeToString(entry.Msg)

			default:
				fmt.Printf(
					"Error decoding entry: unknown entry type (%d); idx=%d",
					int(entry.Type), entry.Index)
				msgText = hex.EncodeToString(entry.Msg)
			}

			fmt.Printf("%16d %10d %20dus | %16s %16s %6s %8s %s\n",
				entry.NumEntries,
				entry.Index,
				entry.Timestamp,
				modText,
				levText,
				entry.Type,
				hex.EncodeToString(entry.ImgHash),
				msgText)
		}
	}
}

func logShowFullCmd(s sesn.Sesn, cfg *logShowCfg) error {
	if cfg.Name == "" {
		return util.FmtNewtError("must specify a single log to read when `-a` is used")
	}

	c := xact.NewLogShowFullCmd()
	c.SetTxOptions(nmutil.TxOptions())

	c.Name = cfg.Name
	c.Index = cfg.Index

	first := true
	c.ProgressCb = func(_ *xact.LogShowFullCmd, rsp *nmp.LogShowRsp) {
		printLogShowRsp(rsp, first)
		first = false
	}

	_, err := c.Run(s)
	if err != nil {
		return err
	}

	return nil
}

func logShowPartialCmd(s sesn.Sesn, cfg *logShowCfg) error {
	c := xact.NewLogShowCmd()
	c.SetTxOptions(nmutil.TxOptions())

	c.Name = cfg.Name
	c.Index = cfg.Index
	c.Timestamp = cfg.Timestamp

	res, err := c.Run(s)
	if err != nil {
		return err
	}

	sres := res.(*xact.LogShowResult)
	fmt.Printf("Status: %d\n", sres.Status())
	fmt.Printf("Next index: %d\n", sres.Rsp.NextIndex)
	if len(sres.Rsp.Logs) == 0 {
		fmt.Printf("(no logs retrieved)\n")
	} else {
		printLogShowRsp(sres.Rsp, true)
	}

	return nil
}

func logNumEntriesCmd(cmd *cobra.Command, args []string) {
	cfg, err := logNumEntriesParseArgs(args)
	if err != nil {
		nmUsage(cmd, err)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

  logNumEntriesProcCmd(s, cfg)
}

func logShowCmd(cmd *cobra.Command, args []string) {
	cfg, err := logShowParseArgs(args)
	if err != nil {
		nmUsage(cmd, err)
	}

	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	if optLogShowFull {
		err = logShowFullCmd(s, cfg)
	} else {
		err = logShowPartialCmd(s, cfg)
	}
	if err != nil {
		nmUsage(nil, err)
	}
}

func logListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	sort.Strings(sres.Rsp.List)

	fmt.Printf("available logs:\n")
	for _, log := range sres.Rsp.List {
		fmt.Printf("    %s\n", log)
	}
}

func logModuleListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogModuleListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogModuleListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	names := make([]string, 0, len(sres.Rsp.Map))
	for k, _ := range sres.Rsp.Map {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Printf("available modules:\n")
	for _, name := range names {
		fmt.Printf("    %s (%d)\n", name, sres.Rsp.Map[name])
	}
}

func logNumEntriesProcCmd(s sesn.Sesn, cfg *logNumEntriesCfg) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogNumEntriesCmd()
	c.SetTxOptions(nmutil.TxOptions())
	c.Name = cfg.Name
	c.Index = cfg.Index

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
  }

	sres := res.(*xact.LogNumEntriesResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
	  return;
  }

	fmt.Printf("Number of entries: %d\n", sres.Rsp.NumEntries)

  return
}

func logLevelListCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogLevelListCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogLevelListResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	vals := make([]int, 0, len(sres.Rsp.Map))
	revmap := make(map[int]string, len(sres.Rsp.Map))
	for name, val := range sres.Rsp.Map {
		vals = append(vals, val)
		revmap[val] = name
	}
	sort.Ints(vals)

	fmt.Printf("available levels:\n")
	for _, val := range vals {
		fmt.Printf("    %d: %s\n", val, revmap[val])
	}
}

func logClearCmd(cmd *cobra.Command, args []string) {
	s, err := GetSesn()
	if err != nil {
		nmUsage(nil, err)
	}

	c := xact.NewLogClearCmd()
	c.SetTxOptions(nmutil.TxOptions())

	res, err := c.Run(s)
	if err != nil {
		nmUsage(nil, util.ChildNewtError(err))
	}

	sres := res.(*xact.LogClearResult)
	if sres.Rsp.Rc != 0 {
		fmt.Printf("error: %d\n", sres.Rsp.Rc)
		return
	}

	fmt.Printf("done\n")
}

func logCmd() *cobra.Command {
	logCmd := &cobra.Command{
		Use:   "log",
		Short: "Manage logs on a device",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
	}

	logShowHelpText := "Show logs on a device.  Optional log-name, min-index, and min-timestamp\nparameters can be specified to filter the logs to display.\n\n"
	logShowHelpText += "- log-name specifies the log to display.  If log-name is not specified, all\nlogs are displayed.\n\n"
	logShowHelpText += "- min-index specifies to only display the log entries with an index value equal to or higher than min-index.  "
	logShowHelpText += "If \"last\"  is specified for min-index, the last\nlog entry is displayed.\n\n"
	logShowHelpText += "- min-timestamp specifies to only display the log entries with a timestamp\nequal to or later than min-timestamp. Log entries with a timestamp equal to\nmin-timestamp are only displayed if the entry index is equal to or higher than min-index.\n"

	logShowEx := nmutil.ToolInfo.ExeName + " log show -c myserial\n"
	logShowEx += nmutil.ToolInfo.ExeName + " log show reboot_log -c myserial\n"
	logShowEx += nmutil.ToolInfo.ExeName + " log show reboot_log last -c myserial\n"
	logShowEx += nmutil.ToolInfo.ExeName + " log show reboot_log 5 -c myserial\n"
	logShowEx += nmutil.ToolInfo.ExeName + " log show reboot_log 3 1122222 -c myserial\n"

	showCmd := &cobra.Command{
		Use:     "show [log-name [min-index [min-timestamp]]] -c <conn_profile>",
		Example: logShowEx,
		Long:    logShowHelpText,
		Short:   "Show the logs on a device",
		Run:     logShowCmd,
	}
	showCmd.PersistentFlags().BoolVarP(&optLogShowFull, "all", "a", false, "read until end of log")
	logCmd.AddCommand(showCmd)

	clearCmd := &cobra.Command{
		Use:     "clear -c <conn_profile>",
		Short:   "Clear the logs on a device",
		Example: logShowEx,
		Run:     logClearCmd,
	}
	logCmd.AddCommand(clearCmd)

	moduleListCmd := &cobra.Command{
		Use:   "module_list -c <conn_profile>",
		Short: "Show the log module names",
		Run:   logModuleListCmd,
	}
	logCmd.AddCommand(moduleListCmd)

	levelListCmd := &cobra.Command{
		Use:   "level_list -c <conn_profile>",
		Short: "Show the log levels",
		Run:   logLevelListCmd,
	}

	logCmd.AddCommand(levelListCmd)

	ListCmd := &cobra.Command{
		Use:   "list -c <conn_profile>",
		Short: "Show the log names",
		Run:   logListCmd,
	}

	logCmd.AddCommand(ListCmd)

	NumEntriesCmd := &cobra.Command{
		Use:   "num_entries [log-name [min-index]] -c <conn_profile>",
		Short: "Show the number of entries from index for a particular log",
		Run:   logNumEntriesCmd,
	}

	logCmd.AddCommand(NumEntriesCmd)

	return logCmd
}
