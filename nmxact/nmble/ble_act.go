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
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// Blocking
func connect(x *BleXport, bl *Listener, r *BleConnectReq,
	timeout time.Duration) (uint16, error) {

	j, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return 0, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleConnectRsp:
				bl.Acked = true
				if msg.Status != 0 {
					str := fmt.Sprintf("BLE connection attempt failed; "+
						"status=%s (%d)",
						ErrCodeToString(msg.Status), msg.Status)
					log.Debugf(str)
					return 0, nmxutil.NewBleHostError(msg.Status, str)
				}

			case *BleConnectEvt:
				if msg.Status == 0 {
					return msg.ConnHandle, nil
				} else {
					str := fmt.Sprintf("BLE connection attempt failed; "+
						"status=%s (%d)",
						ErrCodeToString(msg.Status), msg.Status)
					log.Debugf(str)
					return 0, nmxutil.NewBleHostError(msg.Status, str)
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return 0, BhdTimeoutError(MSG_TYPE_CONNECT, r.Seq)

		case <-time.After(timeout):
			return 0, fmt.Errorf("Failed to connect to peer after %s",
				timeout.String())
		}
	}
}

// Blocking
func terminate(x *BleXport, bl *Listener, r *BleTerminateReq) error {
	const rspType = MSG_TYPE_TERMINATE

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleTerminateRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				} else {
					return nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

func connCancel(x *BleXport, bl *Listener, r *BleConnCancelReq) error {
	const rspType = MSG_TYPE_CONN_CANCEL

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleConnCancelRsp:
				bl.Acked = true
				if msg.Status != 0 && msg.Status != ERR_CODE_EALREADY {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking.
func discSvcUuid(x *BleXport, bl *Listener, r *BleDiscSvcUuidReq) (
	*BleSvc, error) {

	const rspType = MSG_TYPE_DISC_SVC_UUID
	const evtType = MSG_TYPE_DISC_SVC_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	var svc *BleSvc
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleDiscSvcUuidRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleDiscSvcEvt:
				switch msg.Status {
				case 0:
					svc = &msg.Svc
				case ERR_CODE_EDONE:
					if svc == nil {
						return nil, nmxutil.FmtBleHostError(
							msg.Status,
							"Peer doesn't support required service: %s",
							r.Uuid.String())
					}
					return svc, nil
				default:
					return nil, StatusError(MSG_OP_EVT, evtType, msg.Status)
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return nil, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking.
func discAllChrs(x *BleXport, bl *Listener, r *BleDiscAllChrsReq) (
	[]*BleChr, error) {

	const rspType = MSG_TYPE_DISC_ALL_CHRS
	const evtType = MSG_TYPE_DISC_CHR_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	chrs := []*BleChr{}
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleDiscAllChrsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleDiscChrEvt:
				switch msg.Status {
				case 0:
					chrs = append(chrs, &msg.Chr)
				case ERR_CODE_EDONE:
					return chrs, nil
				default:
					return nil, StatusError(MSG_OP_EVT, evtType, msg.Status)
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return nil, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking.
func writeCmd(x *BleXport, bl *Listener, r *BleWriteCmdReq) error {
	const rspType = MSG_TYPE_WRITE_CMD

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleWriteCmdRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				} else {
					return nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking.
func exchangeMtu(x *BleXport, bl *Listener, r *BleExchangeMtuReq) (
	int, error) {

	const rspType = MSG_TYPE_EXCHANGE_MTU
	const evtType = MSG_TYPE_MTU_CHANGE_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return 0, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleExchangeMtuRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return 0, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleMtuChangeEvt:
				if msg.Status != 0 {
					return 0, StatusError(MSG_OP_EVT, evtType, msg.Status)
				} else {
					return int(msg.Mtu), nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return 0, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

func actScan(x *BleXport, bl *Listener, r *BleScanReq, abortChan chan struct{},
	advRptCb BleAdvRptFn) error {

	const rspType = MSG_TYPE_SCAN

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleScanRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleScanEvt:
				r := BleAdvReportFromScanEvt(msg)
				advRptCb(r)

			case *BleScanTmoEvt:
				return nmxutil.NewScanTmoError("scan duration expired")

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)

		case <-abortChan:
			return nil
		}
	}
}

func scanCancel(x *BleXport, bl *Listener, r *BleScanCancelReq) error {
	const rspType = MSG_TYPE_SCAN_CANCEL

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleScanCancelRsp:
				bl.Acked = true
				if msg.Status != 0 && msg.Status != ERR_CODE_EALREADY {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

func connFind(x *BleXport, bl *Listener, r *BleConnFindReq) (
	BleConnDesc, error) {

	const rspType = MSG_TYPE_CONN_FIND

	j, err := json.Marshal(r)
	if err != nil {
		return BleConnDesc{}, err
	}

	if err := x.Tx(j); err != nil {
		return BleConnDesc{}, err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return BleConnDesc{}, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleConnFindRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return BleConnDesc{},
						StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

				return BleDescFromConnFindRsp(msg), nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BleConnDesc{}, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Tells the host to reset the controller.
func reset(x *BleXport, bl *Listener,
	r *BleResetReq) error {

	const rspType = MSG_TYPE_RESET

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	x.txNoSync(j)
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch bm.(type) {
			case *BleResetRsp:
				bl.Acked = true
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func encInitiate(x *BleXport, bl *Listener,
	r *BleSecurityInitiateReq) error {

	const rspType = MSG_TYPE_SECURITY_INITIATE

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSecurityInitiateRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func advStart(x *BleXport, bl *Listener, r *BleAdvStartReq) error {
	const rspType = MSG_TYPE_ADV_START

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvStartRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func advStop(x *BleXport, bl *Listener, r *BleAdvStopReq) error {
	const rspType = MSG_TYPE_ADV_STOP

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvStopRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func advSetData(x *BleXport, bl *Listener, r *BleAdvSetDataReq) error {
	const rspType = MSG_TYPE_ADV_SET_DATA

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvSetDataRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func advRspSetData(x *BleXport, bl *Listener, r *BleAdvRspSetDataReq) error {
	const rspType = MSG_TYPE_ADV_RSP_SET_DATA

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvRspSetDataRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Blocking
func advFields(x *BleXport, bl *Listener, r *BleAdvFieldsReq) (
	[]byte, error) {

	const rspType = MSG_TYPE_ADV_FIELDS

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvFieldsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

				return msg.Data, nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return nil, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Asks the controller to generate a random address.  This is done when the
// transport is starting up, and therefore does not require the transport to be
// synced.  Only the transport should call this function.
func genRandAddr(x *BleXport, bl *Listener, r *BleGenRandAddrReq) (
	BleAddr, error) {

	const rspType = MSG_TYPE_GEN_RAND_ADDR

	j, err := json.Marshal(r)
	if err != nil {
		return BleAddr{}, err
	}

	x.txNoSync(j)
	for {
		select {
		case err := <-bl.ErrChan:
			return BleAddr{}, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleGenRandAddrRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return BleAddr{},
						StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return msg.Addr, nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BleAddr{}, BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Configures the controller with the specified random address.  This is done
// when the transport is starting up, and therefore does not require the
// transport to be synced.  Only the transport should call this function.
func setRandAddr(x *BleXport, bl *Listener, r *BleSetRandAddrReq) error {
	const rspType = MSG_TYPE_SET_RAND_ADDR

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	x.txNoSync(j)
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSetRandAddrRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}

// Configures the host with the specified preferred ATT MTU.  This is done
// when the transport is starting up, and therefore does not require the
// transport to be synced.  Only the transport should call this function.
func setPreferredMtu(x *BleXport, bl *Listener,
	r *BleSetPreferredMtuReq) error {

	const rspType = MSG_TYPE_SET_PREFERRED_MTU

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	x.txNoSync(j)
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSetPreferredMtuRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case <-bl.AfterTimeout(x.RspTimeout()):
			return BhdTimeoutError(rspType, r.Seq)
		}
	}
}
