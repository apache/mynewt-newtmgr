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
func connect(x *BleXport, bl *Listener, r *BleConnectReq) (uint16, error) {
	const rspType = MSG_TYPE_CONNECT

	j, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, err
	}

	// Give blehostd three seconds of leeway to tell us the connection attempt
	// timed out.
	rspTimeout := time.Duration(r.DurationMs+3000) * time.Millisecond
	rspTmoChan := time.After(rspTimeout)
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

			case *BleConnCancelEvt:
				str := "BLE connection attempt cancelled"
				log.Debugf(str)
				return 0, fmt.Errorf("%s", str)

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil

		case _, ok := <-rspTmoChan:
			if ok {
				return 0, fmt.Errorf("Failed to connect to peer after %s",
					rspTimeout.String())
			}
			rspTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	// Allow 10 seconds for the controller to cancel the connection.
	rspTimeout := 10 * time.Second
	rspTmoChan := time.After(rspTimeout)
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleConnCancelRsp:
				bl.Acked = true
				switch msg.Status {
				case 0:
					// Cancel initiated.  Await connect failure.
					return nil

				case ERR_CODE_EALREADY:
					// No connect in progress.  Pretend success.
					return nil

				default:
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil

		case _, ok := <-rspTmoChan:
			if ok {
				x.Restart("Failed to cancel connect after " + rspTimeout.String())
			}
			rspTmoChan = nil
		}
	}
}

