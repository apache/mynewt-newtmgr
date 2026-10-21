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

package nmble

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func testMsgJson(t *testing.T, op MsgOp, typ MsgType, seq BleSeq,
	connHandle int) []byte {

	t.Helper()

	m := map[string]interface{}{
		"op":   MsgOpToString(op),
		"type": MsgTypeToString(typ),
		"seq":  seq,
	}
	if connHandle >= 0 {
		m["conn_handle"] = connHandle
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return b
}

func TestDecodeMsgAllTypes(t *testing.T) {
	tests := []struct {
		op   MsgOp
		typ  MsgType
		want Msg
	}{
		{MSG_OP_RSP, MSG_TYPE_ERR, &BleErrRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SYNC, &BleSyncRsp{}},
		{MSG_OP_RSP, MSG_TYPE_CONNECT, &BleConnectRsp{}},
		{MSG_OP_RSP, MSG_TYPE_TERMINATE, &BleTerminateRsp{}},
		{MSG_OP_RSP, MSG_TYPE_DISC_ALL_SVCS, &BleDiscAllSvcsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_DISC_SVC_UUID, &BleDiscSvcUuidRsp{}},
		{MSG_OP_RSP, MSG_TYPE_DISC_ALL_CHRS, &BleDiscAllChrsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_DISC_CHR_UUID, &BleDiscChrUuidRsp{}},
		{MSG_OP_RSP, MSG_TYPE_DISC_ALL_DSCS, &BleDiscAllDscsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_WRITE, &BleWriteRsp{}},
		{MSG_OP_RSP, MSG_TYPE_WRITE_CMD, &BleWriteCmdRsp{}},
		{MSG_OP_RSP, MSG_TYPE_EXCHANGE_MTU, &BleExchangeMtuRsp{}},
		{MSG_OP_RSP, MSG_TYPE_GEN_RAND_ADDR, &BleGenRandAddrRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SET_RAND_ADDR, &BleSetRandAddrRsp{}},
		{MSG_OP_RSP, MSG_TYPE_CONN_CANCEL, &BleConnCancelRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SCAN, &BleScanRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SCAN_CANCEL, &BleScanCancelRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SET_PREFERRED_MTU, &BleSetPreferredMtuRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SECURITY_INITIATE, &BleSecurityInitiateRsp{}},
		{MSG_OP_RSP, MSG_TYPE_CONN_FIND, &BleConnFindRsp{}},
		{MSG_OP_RSP, MSG_TYPE_RESET, &BleResetRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADV_START, &BleAdvStartRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADV_STOP, &BleAdvStopRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADV_SET_DATA, &BleAdvSetDataRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADV_RSP_SET_DATA, &BleAdvRspSetDataRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADV_FIELDS, &BleAdvFieldsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_CLEAR_SVCS, &BleClearSvcsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ADD_SVCS, &BleAddSvcsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_COMMIT_SVCS, &BleCommitSvcsRsp{}},
		{MSG_OP_RSP, MSG_TYPE_ACCESS_STATUS, &BleAccessStatusRsp{}},
		{MSG_OP_RSP, MSG_TYPE_NOTIFY, &BleNotifyRsp{}},
		{MSG_OP_RSP, MSG_TYPE_FIND_CHR, &BleFindChrRsp{}},
		{MSG_OP_RSP, MSG_TYPE_SM_INJECT_IO, &BleSmInjectIoRsp{}},
		{MSG_OP_EVT, MSG_TYPE_SYNC_EVT, &BleSyncEvt{}},
		{MSG_OP_EVT, MSG_TYPE_CONNECT_EVT, &BleConnectEvt{}},
		{MSG_OP_EVT, MSG_TYPE_CONN_CANCEL_EVT, &BleConnCancelEvt{}},
		{MSG_OP_EVT, MSG_TYPE_DISCONNECT_EVT, &BleDisconnectEvt{}},
		{MSG_OP_EVT, MSG_TYPE_DISC_SVC_EVT, &BleDiscSvcEvt{}},
		{MSG_OP_EVT, MSG_TYPE_DISC_CHR_EVT, &BleDiscChrEvt{}},
		{MSG_OP_EVT, MSG_TYPE_DISC_DSC_EVT, &BleDiscDscEvt{}},
		{MSG_OP_EVT, MSG_TYPE_WRITE_ACK_EVT, &BleWriteAckEvt{}},
		{MSG_OP_EVT, MSG_TYPE_NOTIFY_RX_EVT, &BleNotifyRxEvt{}},
		{MSG_OP_EVT, MSG_TYPE_MTU_CHANGE_EVT, &BleMtuChangeEvt{}},
		{MSG_OP_EVT, MSG_TYPE_SCAN_EVT, &BleScanEvt{}},
		{MSG_OP_EVT, MSG_TYPE_SCAN_COMPLETE_EVT, &BleScanCompleteEvt{}},
		{MSG_OP_EVT, MSG_TYPE_ADV_COMPLETE_EVT, &BleAdvCompleteEvt{}},
		{MSG_OP_EVT, MSG_TYPE_ENC_CHANGE_EVT, &BleEncChangeEvt{}},
		{MSG_OP_EVT, MSG_TYPE_RESET_EVT, &BleResetEvt{}},
		{MSG_OP_EVT, MSG_TYPE_ACCESS_EVT, &BleAccessEvt{}},
		{MSG_OP_EVT, MSG_TYPE_PASSKEY_EVT, &BlePasskeyEvt{}},
	}

	if len(tests) != len(msgCtorMap) {
		t.Errorf("table has %d entries, msgCtorMap has %d",
			len(tests), len(msgCtorMap))
	}

	for _, tt := range tests {
		name := MsgOpToString(tt.op) + "/" + MsgTypeToString(tt.typ)
		t.Run(name, func(t *testing.T) {
			data := testMsgJson(t, tt.op, tt.typ, 1234, 7)

			base, msg, err := decodeMsg(data)
			if err != nil {
				t.Fatalf("decodeMsg failed: %v", err)
			}
			if base.Op != tt.op || base.Type != tt.typ {
				t.Errorf("base op/type = %d/%d, want %d/%d",
					base.Op, base.Type, tt.op, tt.typ)
			}
			if base.Seq != 1234 {
				t.Errorf("base seq = %d, want 1234", base.Seq)
			}
			if base.ConnHandle != 7 {
				t.Errorf("base conn handle = %d, want 7", base.ConnHandle)
			}
			if reflect.TypeOf(msg) != reflect.TypeOf(tt.want) {
				t.Errorf("decoded %T, want %T", msg, tt.want)
			}
		})
	}
}

func TestDecodeMsgFields(t *testing.T) {
	data := []byte(`{"op":"event","type":"notify_rx_evt","seq":3,` +
		`"conn_handle":9,"attr_handle":17,"indication":true,` +
		`"data":"0x01:0x02:0x03"}`)

	_, msg, err := decodeMsg(data)
	if err != nil {
		t.Fatalf("decodeMsg failed: %v", err)
	}

	evt, ok := msg.(*BleNotifyRxEvt)
	if !ok {
		t.Fatalf("decoded %T, want *BleNotifyRxEvt", msg)
	}
	if evt.Seq != 3 || evt.ConnHandle != 9 || evt.AttrHandle != 17 ||
		!evt.Indication {

		t.Errorf("unexpected decoded event: %+v", evt)
	}
	if !reflect.DeepEqual(evt.Data.Bytes, []byte{1, 2, 3}) {
		t.Errorf("data = %x, want 010203", evt.Data.Bytes)
	}
}

func TestDecodeMsgInvalid(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"not json", `{"op":`},
		{"unknown op", `{"op":"bogus","type":"connect","seq":1}`},
		{"unknown type", `{"op":"response","type":"bogus","seq":1}`},
		{"request op", `{"op":"request","type":"connect","seq":1}`},
		{"event type as response",
			`{"op":"response","type":"connect_evt","seq":1}`},
		{"bad field type",
			`{"op":"response","type":"connect","seq":1,"status":"x"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, msg, err := decodeMsg([]byte(tt.data)); err == nil {
				t.Errorf("decodeMsg succeeded: %T %+v", msg, msg)
			}
		})
	}
}

func recvMsg(t *testing.T, l *Listener) Msg {
	t.Helper()

	select {
	case m := <-l.MsgChan:
		return m
	case err := <-l.ErrChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(testTimeout):
		t.Fatalf("timeout waiting for BLE message")
	}
	return nil
}

func addTestListener(t *testing.T, d *Dispatcher, key ListenerKey) *Listener {
	t.Helper()

	l := NewListener()
	if err := d.AddListener(key, l); err != nil {
		t.Fatalf("AddListener(%+v) failed: %v", key, err)
	}
	return l
}

func TestDispatcherRouting(t *testing.T) {
	const typ = MSG_TYPE_NOTIFY_RX_EVT

	tests := []struct {
		name       string
		seq        BleSeq
		typ        MsgType
		connHandle int
		want       string
	}{
		{"seq key wins", 5, typ, 3, "seq"},
		{"seq key any type", 5, MSG_TYPE_DISCONNECT_EVT, 3, "seq"},
		{"type and conn handle", BLE_SEQ_NONE, typ, 3, "tch"},
		{"type with any conn handle", BLE_SEQ_NONE, typ, 4, "type"},
		{"no conn handle in message", 6, typ, -1, "type"},
		{"no listener", 6, MSG_TYPE_DISCONNECT_EVT, 3, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher()
			lners := map[string]*Listener{
				"seq":  addTestListener(t, d, SeqKey(5)),
				"tch":  addTestListener(t, d, TchKey(typ, 3)),
				"type": addTestListener(t, d, TchKey(typ, -1)),
			}

			d.Dispatch(testMsgJson(t, MSG_OP_EVT, tt.typ, tt.seq,
				tt.connHandle))

			for name, l := range lners {
				if name != tt.want {
					if n := len(l.MsgChan); n != 0 {
						t.Errorf("listener %q got %d messages", name, n)
					}
					continue
				}

				m := recvMsg(t, l)
				want := msgCtorMap[OpTypePair{MSG_OP_EVT, tt.typ}]()
				if reflect.TypeOf(m) != reflect.TypeOf(want) {
					t.Errorf("got %T, want %T", m, want)
				}
			}
		})
	}
}

// Regression test for c2b0518: blehostd sends conn_cancel_evt with the
// sequence number of the connect request, so it must reach the listener
// registered for the connect procedure.
func TestDispatcherConnCancelEvtUsesConnectSeq(t *testing.T) {
	const connectSeq BleSeq = 100
	const cancelSeq BleSeq = 101

	d := NewDispatcher()
	connL := addTestListener(t, d, SeqKey(connectSeq))
	cancelL := addTestListener(t, d, SeqKey(cancelSeq))

	d.Dispatch(testMsgJson(t, MSG_OP_RSP, MSG_TYPE_CONN_CANCEL, cancelSeq,
		-1))
	d.Dispatch(testMsgJson(t, MSG_OP_EVT, MSG_TYPE_CONN_CANCEL_EVT,
		connectSeq, -1))

	if m, ok := recvMsg(t, cancelL).(*BleConnCancelRsp); !ok {
		t.Errorf("cancel listener got %T, want *BleConnCancelRsp", m)
	}
	if m, ok := recvMsg(t, connL).(*BleConnCancelEvt); !ok {
		t.Errorf("connect listener got %T, want *BleConnCancelEvt", m)
	}
	if n := len(cancelL.MsgChan); n != 0 {
		t.Errorf("cancel listener got %d extra messages", n)
	}
}

func TestDispatcherDropsInvalid(t *testing.T) {
	d := NewDispatcher()
	l := addTestListener(t, d, SeqKey(1))

	for _, data := range []string{
		`garbage`,
		`{"op":"bogus","type":"connect","seq":1}`,
		`{"op":"response","type":"bogus","seq":1}`,
	} {
		d.Dispatch([]byte(data))
	}

	if n := len(l.MsgChan); n != 0 {
		t.Errorf("listener got %d messages from invalid input", n)
	}
}

func TestDispatcherAddRemove(t *testing.T) {
	d := NewDispatcher()
	key := SeqKey(9)
	l := addTestListener(t, d, key)

	if err := d.AddListener(key, NewListener()); err == nil {
		t.Errorf("duplicate key accepted")
	}
	if err := d.AddListener(SeqKey(10), l); err == nil {
		t.Errorf("duplicate listener accepted")
	}

	got := d.RemoveListener(l)
	if got == nil || *got != key {
		t.Fatalf("RemoveListener returned %v, want %+v", got, key)
	}
	if _, ok := <-l.MsgChan; ok {
		t.Errorf("MsgChan not closed")
	}
	if got := d.RemoveListener(l); got != nil {
		t.Errorf("second RemoveListener returned %+v", *got)
	}

	l2 := addTestListener(t, d, key)
	if got := d.RemoveKey(key); got != l2 {
		t.Errorf("RemoveKey returned %p, want %p", got, l2)
	}
	if got := d.RemoveKey(key); got != nil {
		t.Errorf("second RemoveKey returned %p", got)
	}

	d.Dispatch(testMsgJson(t, MSG_OP_RSP, MSG_TYPE_CONNECT, 9, -1))
}

func TestDispatcherErrorAll(t *testing.T) {
	d := NewDispatcher()

	var lners []*Listener
	for i := 0; i < 3; i++ {
		lners = append(lners, addTestListener(t, d, SeqKey(BleSeq(i))))
	}
	lners = append(lners, addTestListener(t, d,
		TchKey(MSG_TYPE_DISCONNECT_EVT, -1)))

	want := errors.New("xport stopped")
	d.ErrorAll(want)

	for i, l := range lners {
		select {
		case err := <-l.ErrChan:
			if err != want {
				t.Errorf("listener %d: got %v, want %v", i, err, want)
			}
		case <-time.After(testTimeout):
			t.Fatalf("listener %d: no error delivered", i)
		}
		if _, ok := <-l.MsgChan; ok {
			t.Errorf("listener %d: MsgChan not closed", i)
		}
	}

	// All listeners were removed; keys can be reused.
	addTestListener(t, d, SeqKey(0))
}

func TestListenerMapFind(t *testing.T) {
	lm := NewListenerMap()
	seqL := NewListener()
	tchL := NewListener()
	anyL := NewListener()

	for _, e := range []struct {
		key ListenerKey
		l   *Listener
	}{
		{SeqKey(1), seqL},
		{TchKey(MSG_TYPE_WRITE_ACK_EVT, 2), tchL},
		{TchKey(MSG_TYPE_WRITE_ACK_EVT, -1), anyL},
	} {
		if err := lm.AddListener(e.key, e.l); err != nil {
			t.Fatalf("AddListener failed: %v", err)
		}
	}

	tests := []struct {
		seq     BleSeq
		typ     MsgType
		conn    int
		wantKey ListenerKey
		want    *Listener
	}{
		{1, MSG_TYPE_WRITE_ACK_EVT, 2, SeqKey(1), seqL},
		{2, MSG_TYPE_WRITE_ACK_EVT, 2,
			TchKey(MSG_TYPE_WRITE_ACK_EVT, 2), tchL},
		{2, MSG_TYPE_WRITE_ACK_EVT, 3,
			TchKey(MSG_TYPE_WRITE_ACK_EVT, -1), anyL},
		{2, MSG_TYPE_NOTIFY_RX_EVT, 2,
			TchKey(MSG_TYPE_NOTIFY_RX_EVT, -1), nil},
	}

	for i, tt := range tests {
		key, l := lm.FindListener(tt.seq, tt.typ, tt.conn)
		if l != tt.want {
			t.Errorf("%d: listener = %p, want %p", i, l, tt.want)
		}
		if key != tt.wantKey {
			t.Errorf("%d: key = %+v, want %+v", i, key, tt.wantKey)
		}
	}
}

func TestListenerMapAddRemove(t *testing.T) {
	lm := NewListenerMap()
	l1 := NewListener()
	l2 := NewListener()
	k1 := SeqKey(1)
	k2 := TchKey(MSG_TYPE_CONNECT_EVT, 5)

	if err := lm.AddListener(k1, l1); err != nil {
		t.Fatal(err)
	}
	if err := lm.AddListener(k1, l2); err == nil {
		t.Errorf("duplicate key accepted")
	}
	if err := lm.AddListener(k2, l1); err == nil {
		t.Errorf("duplicate listener accepted")
	}
	if err := lm.AddListener(k2, l2); err != nil {
		t.Fatal(err)
	}

	if got := lm.RemoveListener(l1); got == nil || *got != k1 {
		t.Errorf("RemoveListener(l1) = %v, want %+v", got, k1)
	}
	if got := lm.RemoveListener(l1); got != nil {
		t.Errorf("second RemoveListener(l1) = %+v", *got)
	}
	if got := lm.RemoveKey(k2); got != l2 {
		t.Errorf("RemoveKey(k2) = %p, want %p", got, l2)
	}
	if got := lm.RemoveKey(k2); got != nil {
		t.Errorf("second RemoveKey(k2) = %p", got)
	}
	if _, l := lm.FindListener(1, MSG_TYPE_CONNECT_EVT, 5); l != nil {
		t.Errorf("found removed listener")
	}

	// Both can be added again.
	if err := lm.AddListener(k1, l1); err != nil {
		t.Errorf("re-adding l1 failed: %v", err)
	}
	if err := lm.AddListener(k2, l2); err != nil {
		t.Errorf("re-adding l2 failed: %v", err)
	}
}

func TestListenerMapExtractAll(t *testing.T) {
	lm := NewListenerMap()
	want := map[*Listener]bool{}
	for i := 0; i < 4; i++ {
		l := NewListener()
		if err := lm.AddListener(SeqKey(BleSeq(i)), l); err != nil {
			t.Fatal(err)
		}
		want[l] = true
	}

	got := lm.ExtractAll()
	if len(got) != len(want) {
		t.Fatalf("ExtractAll returned %d listeners, want %d",
			len(got), len(want))
	}
	for _, l := range got {
		if !want[l] {
			t.Errorf("unexpected listener %p", l)
		}
	}

	if got := lm.ExtractAll(); len(got) != 0 {
		t.Errorf("second ExtractAll returned %d listeners", len(got))
	}
	if _, l := lm.FindListener(0, MSG_TYPE_CONNECT_EVT, 0); l != nil {
		t.Errorf("listener still present after ExtractAll")
	}
}

func TestListenerTimeout(t *testing.T) {
	l := NewListener()

	select {
	case <-l.AfterTimeout(time.Millisecond):
	case <-time.After(testTimeout):
		t.Fatalf("timeout did not fire")
	}

	l.Close()
	for name, closed := range map[string]bool{
		"MsgChan": isClosedMsg(l.MsgChan),
		"ErrChan": isClosedErr(l.ErrChan),
		"TmoChan": isClosedTmo(l.TmoChan),
	} {
		if !closed {
			t.Errorf("%s not closed", name)
		}
	}
}

func TestListenerCloseDrains(t *testing.T) {
	l := NewListener()
	l.MsgChan <- &BleSyncEvt{}
	l.ErrChan <- fmt.Errorf("pending")
	l.AfterTimeout(time.Hour)

	l.Close()

	if !isClosedMsg(l.MsgChan) {
		t.Errorf("MsgChan not drained and closed")
	}

	// A pending error is kept for the owner.
	if err, ok := <-l.ErrChan; !ok || err == nil {
		t.Errorf("pending error lost by Close")
	}
	if !isClosedErr(l.ErrChan) {
		t.Errorf("ErrChan not closed")
	}
	if l.timer.Stop() {
		t.Errorf("timer still active after Close")
	}
}

func isClosedMsg(c chan Msg) bool {
	select {
	case _, ok := <-c:
		return !ok
	default:
		return false
	}
}

func isClosedErr(c chan error) bool {
	select {
	case _, ok := <-c:
		return !ok
	default:
		return false
	}
}

func isClosedTmo(c chan time.Time) bool {
	select {
	case _, ok := <-c:
		return !ok
	default:
		return false
	}
}

// ErrorAll must deliver the error even if the listener owner is not
// blocked on ErrChan at that moment (e.g. it is busy handling a message).
// Previously Close() drained the error that ErrorAll had just queued, so
// the owner only saw a closed channel and a nil error.
func TestDispatcherErrorAllDeliversError(t *testing.T) {
	d := NewDispatcher()
	l := addTestListener(t, d, SeqKey(1))

	want := errors.New("xport stopped")
	d.ErrorAll(want)

	err, ok := <-l.ErrChan
	if !ok {
		t.Fatalf("ErrChan closed without delivering the error")
	}
	if err != want {
		t.Fatalf("got %v, want %v", err, want)
	}
	if _, ok := <-l.ErrChan; ok {
		t.Errorf("ErrChan not closed after the error")
	}
}

// The timer callback may run concurrently with Close.  It must neither
// race with Close nor panic with "send on closed channel".  The race is
// timing dependent, so try many times.
func TestListenerTimeoutRacesClose(t *testing.T) {
	for i := 0; i < 2000; i++ {
		l := NewListener()
		l.AfterTimeout(0)
		l.Close()
	}
}
