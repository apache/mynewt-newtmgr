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

package xact

import (
	"bytes"
	"fmt"
	"testing"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// rcDev answers every request with a response of the matching type
// carrying the given rc.
func rcDev(rc int) fakeHandler {
	return func(req fakeReq) (nmp.NmpRsp, error) {
		return rspFor(req, map[string]interface{}{"rc": rc})
	}
}

func TestSimpleCmds(t *testing.T) {
	hash := []byte{0xde, 0xad, 0xbe, 0xef}

	tests := []struct {
		name   string
		newCmd func() Cmd
		op     uint8
		group  uint16
		id     uint8
		fields map[string]string

		// Status() ignores the rc.
		noRc bool
	}{
		{
			name: "echo",
			newCmd: func() Cmd {
				c := NewEchoCmd()
				c.Payload = "hello"
				return c
			},
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_ECHO,
			fields: map[string]string{"d": "hello"},
		},
		{
			name:   "datetime read",
			newCmd: func() Cmd { return NewDateTimeReadCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_DATETIME_STR,
			fields: map[string]string{},
		},
		{
			name: "datetime write",
			newCmd: func() Cmd {
				c := NewDateTimeWriteCmd()
				c.DateTime = "2026-10-22T10:00:00"
				return c
			},
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_DATETIME_STR,
			fields: map[string]string{"datetime": "2026-10-22T10:00:00"},
		},
		{
			name: "config read",
			newCmd: func() Cmd {
				c := NewConfigReadCmd()
				c.Name = "id/serial"
				return c
			},
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_CONFIG,
			id:     nmp.NMP_ID_CONFIG_VAL,
			fields: map[string]string{"name": "id/serial"},
		},
		{
			name: "config write",
			newCmd: func() Cmd {
				c := NewConfigWriteCmd()
				c.Name = "a/b"
				c.Val = "42"
				c.Save = true
				return c
			},
			op:    nmp.NMP_OP_WRITE,
			group: nmp.NMP_GROUP_CONFIG,
			id:    nmp.NMP_ID_CONFIG_VAL,
			fields: map[string]string{
				"name": "a/b",
				"val":  "42",
				"save": "true",
			},
		},
		{
			name: "stat read",
			newCmd: func() Cmd {
				c := NewStatReadCmd()
				c.Name = "ble_phy"
				return c
			},
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_STAT,
			id:     nmp.NMP_ID_STAT_READ,
			fields: map[string]string{"name": "ble_phy"},
		},
		{
			name:   "stat list",
			newCmd: func() Cmd { return NewStatListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_STAT,
			id:     nmp.NMP_ID_STAT_LIST,
			fields: map[string]string{},
		},
		{
			name: "crash",
			newCmd: func() Cmd {
				c := NewCrashCmd()
				c.CrashType = CRASH_TYPE_ASSERT
				return c
			},
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_CRASH,
			id:     nmp.NMP_ID_CRASH_TRIGGER,
			fields: map[string]string{"t": "assert"},
		},
		{
			name:   "reset",
			newCmd: func() Cmd { return NewResetCmd() },
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_RESET,
			fields: map[string]string{},
			noRc:   true,
		},
		{
			name: "run test",
			newCmd: func() Cmd {
				c := NewRunTestCmd()
				c.Testname = "all"
				c.Token = "tok"
				return c
			},
			op:    nmp.NMP_OP_WRITE,
			group: nmp.NMP_GROUP_RUN,
			id:    nmp.NMP_ID_RUN_TEST,
			fields: map[string]string{
				"testname": "all",
				"token":    "tok",
			},
		},
		{
			name:   "run list",
			newCmd: func() Cmd { return NewRunListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_RUN,
			id:     nmp.NMP_ID_RUN_LIST,
			fields: map[string]string{},
		},
		{
			name: "shell exec",
			newCmd: func() Cmd {
				c := NewShellExecCmd()
				c.Argv = []string{"echo", "hi"}
				return c
			},
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_SHELL,
			id:     nmp.NMP_ID_SHELL_EXEC,
			fields: map[string]string{"argv": "[echo hi]"},
		},
		{
			name:   "mpstat",
			newCmd: func() Cmd { return NewMempoolStatCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_MPSTAT,
			fields: map[string]string{},
		},
		{
			name:   "taskstat",
			newCmd: func() Cmd { return NewTaskStatCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_TASKSTAT,
			fields: map[string]string{},
		},
		{
			name:   "mcumgr params",
			newCmd: func() Cmd { return NewMcumgrParamsCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_DEFAULT,
			id:     nmp.NMP_ID_DEF_PARAMS,
			fields: map[string]string{},
		},
		{
			name:   "image state read",
			newCmd: func() Cmd { return NewImageStateReadCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_IMAGE,
			id:     nmp.NMP_ID_IMAGE_STATE,
			fields: map[string]string{},
		},
		{
			name: "image state write",
			newCmd: func() Cmd {
				c := NewImageStateWriteCmd()
				c.Hash = hash
				c.Confirm = true
				return c
			},
			op:    nmp.NMP_OP_WRITE,
			group: nmp.NMP_GROUP_IMAGE,
			id:    nmp.NMP_ID_IMAGE_STATE,
			fields: map[string]string{
				"hash":    fmt.Sprint(hash),
				"confirm": "true",
			},
		},
		{
			name:   "image erase",
			newCmd: func() Cmd { return NewImageEraseCmd() },
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_IMAGE,
			id:     nmp.NMP_ID_IMAGE_ERASE,
			fields: map[string]string{},
		},
		{
			name:   "core list",
			newCmd: func() Cmd { return NewCoreListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_IMAGE,
			id:     nmp.NMP_ID_IMAGE_CORELIST,
			fields: map[string]string{},
		},
		{
			name:   "core erase",
			newCmd: func() Cmd { return NewCoreEraseCmd() },
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_IMAGE,
			id:     nmp.NMP_ID_IMAGE_CORELOAD,
			fields: map[string]string{},
		},
		{
			name: "log show",
			newCmd: func() Cmd {
				c := NewLogShowCmd()
				c.Name = "reboot_log"
				c.Timestamp = 123
				c.Index = 7
				return c
			},
			op:    nmp.NMP_OP_READ,
			group: nmp.NMP_GROUP_LOG,
			id:    nmp.NMP_ID_LOG_SHOW,
			fields: map[string]string{
				"log_name": "reboot_log",
				"ts":       "123",
				"index":    "7",
			},
		},
		{
			name:   "log list",
			newCmd: func() Cmd { return NewLogListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_LOG,
			id:     nmp.NMP_ID_LOG_LIST,
			fields: map[string]string{},
		},
		{
			name:   "log module list",
			newCmd: func() Cmd { return NewLogModuleListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_LOG,
			id:     nmp.NMP_ID_LOG_MODULE_LIST,
			fields: map[string]string{},
		},
		{
			name:   "log level list",
			newCmd: func() Cmd { return NewLogLevelListCmd() },
			op:     nmp.NMP_OP_READ,
			group:  nmp.NMP_GROUP_LOG,
			id:     nmp.NMP_ID_LOG_LEVEL_LIST,
			fields: map[string]string{},
		},
		{
			name:   "log clear",
			newCmd: func() Cmd { return NewLogClearCmd() },
			op:     nmp.NMP_OP_WRITE,
			group:  nmp.NMP_GROUP_LOG,
			id:     nmp.NMP_ID_LOG_CLEAR,
			fields: map[string]string{},
		},
	}

	for _, tt := range tests {
		for _, rc := range []int{0, nmp.NMP_ERR_EINVAL} {
			s := newFakeSesn(rcDev(rc))

			res, err := runCmd(t, tt.newCmd(), s)
			if err != nil {
				t.Errorf("%s rc %d: %v", tt.name, rc, err)
				continue
			}

			wantRc := rc
			if tt.noRc {
				wantRc = 0
			}
			if st := res.Status(); st != wantRc {
				t.Errorf("%s rc %d: status %d", tt.name, rc, st)
			}

			reqs := s.Requests()
			if len(reqs) != 1 {
				t.Fatalf("%s: %d requests", tt.name, len(reqs))
			}
			r := reqs[0]
			if r.Hdr.Op != tt.op || r.Hdr.Group != tt.group ||
				r.Hdr.Id != tt.id {

				t.Errorf("%s: hdr op %d group %d id %d", tt.name,
					r.Hdr.Op, r.Hdr.Group, r.Hdr.Id)
			}
			if len(r.Fields) != len(tt.fields) {
				t.Errorf("%s: fields %v, want %v", tt.name, r.Fields,
					tt.fields)
			}
			for k, v := range tt.fields {
				if got := fmt.Sprint(r.Fields[k]); got != v {
					t.Errorf("%s: field %q = %s, want %s", tt.name, k,
						got, v)
				}
			}
		}

		// Transport errors are returned to the caller.
		s := newFakeSesn(func(fakeReq) (nmp.NmpRsp, error) {
			return nil, fmt.Errorf("tx failed")
		})
		if res, err := runCmd(t, tt.newCmd(), s); err == nil ||
			res != nil {

			t.Errorf("%s: tx error: res %v err %v", tt.name, res, err)
		}
	}
}

// Requests are retried on timeout (according to TxOptions.Tries) and
// only on timeout.
func TestCmdRetries(t *testing.T) {
	tests := []struct {
		name     string
		tries    int
		failures int
		err      error
		wantReqs int
		wantErr  bool
	}{
		{"one try timeout", 1, 1, nmxutil.NewRspTimeoutError("t"), 1, true},
		{"zero tries", 0, 1, nmxutil.NewRspTimeoutError("t"), 1, true},
		{"retry ok", 3, 2, nmxutil.NewRspTimeoutError("t"), 3, false},
		{"retries exhausted", 3, 3, nmxutil.NewRspTimeoutError("t"), 3,
			true},
		{"no retry on other error", 3, 1, fmt.Errorf("x"), 1, true},
	}

	for _, tt := range tests {
		fails := tt.failures
		s := newFakeSesn(func(req fakeReq) (nmp.NmpRsp, error) {
			if fails > 0 {
				fails--
				return nil, tt.err
			}
			return rspFor(req, map[string]interface{}{"rc": 0})
		})

		c := NewEchoCmd()
		opt := c.TxOptions()
		opt.Tries = tt.tries
		c.SetTxOptions(opt)

		_, err := runCmd(t, c, s)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err %v", tt.name, err)
		}
		if n := len(s.Requests()); n != tt.wantReqs {
			t.Errorf("%s: %d requests, want %d", tt.name, n, tt.wantReqs)
		}
	}
}

func TestCmdAbort(t *testing.T) {
	s := newFakeSesn(rcDev(0))

	c := NewEchoCmd()
	if err := c.Abort(); err != nil {
		t.Fatal(err)
	}
	_, err := runCmd(t, c, s)
	if err == nil || err.Error() != "Command aborted" {
		t.Errorf("err %v", err)
	}
	if n := len(s.Requests()); n != 0 {
		t.Errorf("%d requests after abort", n)
	}
	if len(s.abortSeqs) != 0 {
		t.Errorf("AbortRx called without a transaction in progress")
	}

	// A transaction in progress gets its receive aborted.
	c = NewEchoCmd()
	c.curSesn = s
	c.curNmpSeq = 7
	if err := c.Abort(); err != nil {
		t.Fatal(err)
	}
	if len(s.abortSeqs) != 1 || s.abortSeqs[0] != 7 {
		t.Errorf("AbortRx seqs %v", s.abortSeqs)
	}

	// A failing AbortRx is reported and does not mark the command
	// aborted.
	s.abortErr = fmt.Errorf("cannot abort")
	c = NewEchoCmd()
	c.curSesn = s
	if err := c.Abort(); err == nil {
		t.Errorf("AbortRx error not reported")
	}
	c.curSesn = nil
	if _, err := runCmd(t, c, s); err != nil {
		t.Errorf("command marked aborted: %v", err)
	}
}

func TestCmdTxOptions(t *testing.T) {
	c := NewEchoCmd()
	if c.TxOptions() != NewEchoCmd().TxOptions() {
		t.Errorf("default options differ")
	}

	opt := c.TxOptions()
	opt.Tries = 5
	opt.Timeout = 123
	c.SetTxOptions(opt)
	if c.TxOptions() != opt {
		t.Errorf("options %+v, want %+v", c.TxOptions(), opt)
	}
}

func logShowRsp(rc int, idxs ...uint32) *nmp.LogShowRsp {
	rsp := &nmp.LogShowRsp{Rc: rc}
	if idxs == nil {
		return rsp
	}

	l := nmp.LogShowLog{Name: "log"}
	for _, i := range idxs {
		l.Entries = append(l.Entries, nmp.LogEntry{Index: i})
	}
	rsp.Logs = []nmp.LogShowLog{l}
	return rsp
}

func TestLogShowFullRun(t *testing.T) {
	tests := []struct {
		name     string
		rsps     []*nmp.LogShowRsp
		wantIdxs []uint32
	}{
		{
			name:     "done at once",
			rsps:     []*nmp.LogShowRsp{logShowRsp(0, 3, 4)},
			wantIdxs: []uint32{10},
		},
		{
			name: "more logs",
			rsps: []*nmp.LogShowRsp{
				logShowRsp(1, 10, 11, 12),
				logShowRsp(1, 13, 20),
				logShowRsp(0, 21),
			},
			wantIdxs: []uint32{10, 13, 21},
		},
		{
			name: "rc 1 without logs",
			rsps: []*nmp.LogShowRsp{
				logShowRsp(1, 10),
				logShowRsp(1),
			},
			wantIdxs: []uint32{10, 11},
		},
		{
			name: "rc 1 without entries",
			rsps: []*nmp.LogShowRsp{
				{Rc: 1, Logs: []nmp.LogShowLog{{Name: "log"}}},
			},
			wantIdxs: []uint32{10},
		},
		{
			name:     "error",
			rsps:     []*nmp.LogShowRsp{logShowRsp(nmp.NMP_ERR_ENOENT)},
			wantIdxs: []uint32{10},
		},
	}

	for _, tt := range tests {
		i := 0
		s := newFakeSesn(func(req fakeReq) (nmp.NmpRsp, error) {
			if i >= len(tt.rsps) {
				return nil, fmt.Errorf("unexpected request")
			}
			i++
			return tt.rsps[i-1], nil
		})

		var cbs int
		c := NewLogShowFullCmd()
		c.Name = "log"
		c.Index = 10
		c.ProgressCb = func(*LogShowFullCmd, *nmp.LogShowRsp) { cbs++ }

		res, err := runCmd(t, c, s)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}

		reqs := s.Requests()
		if len(reqs) != len(tt.wantIdxs) {
			t.Fatalf("%s: %d requests, want %d", tt.name, len(reqs),
				len(tt.wantIdxs))
		}
		for j, r := range reqs {
			lr := r.Body.(*nmp.LogShowReq)
			if lr.Index != tt.wantIdxs[j] || lr.Name != "log" {
				t.Errorf("%s: req %d: name %q index %d, want %d", tt.name,
					j, lr.Name, lr.Index, tt.wantIdxs[j])
			}
		}

		lres := res.(*LogShowFullResult)
		if len(lres.Rsps) != len(reqs) || cbs != len(reqs) {
			t.Errorf("%s: %d rsps, %d callbacks for %d requests",
				tt.name, len(lres.Rsps), cbs, len(reqs))
		}
		if st := res.Status(); st != tt.rsps[len(tt.rsps)-1].Rc {
			t.Errorf("%s: status %d", tt.name, st)
		}
	}
}

func TestCoreLoadRun(t *testing.T) {
	core := testImage(1050)
	chunk := 100

	for _, failOff := range []int{-1, 0, 500} {
		s := newFakeSesn(func(req fakeReq) (nmp.NmpRsp, error) {
			r := req.Body.(*nmp.CoreLoadReq)
			off := int(r.Off)
			if failOff >= 0 && off >= failOff {
				return &nmp.CoreLoadRsp{Rc: nmp.NMP_ERR_ENOENT}, nil
			}
			end := off + chunk
			if end > len(core) {
				end = len(core)
			}
			return &nmp.CoreLoadRsp{
				Off:  r.Off,
				Len:  uint32(len(core)),
				Data: core[off:end],
			}, nil
		})

		var got []byte
		var cbs int
		c := NewCoreLoadCmd()
		c.ProgressCb = func(_ *CoreLoadCmd, r *nmp.CoreLoadRsp) {
			cbs++
			got = append(got, r.Data...)
		}

		res, err := runCmd(t, c, s)
		if err != nil {
			t.Fatalf("fail at %d: %v", failOff, err)
		}

		want := core
		wantSt := 0
		wantReqs := (len(core)+chunk-1)/chunk + 1
		if failOff >= 0 {
			want = core[:failOff]
			wantSt = nmp.NMP_ERR_ENOENT
			wantReqs = failOff/chunk + 1
		}
		if !bytes.Equal(got, want) {
			t.Errorf("fail at %d: got %d bytes, want %d", failOff,
				len(got), len(want))
		}
		if st := res.Status(); st != wantSt {
			t.Errorf("fail at %d: status %d", failOff, st)
		}
		reqs := s.Requests()
		if len(reqs) != wantReqs || cbs != wantReqs {
			t.Errorf("fail at %d: %d requests, %d callbacks, want %d",
				failOff, len(reqs), cbs, wantReqs)
		}
		for i, r := range reqs {
			wantOff := i * chunk
			if wantOff > len(core) {
				wantOff = len(core)
			}
			if int(r.Body.(*nmp.CoreLoadReq).Off) != wantOff ||
				r.Hdr.Op != nmp.NMP_OP_READ {

				t.Errorf("fail at %d: req %d: %+v", failOff, i, r.Hdr)
			}
		}
	}
}