// Blocking.
func discAllSvcs(x *BleXport, bl *Listener, r *BleDiscAllSvcsReq) (
	[]*BleDiscSvc, error) {

	const rspType = MSG_TYPE_DISC_ALL_SVCS
	const evtType = MSG_TYPE_DISC_SVC_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	var svcs []*BleDiscSvc
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleDiscAllSvcsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleDiscSvcEvt:
				switch msg.Status {
				case 0:
					svcs = append(svcs, &msg.Svc)
				case ERR_CODE_EDONE:
					return svcs, nil
				default:
					return nil, StatusError(MSG_OP_EVT, evtType, msg.Status)
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func discSvcUuid(x *BleXport, bl *Listener, r *BleDiscSvcUuidReq) (
	*BleDiscSvc, error) {

	const rspType = MSG_TYPE_DISC_SVC_UUID
	const evtType = MSG_TYPE_DISC_SVC_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	var svc *BleDiscSvc
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func discAllChrs(x *BleXport, bl *Listener, r *BleDiscAllChrsReq) (
	[]*BleDiscChr, error) {

	const rspType = MSG_TYPE_DISC_ALL_CHRS
	const evtType = MSG_TYPE_DISC_CHR_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	chrs := []*BleDiscChr{}
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func discAllDscs(x *BleXport, bl *Listener, r *BleDiscAllDscsReq) (
	[]*BleDiscDsc, error) {

	const rspType = MSG_TYPE_DISC_ALL_DSCS
	const evtType = MSG_TYPE_DISC_DSC_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	dscs := []*BleDiscDsc{}
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleDiscAllDscsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleDiscDscEvt:
				switch msg.Status {
				case 0:
					dscs = append(dscs, &msg.Dsc)
				case ERR_CODE_EDONE:
					return dscs, nil
				default:
					return nil, StatusError(MSG_OP_EVT, evtType, msg.Status)
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func write(x *BleXport, bl *Listener, r *BleWriteReq) error {
	const rspType = MSG_TYPE_WRITE_CMD
	const evtType = MSG_TYPE_WRITE_ACK_EVT

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleWriteRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleWriteAckEvt:
				switch msg.Status {
				case 0:
					return nil
				default:
					return StatusError(MSG_OP_EVT, evtType, msg.Status)
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func notify(x *BleXport, bl *Listener, r *BleNotifyReq) error {
	const rspType = MSG_TYPE_NOTIFY

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleNotifyRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				} else {
					return nil
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking.
func findChr(x *BleXport, bl *Listener, r *BleFindChrReq) (
	uint16, uint16, error) {

	const rspType = MSG_TYPE_NOTIFY

	j, err := json.Marshal(r)
	if err != nil {
		return 0, 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, 0, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return 0, 0, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleFindChrRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return 0, 0, StatusError(MSG_OP_RSP, rspType, msg.Status)
				} else {
					return msg.DefHandle, msg.ValHandle, nil
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func actScan(x *BleXport, bl *Listener, r *BleScanReq) (
	<-chan BleAdvReport, <-chan error, error) {

	const rspType = MSG_TYPE_SCAN

	j, err := json.Marshal(r)
	if err != nil {
		return nil, nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, nil, err
	}

	ach := make(chan BleAdvReport)
	ech := make(chan error)

	nmxutil.Assert(bl != nil)

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	go func() {
		defer close(ach)
		defer close(ech)

		for {
			select {
			case err := <-bl.ErrChan:
				ech <- err
				return

			case bm, ok := <-bl.MsgChan:
				if ok {
					switch msg := bm.(type) {
					case *BleScanRsp:
						bl.Acked = true
						if msg.Status != 0 {
							ech <- StatusError(MSG_OP_RSP, rspType, msg.Status)
							return
						}

					case *BleScanEvt:
						r := BleAdvReportFromScanEvt(msg)
						ach <- r

					case *BleScanCompleteEvt:
						if msg.Reason == 0 {
							// On successful completion, just return and allow
							// the ech channel to close.
							return
						} else {
							ech <- StatusError(MSG_OP_RSP, rspType, msg.Reason)
							return
						}

					default:
					}
				}

			case _, ok := <-bhdTmoChan:
				if ok {
					x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
				}
				bhdTmoChan = nil
			}
		}
	}()

	return ach, ech, nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Tells the host to reset the controller.
func reset(x *BleXport, bl *Listener, r *BleResetReq) error {
	const rspType = MSG_TYPE_RESET

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.txNoSync(j); err != nil {
		return err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				return fmt.Errorf("Blehostd timeout: %s",
					MsgTypeToString(rspType))
			}
		}
	}
}

// Blocking
func securityInitiate(x *BleXport, bl *Listener,
	r *BleSecurityInitiateReq) error {

	const rspType = MSG_TYPE_SECURITY_INITIATE

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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
				} else {
					return nil
				}

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func smInjectIo(x *BleXport, bl *Listener, r *BleSmInjectIoReq) error {
	const rspType = MSG_TYPE_SM_INJECT_IO

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSmInjectIoRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

// Blocking
func advStart(x *BleXport, bl *Listener, stopChan chan struct{},
	r *BleAdvStartReq) (uint16, error) {

	const rspType = MSG_TYPE_ADV_START

	j, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, err
	}

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return 0, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAdvStartRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return 0, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}

			case *BleConnectEvt:
				if msg.Status == 0 {
					return msg.ConnHandle, nil
				} else {
					str := fmt.Sprintf("BLE peer failed to connect to us; "+
						"status=%s (%d)",
						ErrCodeToString(msg.Status), msg.Status)
					log.Debugf(str)
					return 0, nmxutil.NewBleHostError(msg.Status, str)
				}

			case *BleAdvCompleteEvt:
				str := fmt.Sprintf("Advertising stopped; reason=%s (%d)",
					ErrCodeToString(msg.Reason), msg.Reason)
				log.Debugf(str)
				return 0, nmxutil.NewBleHostError(msg.Reason, str)

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil

		case <-stopChan:
			return 0, fmt.Errorf("advertise aborted")
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

				return msg.Data.Bytes, nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func clearSvcs(x *BleXport, bl *Listener, r *BleClearSvcsReq) error {
	const rspType = MSG_TYPE_CLEAR_SVCS

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.txNoSync(j); err != nil {
		return err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleClearSvcsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func addSvcs(x *BleXport, bl *Listener, r *BleAddSvcsReq) error {
	const rspType = MSG_TYPE_ADD_SVCS

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.txNoSync(j); err != nil {
		return err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAddSvcsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func commitSvcs(x *BleXport, bl *Listener, r *BleCommitSvcsReq) (
	[]BleRegSvc, error) {

	const rspType = MSG_TYPE_COMMIT_SVCS

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.txNoSync(j); err != nil {
		return nil, err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleCommitSvcsRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return nil, StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return msg.Svcs, nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func accessStatus(x *BleXport, bl *Listener, r *BleAccessStatusReq) error {
	const rspType = MSG_TYPE_ACCESS_STATUS

	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	x.Tx(j)
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return err

		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleAccessStatusRsp:
				bl.Acked = true
				if msg.Status != 0 {
					return StatusError(MSG_OP_RSP, rspType, msg.Status)
				}
				return nil

			default:
			}

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	if err := x.txNoSync(j); err != nil {
		return BleAddr{}, err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	if err := x.txNoSync(j); err != nil {
		return err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
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

	if err := x.txNoSync(j); err != nil {
		return err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
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

		case _, ok := <-bhdTmoChan:
			if ok {
				x.Restart("Blehostd timeout: " + MsgTypeToString(rspType))
			}
			bhdTmoChan = nil
		}
	}
}

func checkSync(x *BleXport, bl *Listener, r *BleSyncReq) (bool, error) {
	const rspType = MSG_TYPE_SYNC

	j, err := json.Marshal(r)
	if err != nil {
		return false, err
	}

	if err := x.txNoSync(j); err != nil {
		return false, err
	}
	bhdTmoChan := bl.AfterTimeout(x.RspTimeout())
	for {
		select {
		case err := <-bl.ErrChan:
			return false, err
		case bm := <-bl.MsgChan:
			switch msg := bm.(type) {
			case *BleSyncRsp:
				bl.Acked = true
				return msg.Synced, nil
			}
		case _, ok := <-bhdTmoChan:
			if ok {
				return false, fmt.Errorf("Blehostd timeout: %s",
					MsgTypeToString(rspType))
			}
		}
	}
}
