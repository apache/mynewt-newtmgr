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
	"sync"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

func coapReply(isTcp bool, req coap.Message,
	code coap.COAPCode) coap.Message {

	p := coap.MessageParams{
		Type:      coap.Acknowledgement,
		Code:      code,
		MessageID: req.MessageID(),
		Token:     req.Token(),
		Payload:   []byte{0xa0},
	}
	if isTcp {
		return coap.NewTcpMessage(p)
	}
	return coap.NewDgramMessage(p)
}

// All 2.xx codes are success (5029f07).
func TestResResultStatus(t *testing.T) {
	tests := []struct {
		code coap.COAPCode
		want int
	}{
		{coap.Created, 0},
		{coap.Deleted, 0},
		{coap.Valid, 0},
		{coap.Changed, 0},
		{coap.Content, 0},
		{coap.BadRequest, int(coap.BadRequest)},
		{coap.Unauthorized, int(coap.Unauthorized)},
		{coap.NotFound, int(coap.NotFound)},
		{coap.MethodNotAllowed, int(coap.MethodNotAllowed)},
		{coap.InternalServerError, int(coap.InternalServerError)},
	}

	for _, tt := range tests {
		r := &ResResult{
			Rsp: coap.NewDgramMessage(coap.MessageParams{Code: tt.code}),
		}
		if st := r.Status(); st != tt.want {
			t.Errorf("code %v: status %d, want %d", tt.code, st, tt.want)
		}
	}
}

func TestResCmdRun(t *testing.T) {
	for _, isTcp := range []bool{false, true} {
		for _, code := range []coap.COAPCode{coap.Content, coap.NotFound} {
			s := newFakeSesn(nil)
			s.isTcp = isTcp
			s.coapHandler = func(m coap.Message) coap.Message {
				return coapReply(isTcp, m, code)
			}

			c := NewResCmd()
			c.MsgParams = nmcoap.MsgParams{
				Code:    coap.PUT,
				Uri:     "/a/b",
				Token:   []byte{1, 2, 3, 4},
				Payload: []byte{0xa1, 0x61, 0x78, 0x01},
			}

			res, err := runCmd(t, c, s)
			if err != nil {
				t.Fatalf("tcp %v code %v: %v", isTcp, code, err)
			}
			rres := res.(*ResResult)
			if rres.Rsp.Code() != code {
				t.Errorf("tcp %v: rsp code %v, want %v", isTcp,
					rres.Rsp.Code(), code)
			}

			if len(s.coapTx) != 1 {
				t.Fatalf("tcp %v: %d messages sent", isTcp, len(s.coapTx))
			}
			m := s.coapTx[0]
			if m.Code() != coap.PUT || m.PathString() != "a/b" ||
				!bytes.Equal(m.Token(), c.MsgParams.Token) ||
				!bytes.Equal(m.Payload(), c.MsgParams.Payload) {

				t.Errorf("tcp %v: request code %v path %q token %x",
					isTcp, m.Code(), m.PathString(), m.Token())
			}
			if len(s.listeners) != 0 {
				t.Errorf("tcp %v: listener not removed", isTcp)
			}
		}
	}
}

// A request without a response is retried on timeout.
func TestResCmdRetry(t *testing.T) {
	tests := []struct {
		name    string
		tries   int
		drops   int
		wantTx  int
		wantErr bool
	}{
		{"no retry", 1, 1, 1, true},
		{"retry ok", 3, 2, 3, false},
		{"retries exhausted", 2, 2, 2, true},
	}

	for _, tt := range tests {
		var mtx sync.Mutex
		drops := tt.drops

		s := newFakeSesn(nil)
		s.coapHandler = func(m coap.Message) coap.Message {
			mtx.Lock()
			defer mtx.Unlock()

			if drops > 0 {
				drops--
				return nil
			}
			return coapReply(false, m, coap.Changed)
		}

		c := NewResCmd()
		c.MsgParams = nmcoap.MsgParams{Code: coap.GET, Uri: "/x"}
		opt := c.TxOptions()
		opt.Tries = tt.tries
		opt.Timeout = 20 * time.Millisecond
		c.SetTxOptions(opt)

		res, err := runCmd(t, c, s)
		if tt.wantErr {
			if !nmxutil.IsRspTimeout(err) {
				t.Errorf("%s: err %v, want timeout", tt.name, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", tt.name, err)
		} else if res.Status() != 0 {
			t.Errorf("%s: status %d", tt.name, res.Status())
		}

		if len(s.coapTx) != tt.wantTx {
			t.Errorf("%s: %d messages sent, want %d", tt.name,
				len(s.coapTx), tt.wantTx)
		}
	}
}

func TestResNoRxCmdRun(t *testing.T) {
	s := newFakeSesn(nil)

	c := NewResNoRxCmd()
	c.MsgParams = nmcoap.MsgParams{Code: coap.POST, Uri: "/r"}

	res, err := runCmd(t, c, s)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status() != 0 {
		t.Errorf("status %d", res.Status())
	}
	if len(s.coapTx) != 1 || s.coapTx[0].Code() != coap.POST {
		t.Errorf("sent %v", s.coapTx)
	}
}
