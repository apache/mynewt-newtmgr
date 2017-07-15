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
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

type OpTypePair struct {
	Op   MsgOp
	Type MsgType
}

type MsgBase struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Optional
	ConnHandle int `json:"conn_handle" json:",omitempty"`
}

type Listener struct {
	MsgChan chan Msg
	ErrChan chan error
	TmoChan chan time.Time
	Acked   bool

	timer *time.Timer
}

func NewListener() *Listener {
	return &Listener{
		MsgChan: make(chan Msg, 16),
		ErrChan: make(chan error, 1),
		TmoChan: make(chan time.Time, 1),
	}
}

func (bl *Listener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		if !bl.Acked {
			bl.TmoChan <- time.Now()
		}
	}
	bl.timer = time.AfterFunc(tmo, fn)
	return bl.TmoChan
}

func (bl *Listener) Close() {
	if bl.timer != nil {
		bl.timer.Stop()
	}

	close(bl.MsgChan)
	close(bl.ErrChan)
	close(bl.TmoChan)
}

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	seqMap  map[BleSeq]*Listener
	baseMap map[MsgBase]*Listener
	mtx     sync.Mutex
}

type msgCtor func() Msg

func errRspCtor() Msg              { return &BleErrRsp{} }
func syncRspCtor() Msg             { return &BleSyncRsp{} }
func connectRspCtor() Msg          { return &BleConnectRsp{} }
func terminateRspCtor() Msg        { return &BleTerminateRsp{} }
func discSvcUuidRspCtor() Msg      { return &BleDiscSvcUuidRsp{} }
func discAllChrsRspCtor() Msg      { return &BleDiscAllChrsRsp{} }
func discChrUuidRspCtor() Msg      { return &BleDiscChrUuidRsp{} }
func writeCmdRspCtor() Msg         { return &BleWriteCmdRsp{} }
func exchangeMtuRspCtor() Msg      { return &BleExchangeMtuRsp{} }
func genRandAddrRspCtor() Msg      { return &BleGenRandAddrRsp{} }
func setRandAddrRspCtor() Msg      { return &BleSetRandAddrRsp{} }
func connCancelRspCtor() Msg       { return &BleConnCancelRsp{} }
func scanRspCtor() Msg             { return &BleScanRsp{} }
func scanCancelRspCtor() Msg       { return &BleScanCancelRsp{} }
func setPreferredMtuRspCtor() Msg  { return &BleSetPreferredMtuRsp{} }
func securityInitiateRspCtor() Msg { return &BleSecurityInitiateRsp{} }
func connFindRspCtor() Msg         { return &BleConnFindRsp{} }
func resetRspCtor() Msg            { return &BleResetRsp{} }

func syncEvtCtor() Msg       { return &BleSyncEvt{} }
func connectEvtCtor() Msg    { return &BleConnectEvt{} }
func disconnectEvtCtor() Msg { return &BleDisconnectEvt{} }
func discSvcEvtCtor() Msg    { return &BleDiscSvcEvt{} }
func discChrEvtCtor() Msg    { return &BleDiscChrEvt{} }
func notifyRxEvtCtor() Msg   { return &BleNotifyRxEvt{} }
func mtuChangeEvtCtor() Msg  { return &BleMtuChangeEvt{} }
func scanEvtCtor() Msg       { return &BleScanEvt{} }
func scanTmoEvtCtor() Msg    { return &BleScanTmoEvt{} }
func encChangeEvtCtor() Msg  { return &BleEncChangeEvt{} }

var msgCtorMap = map[OpTypePair]msgCtor{
	{MSG_OP_RSP, MSG_TYPE_ERR}:               errRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SYNC}:              syncRspCtor,
	{MSG_OP_RSP, MSG_TYPE_CONNECT}:           connectRspCtor,
	{MSG_OP_RSP, MSG_TYPE_TERMINATE}:         terminateRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_SVC_UUID}:     discSvcUuidRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_CHR_UUID}:     discChrUuidRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_ALL_CHRS}:     discAllChrsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_WRITE_CMD}:         writeCmdRspCtor,
	{MSG_OP_RSP, MSG_TYPE_EXCHANGE_MTU}:      exchangeMtuRspCtor,
	{MSG_OP_RSP, MSG_TYPE_GEN_RAND_ADDR}:     genRandAddrRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SET_RAND_ADDR}:     setRandAddrRspCtor,
	{MSG_OP_RSP, MSG_TYPE_CONN_CANCEL}:       connCancelRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SCAN}:              scanRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SCAN_CANCEL}:       scanCancelRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SET_PREFERRED_MTU}: setPreferredMtuRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SECURITY_INITIATE}: securityInitiateRspCtor,
	{MSG_OP_RSP, MSG_TYPE_CONN_FIND}:         connFindRspCtor,
	{MSG_OP_RSP, MSG_TYPE_RESET}:             resetRspCtor,

	{MSG_OP_EVT, MSG_TYPE_SYNC_EVT}:       syncEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_CONNECT_EVT}:    connectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISCONNECT_EVT}: disconnectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_SVC_EVT}:   discSvcEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_CHR_EVT}:   discChrEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_NOTIFY_RX_EVT}:  notifyRxEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_MTU_CHANGE_EVT}: mtuChangeEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_SCAN_EVT}:       scanEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_SCAN_TMO_EVT}:   scanTmoEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_ENC_CHANGE_EVT}: encChangeEvtCtor,
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		seqMap:  map[BleSeq]*Listener{},
		baseMap: map[MsgBase]*Listener{},
	}
}

