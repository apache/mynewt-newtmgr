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

package nmp

import (
	"fmt"
)

//////////////////////////////////////////////////////////////////////////////
// $defs                                                                    //
//////////////////////////////////////////////////////////////////////////////

const (
	LEVEL_DEBUG    int = 0
	LEVEL_INFO         = 1
	LEVEL_WARN         = 2
	LEVEL_ERROR        = 3
	LEVEL_CRITICAL     = 4
	/* Upto 7 custom loglevels */
	LEVEL_MAX = 255
)

const (
	STREAM_LOG  int = 0
	MEMORY_LOG      = 1
	STORAGE_LOG     = 2
)

const (
	MODULE_DEFAULT     int = 0
	MODULE_OS              = 1
	MODULE_NEWTMGR         = 2
	MODULE_NIMBLE_CTLR     = 3
	MODULE_NIMBLE_HOST     = 4
	MODULE_NFFS            = 5
	MODULE_REBOOT          = 6
	MODULE_TEST            = 8
	MODULE_MAX             = 255
)

var LogModuleNameMap = map[int]string{
	MODULE_DEFAULT:     "DEFAULT",
	MODULE_OS:          "OS",
	MODULE_NEWTMGR:     "NEWTMGR",
	MODULE_NIMBLE_CTLR: "NIMBLE_CTLR",
	MODULE_NIMBLE_HOST: "NIMBLE_HOST",
	MODULE_NFFS:        "NFFS",
	MODULE_REBOOT:      "REBOOT",
	MODULE_TEST:        "TEST",
}

var LogLevelNameMap = map[int]string{
	LEVEL_DEBUG:    "DEBUG",
	LEVEL_INFO:     "INFO",
	LEVEL_WARN:     "WARN",
	LEVEL_ERROR:    "ERROR",
	LEVEL_CRITICAL: "CRITICAL",
}

var LogTypeNameMap = map[int]string{
	STREAM_LOG:  "STREAM",
	MEMORY_LOG:  "MEMORY",
	STORAGE_LOG: "STORAGE",
}

func LogModuleToString(lm int) string {
	name := LogModuleNameMap[lm]
	if name == "" {
		name = "CUSTOM"
	}
	return name
}

func LogLevelToString(lm int) string {
	name := LogLevelNameMap[lm]
	if name == "" {
		name = "CUSTOM"
	}
	return name
}

func LogTypeToString(lm int) string {
	name := LogTypeNameMap[lm]
	if name == "" {
		name = "UNDEFINED"
	}
	return name
}

//////////////////////////////////////////////////////////////////////////////
// $show                                                                    //
//////////////////////////////////////////////////////////////////////////////

type LogEntryType int

const (
	LOG_ENTRY_TYPE_STRING LogEntryType = 0
	LOG_ENTRY_TYPE_CBOR                = 1
	LOG_ENTRY_TYPE_BINARY              = 2
)

var LogEntryTypeStringMap = map[LogEntryType]string{
	LOG_ENTRY_TYPE_STRING: "str",
	LOG_ENTRY_TYPE_CBOR:   "cbor",
	LOG_ENTRY_TYPE_BINARY: "bin",
}

type LogShowReq struct {
	NmpBase   `codec:"-"`
	Name      string `codec:"log_name"`
	Timestamp int64  `codec:"ts"`
	Index     uint32 `codec:"index"`
}

type LogEntry struct {
	Index     uint32       `codec:"index"`
	Timestamp int64        `codec:"ts"`
	Module    uint8        `codec:"module"`
	Level     uint8        `codec:"level"`
	Type      LogEntryType `codec:"type"`
	Msg       []byte       `codec:"msg"`
}

type LogShowLog struct {
	Name    string     `codec:"name"`
	Type    int        `codec:"type"`
	Entries []LogEntry `codec:"entries"`
}

