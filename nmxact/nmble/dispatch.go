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

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"

	log "github.com/Sirupsen/logrus"
)

type MsgBase struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Optional
	ConnHandle int `json:"conn_handle" json:",omitempty"`
}

type OpTypePair struct {
	Op   MsgOp
	Type MsgType
}

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	lm  *ListenerMap
	mtx sync.Mutex
}

type msgCtor func() Msg

func errRspCtor() Msg              { return &BleErrRsp{} }
func syncRspCtor() Msg             { return &BleSyncRsp{} }
func connectRspCtor() Msg          { return &BleConnectRsp{} }
func terminateRspCtor() Msg        { return &BleTerminateRsp{} }
func discAllSvcsRspCtor() Msg      { return &BleDiscAllSvcsRsp{} }
func discSvcUuidRspCtor() Msg      { return &BleDiscSvcUuidRsp{} }
func discAllChrsRspCtor() Msg      { return &BleDiscAllChrsRsp{} }
func discChrUuidRspCtor() Msg      { return &BleDiscChrUuidRsp{} }
func discAllDscsRspCtor() Msg      { return &BleDiscAllDscsRsp{} }
func writeRspCtor() Msg            { return &BleWriteRsp{} }
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
func advStartRspCtor() Msg         { return &BleAdvStartRsp{} }
func advStopRspCtor() Msg          { return &BleAdvStopRsp{} }
func advSetDataRspCtor() Msg       { return &BleAdvSetDataRsp{} }
func advRspSetDataRspCtor() Msg    { return &BleAdvRspSetDataRsp{} }
func advFieldsRspCtor() Msg        { return &BleAdvFieldsRsp{} }
func clearSvcsRspCtor() Msg        { return &BleClearSvcsRsp{} }
func addSvcsRspCtor() Msg          { return &BleAddSvcsRsp{} }
func commitSvcsRspCtor() Msg       { return &BleCommitSvcsRsp{} }
func accessStatusRspCtor() Msg     { return &BleAccessStatusRsp{} }
func notifyRspCtor() Msg           { return &BleNotifyRsp{} }
func findChrRspCtor() Msg          { return &BleFindChrRsp{} }
func oobSecDataRspCtor() Msg       { return &BleSmInjectIoRsp{} }

func syncEvtCtor() Msg        { return &BleSyncEvt{} }
func connectEvtCtor() Msg     { return &BleConnectEvt{} }
func connCancelEvtCtor() Msg  { return &BleConnCancelEvt{} }
func disconnectEvtCtor() Msg  { return &BleDisconnectEvt{} }
func discSvcEvtCtor() Msg     { return &BleDiscSvcEvt{} }
func discChrEvtCtor() Msg     { return &BleDiscChrEvt{} }
func discDscEvtCtor() Msg     { return &BleDiscDscEvt{} }
func writeAckEvtCtor() Msg    { return &BleWriteAckEvt{} }
func notifyRxEvtCtor() Msg    { return &BleNotifyRxEvt{} }
func mtuChangeEvtCtor() Msg   { return &BleMtuChangeEvt{} }
func scanEvtCtor() Msg        { return &BleScanEvt{} }
func scanTmoEvtCtor() Msg     { return &BleScanCompleteEvt{} }
func advCompleteEvtCtor() Msg { return &BleAdvCompleteEvt{} }
func encChangeEvtCtor() Msg   { return &BleEncChangeEvt{} }
func resetEvtCtor() Msg       { return &BleResetEvt{} }
func accessEvtCtor() Msg      { return &BleAccessEvt{} }
func passkeyEvtCtor() Msg     { return &BlePasskeyEvt{} }

var msgCtorMap = map[OpTypePair]msgCtor{
	{MSG_OP_RSP, MSG_TYPE_ERR}:               errRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SYNC}:              syncRspCtor,
	{MSG_OP_RSP, MSG_TYPE_CONNECT}:           connectRspCtor,
	{MSG_OP_RSP, MSG_TYPE_TERMINATE}:         terminateRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_ALL_SVCS}:     discAllSvcsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_SVC_UUID}:     discSvcUuidRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_ALL_CHRS}:     discAllChrsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_CHR_UUID}:     discChrUuidRspCtor,
	{MSG_OP_RSP, MSG_TYPE_DISC_ALL_DSCS}:     discAllDscsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_WRITE}:             writeRspCtor,
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
	{MSG_OP_RSP, MSG_TYPE_ADV_START}:         advStartRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ADV_STOP}:          advStopRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ADV_SET_DATA}:      advSetDataRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ADV_RSP_SET_DATA}:  advRspSetDataRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ADV_FIELDS}:        advFieldsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_CLEAR_SVCS}:        clearSvcsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ADD_SVCS}:          addSvcsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_COMMIT_SVCS}:       commitSvcsRspCtor,
	{MSG_OP_RSP, MSG_TYPE_ACCESS_STATUS}:     accessStatusRspCtor,
	{MSG_OP_RSP, MSG_TYPE_NOTIFY}:            notifyRspCtor,
	{MSG_OP_RSP, MSG_TYPE_FIND_CHR}:          findChrRspCtor,
	{MSG_OP_RSP, MSG_TYPE_SM_INJECT_IO}:      oobSecDataRspCtor,

	{MSG_OP_EVT, MSG_TYPE_SYNC_EVT}:          syncEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_CONNECT_EVT}:       connectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_CONN_CANCEL_EVT}:   connCancelEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISCONNECT_EVT}:    disconnectEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_SVC_EVT}:      discSvcEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_CHR_EVT}:      discChrEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_DISC_DSC_EVT}:      discDscEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_WRITE_ACK_EVT}:     writeAckEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_NOTIFY_RX_EVT}:     notifyRxEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_MTU_CHANGE_EVT}:    mtuChangeEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_SCAN_EVT}:          scanEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_SCAN_COMPLETE_EVT}: scanTmoEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_ADV_COMPLETE_EVT}:  advCompleteEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_ENC_CHANGE_EVT}:    encChangeEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_RESET_EVT}:         resetEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_ACCESS_EVT}:        accessEvtCtor,
	{MSG_OP_EVT, MSG_TYPE_PASSKEY_EVT}:       passkeyEvtCtor,
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		lm: NewListenerMap(),
	}
}

func (d *Dispatcher) AddListener(key ListenerKey, listener *Listener) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	return d.lm.AddListener(key, listener)
}

func (d *Dispatcher) RemoveListener(listener *Listener) *ListenerKey {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	key := d.lm.RemoveListener(listener)
	if key == nil {
		return nil
	}

	listener.Close()
	return key
}

func (d *Dispatcher) RemoveKey(key ListenerKey) *Listener {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	listener := d.lm.RemoveKey(key)
	if listener == nil {
		return nil
	}

	listener.Close()
	return listener
}

func (d *Dispatcher) ErrorAll(err error) {
	nmxutil.Assert(err != nil)

	d.mtx.Lock()
	listeners := d.lm.ExtractAll()
	d.mtx.Unlock()

	for _, listener := range listeners {
		listener.ErrChan <- err
		listener.Close()
	}
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
	defer d.mtx.Unlock()

	_, listener := d.lm.FindListener(base.Seq, base.Type, base.ConnHandle)

	if listener == nil {
		log.Debugf(
			"No BLE listener for op=%d type=%d seq=%d connHandle=%d",
			base.Op, base.Type, base.Seq, base.ConnHandle)
		return
	}

	listener.MsgChan <- msg
}