func (d *Dispatcher) findBaseListener(base MsgBase) (
	MsgBase, *Listener) {

	for k, v := range d.baseMap {
		if k.Op != -1 && base.Op != -1 && k.Op != base.Op {
			continue
		}
		if k.Type != -1 && base.Type != -1 && k.Type != base.Type {
			continue
		}
		if k.ConnHandle != -1 && base.ConnHandle != -1 &&
			k.ConnHandle != base.ConnHandle {

			continue
		}

		return k, v
	}

	return base, nil
}

func (d *Dispatcher) findDupListener(base MsgBase) (
	MsgBase, *Listener) {

	if base.Seq != BLE_SEQ_NONE {
		return base, d.seqMap[base.Seq]
	}

	return d.findBaseListener(base)
}

func (d *Dispatcher) findListener(base MsgBase) (
	MsgBase, *Listener) {

	if base.Seq != BLE_SEQ_NONE {
		if bl := d.seqMap[base.Seq]; bl != nil {
			return base, bl
		}
	}

	return d.findBaseListener(base)
}

func (d *Dispatcher) AddListener(base MsgBase,
	listener *Listener) error {

	d.mtx.Lock()
	defer d.mtx.Unlock()

	if ob, old := d.findDupListener(base); old != nil {
		return fmt.Errorf(
			"Duplicate BLE listener;\n"+
				"    old=op=%d type=%d seq=%d connHandle=%d\n"+
				"    new=op=%d type=%d seq=%d connHandle=%d",
			ob.Op, ob.Type, ob.Seq, ob.ConnHandle,
			base.Op, base.Type, base.Seq, base.ConnHandle)
	}

	if base.Seq != BLE_SEQ_NONE {
		if base.Op != -1 ||
			base.Type != -1 ||
			base.ConnHandle != -1 {
			return fmt.Errorf(
				"Invalid listener base; non-wild seq with wild fields")
		}

		d.seqMap[base.Seq] = listener
	} else {
		d.baseMap[base] = listener
	}

	return nil
}

func (d *Dispatcher) RemoveListener(base MsgBase) *Listener {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	base, bl := d.findListener(base)
	if bl != nil {
		bl.Close()
		if base.Seq != BLE_SEQ_NONE {
			delete(d.seqMap, base.Seq)
		} else {
			delete(d.baseMap, base)
		}
	}

	return bl
}

func decodeBleBase(data []byte) (MsgBase, error) {
	base := MsgBase{}
	if err := json.Unmarshal(data, &base); err != nil {
		return base, err
	}

	return base, nil
}

func decodeMsg(data []byte) (MsgBase, Msg, error) {
	base, err := decodeBleBase(data)
	if err != nil {
		return base, nil, err
	}

	opTypePair := OpTypePair{base.Op, base.Type}
	cb := msgCtorMap[opTypePair]
	if cb == nil {
		return base, nil, fmt.Errorf(
			"Unrecognized op+type pair: %s, %s",
			MsgOpToString(base.Op), MsgTypeToString(base.Type))
	}

	msg := cb()
	if err := json.Unmarshal(data, msg); err != nil {
		return base, nil, err
	}

	return base, msg, nil
}

func (d *Dispatcher) Dispatch(data []byte) {
	base, msg, err := decodeMsg(data)
	if err != nil {
		log.Warnf("BLE dispatch error: %s", err.Error())
		return
	}

	d.mtx.Lock()
	_, listener := d.findListener(base)
	d.mtx.Unlock()

	if listener == nil {
		log.Debugf(
			"No BLE listener for op=%d type=%d seq=%d connHandle=%d",
			base.Op, base.Type, base.Seq, base.ConnHandle)
		return
	}

	listener.MsgChan <- msg
}

func (d *Dispatcher) ErrorAll(err error) {
	if err == nil {
		panic("NIL ERROR")
	}

	d.mtx.Lock()

	m1 := d.seqMap
	d.seqMap = map[BleSeq]*Listener{}

	m2 := d.baseMap
	d.baseMap = map[MsgBase]*Listener{}

	d.mtx.Unlock()

	for _, bl := range m1 {
		bl.ErrChan <- err
		bl.Close()
	}
	for _, bl := range m2 {
		bl.ErrChan <- err
		bl.Close()
	}
}