type LogShowRsp struct {
	NmpBase
	Rc        int          `codec:"rc" codec:",omitempty"`
	NextIndex uint32       `codec:"next_index"`
	Logs      []LogShowLog `codec:"logs"`
}

func NewLogShowReq() *LogShowReq {
	r := &LogShowReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_SHOW)
	return r
}

func (r *LogShowReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewLogShowRsp() *LogShowRsp {
	return &LogShowRsp{}
}

func (r *LogShowRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $list                                                                    //
//////////////////////////////////////////////////////////////////////////////

type LogListReq struct {
	NmpBase `codec:"-"`
}

type LogListRsp struct {
	NmpBase
	Rc   int      `codec:"rc"`
	List []string `codec:"log_list"`
}

func NewLogListReq() *LogListReq {
	r := &LogListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_LIST)
	return r
}

func (r *LogListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewLogListRsp() *LogListRsp {
	return &LogListRsp{}
}

func (r *LogListRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $module list                                                             //
//////////////////////////////////////////////////////////////////////////////

type LogModuleListReq struct {
	NmpBase `codec:"-"`
}

type LogModuleListRsp struct {
	NmpBase
	Rc  int            `codec:"rc"`
	Map map[string]int `codec:"module_map"`
}

func NewLogModuleListReq() *LogModuleListReq {
	r := &LogModuleListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_MODULE_LIST)
	return r
}

func (r *LogModuleListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewLogModuleListRsp() *LogModuleListRsp {
	return &LogModuleListRsp{}
}

func (r *LogModuleListRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $level list                                                              //
//////////////////////////////////////////////////////////////////////////////

type LogLevelListReq struct {
	NmpBase `codec:"-"`
}

type LogLevelListRsp struct {
	NmpBase
	Rc  int            `codec:"rc"`
	Map map[string]int `codec:"level_map"`
}

func NewLogLevelListReq() *LogLevelListReq {
	r := &LogLevelListReq{}
	fillNmpReq(r, NMP_OP_READ, NMP_GROUP_LOG, NMP_ID_LOG_LEVEL_LIST)
	return r
}

func (r *LogLevelListReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewLogLevelListRsp() *LogLevelListRsp {
	return &LogLevelListRsp{}
}

func (r *LogLevelListRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $clear                                                                   //
//////////////////////////////////////////////////////////////////////////////

type LogClearReq struct {
	NmpBase `codec:"-"`
}

type LogClearRsp struct {
	NmpBase
	Rc int `codec:"rc"`
}

func NewLogClearReq() *LogClearReq {
	r := &LogClearReq{}
	fillNmpReq(r, NMP_OP_WRITE, NMP_GROUP_LOG, NMP_ID_LOG_CLEAR)
	return r
}

func (r *LogClearReq) Msg() *NmpMsg { return MsgFromReq(r) }

func NewLogClearRsp() *LogClearRsp {
	return &LogClearRsp{}
}

func (r *LogClearRsp) Msg() *NmpMsg { return MsgFromReq(r) }

//////////////////////////////////////////////////////////////////////////////
// $LogType Marshal/Unmarshal                                               //
//////////////////////////////////////////////////////////////////////////////

func LogEntryTypeToString(LogType LogEntryType) string {
	s := LogEntryTypeStringMap[LogType]
	if s == "" {
		return "???"
	}

	return s
}

func LogEntryTypeFromString(s string) (LogEntryType, error) {
	for LogType, name := range LogEntryTypeStringMap {
		if s == name {
			return LogType, nil
		}
	}

	return LogEntryType(0), fmt.Errorf("Invalid LogEntryType string: %s", s)
}

func (l LogEntryType) String() string {
	return LogEntryTypeToString(l)
}

func (l LogEntryType) MarshalBinary() ([]byte, error) {
	return []byte(l.String()), nil
}

func (l *LogEntryType) UnmarshalBinary(data []byte) error {
	var err error

	*l, err = LogEntryTypeFromString(string(data))
	if err != nil {
		return err
	}

	return nil
}
