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

type BleMsgBase struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Optional
	ConnHandle int `json:"conn_handle" json:",omitempty"`
}

type BleListener struct {
	BleChan chan BleMsg
	ErrChan chan error
	TmoChan chan time.Time
	Acked   bool

	timer *time.Timer
}

func NewBleListener() *BleListener {
	return &BleListener{
		BleChan: make(chan BleMsg, 16),
		ErrChan: make(chan error, 4),
		TmoChan: make(chan time.Time, 1),
	}
}

func (bl *BleListener) AfterTimeout(tmo time.Duration) <-chan time.Time {
	fn := func() {
		if !bl.Acked {
			bl.TmoChan <- time.Now()
		}
	}
	bl.timer = time.AfterFunc(tmo, fn)
	return bl.TmoChan
}

func (bl *BleListener) Stop() {
	if bl.timer != nil {
		bl.timer.Stop()
	}
}

type BleDispatcher struct {
	seqMap  map[BleSeq]*BleListener
	baseMap map[BleMsgBase]*BleListener
	mutex   sync.Mutex
}

type msgCtor func() BleMsg

func errRspCtor() BleMsg             { return &BleErrRsp{} }
func syncRspCtor() BleMsg            { return &BleSyncRsp{} }
func connectRspCtor() BleMsg         { return &BleConnectRsp{} }
func terminateRspCtor() BleMsg       { return &BleTerminateRsp{} }
func discSvcUuidRspCtor() BleMsg     { return &BleDiscSvcUuidRsp{} }
func discAllChrsRspCtor() BleMsg     { return &BleDiscAllChrsRsp{} }
func discChrUuidRspCtor() BleMsg     { return &BleDiscChrUuidRsp{} }
func writeCmdRspCtor() BleMsg        { return &BleWriteCmdRsp{} }
func exchangeMtuRspCtor() BleMsg     { return &BleExchangeMtuRsp{} }
func genRandAddrRspCtor() BleMsg     { return &BleGenRandAddrRsp{} }
func setRandAddrRspCtor() BleMsg     { return &BleSetRandAddrRsp{} }
func connCancelRspCtor() BleMsg      { return &BleConnCancelRsp{} }
func scanRspCtor() BleMsg            { return &BleScanRsp{} }
func scanCancelRspCtor() BleMsg      { return &BleScanCancelRsp{} }
func setPreferredMtuRspCtor() BleMsg { return &BleSetPreferredMtuRsp{} }
func connFindRspCtor() BleMsg        { return &BleConnFindRsp{} }

func syncEvtCtor() BleMsg       { return &BleSyncEvt{} }
func connectEvtCtor() BleMsg    { return &BleConnectEvt{} }
func disconnectEvtCtor() BleMsg { return &BleDisconnectEvt{} }
func discSvcEvtCtor() BleMsg    { return &BleDiscSvcEvt{} }
func discChrEvtCtor() BleMsg    { return &BleDiscChrEvt{} }
func notifyRxEvtCtor() BleMsg   { return &BleNotifyRxEvt{} }
func mtuChangeEvtCtor() BleMsg  { return &BleMtuChangeEvt{} }
func scanEvtCtor() BleMsg       { return &BleScanEvt{} }

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
	{MSG_OP_RSP, MSG_TYPE_CONN_FIND}:         connFindRspCtor,

	{MSG_OP_EVT, MSG_TYPE_SYNC_EVT}:       syncEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_CONNECT_EVT}:    connectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISCONNECT_EVT}: disconnectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_SVC_EVT}:   discSvcEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_CHR_EVT}:   discChrEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_NOTIFY_RX_EVT}:  notifyRxEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_MTU_CHANGE_EVT}: mtuChangeEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_SCAN_EVT}:       scanEvtCtor,
}

func NewBleDispatcher() *BleDispatcher {
	return &BleDispatcher{
		seqMap:  map[BleSeq]*BleListener{},
		baseMap: map[BleMsgBase]*BleListener{},
	}
}

func (bd *BleDispatcher) findBaseListener(base BleMsgBase) (
	BleMsgBase, *BleListener) {

	for k, v := range bd.baseMap {
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

func (bd *BleDispatcher) findDupListener(base BleMsgBase) (
	BleMsgBase, *BleListener) {

	if base.Seq != BLE_SEQ_NONE {
		return base, bd.seqMap[base.Seq]
	}

	return bd.findBaseListener(base)
}

func (bd *BleDispatcher) findListener(base BleMsgBase) (
	BleMsgBase, *BleListener) {

	if base.Seq != BLE_SEQ_NONE {
		if bl := bd.seqMap[base.Seq]; bl != nil {
			return base, bl
		}
	}

	return bd.findBaseListener(base)
}

func (bd *BleDispatcher) AddListener(base BleMsgBase,
	listener *BleListener) error {

	bd.mutex.Lock()
	defer bd.mutex.Unlock()

	if ob, old := bd.findDupListener(base); old != nil {
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

		bd.seqMap[base.Seq] = listener
	} else {
		bd.baseMap[base] = listener
	}

	return nil
}

func (bd *BleDispatcher) RemoveListener(base BleMsgBase) *BleListener {
	bd.mutex.Lock()
	defer bd.mutex.Unlock()

	base, bl := bd.findListener(base)
	if bl != nil {
		bl.Stop()
		if base.Seq != BLE_SEQ_NONE {
			delete(bd.seqMap, base.Seq)
		} else {
			delete(bd.baseMap, base)
		}
	}

	return bl
}

func decodeBleBase(data []byte) (BleMsgBase, error) {
	base := BleMsgBase{}
	if err := json.Unmarshal(data, &base); err != nil {
		return base, err
	}

	return base, nil
}

func decodeBleMsg(data []byte) (BleMsgBase, BleMsg, error) {
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

func (bd *BleDispatcher) Dispatch(data []byte) {
	base, msg, err := decodeBleMsg(data)
	if err != nil {
		log.Warnf("BLE dispatch error: %s", err.Error())
		return
	}

	bd.mutex.Lock()
	_, listener := bd.findListener(base)
	bd.mutex.Unlock()

	if listener == nil {
		log.Debugf(
			"No BLE listener for op=%d type=%d seq=%d connHandle=%d",
			base.Op, base.Type, base.Seq, base.ConnHandle)
		return
	}

	listener.BleChan <- msg
}

func (bd *BleDispatcher) ErrorAll(err error) {
	bd.mutex.Lock()

	listeners := make([]*BleListener, 0, len(bd.seqMap)+len(bd.baseMap))
	for _, v := range bd.seqMap {
		listeners = append(listeners, v)
	}
	for _, v := range bd.baseMap {
		listeners = append(listeners, v)
	}

	bd.clear()

	bd.mutex.Unlock()

	for _, listener := range listeners {
		listener.ErrChan <- err
	}
}

// The caller must lock the mutex.
func (bd *BleDispatcher) clear() {
	for s, _ := range bd.seqMap {
		delete(bd.seqMap, s)
	}
	for b, _ := range bd.baseMap {
		delete(bd.baseMap, b)
	}
}
