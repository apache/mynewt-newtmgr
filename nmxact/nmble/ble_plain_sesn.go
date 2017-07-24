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
	"fmt"
	"sync"
	"time"

	"mynewt.apache.org/newt/util"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BlePlainSesn struct {
	bf           *BleFsm
	d            *nmp.Dispatcher
	closeTimeout time.Duration
	onCloseCb    sesn.OnCloseFn
	wg           sync.WaitGroup
	closeBlocker nmxutil.Blocker
}

func NewBlePlainSesn(bx *BleXport, cfg sesn.SesnCfg) *BlePlainSesn {
	bps := &BlePlainSesn{
		closeTimeout: cfg.Ble.CloseTimeout,
		onCloseCb:    cfg.OnCloseCb,
	}

	svcUuid, _ := ParseUuid(NmpPlainSvcUuid)
	chrUuid, _ := ParseUuid(NmpPlainChrUuid)

	bps.bf = NewBleFsm(BleFsmParams{
		Bx:          bx,
		OwnAddrType: cfg.Ble.OwnAddrType,
		Central: BleFsmParamsCentral{
			PeerDev:     cfg.PeerSpec.Ble,
			ConnTries:   cfg.Ble.Central.ConnTries,
			ConnTimeout: cfg.Ble.Central.ConnTimeout,
		},
		SvcUuids:    []BleUuid{svcUuid},
		ReqChrUuid:  chrUuid,
		RspChrUuid:  chrUuid,
		EncryptWhen: cfg.Ble.EncryptWhen,
	})

	return bps
}

func (bps *BlePlainSesn) AbortRx(seq uint8) error {
	return bps.d.ErrorOne(seq, fmt.Errorf("Rx aborted"))
}

func (bps *BlePlainSesn) Open() error {
	// Ensure subsequent calls to Close() block.
	bps.closeBlocker.Block()

	if err := bps.bf.Start(); err != nil {
		if !nmxutil.IsSesnAlreadyOpen(err) {
			bps.closeBlocker.Unblock()
		}
		return err
	}

	bps.d = nmp.NewDispatcher(3)

	// Listen for disconnect in the background.
	bps.wg.Add(1)
	go func() {
		// If the session is being closed, unblock the close() call.
		defer bps.closeBlocker.Unblock()

		// Block until disconnect.
		<-bps.bf.DisconnectChan()
		nmxutil.Assert(!bps.IsOpen())

		pd := bps.bf.PrevDisconnect()

		// Signal error to all listeners.
		bps.d.ErrorAll(pd.Err)
		bps.wg.Done()
		bps.wg.Wait()

		// Only execute the client's disconnect callback if the disconnect was
		// unsolicited.
		if pd.Dt != FSM_DISCONNECT_TYPE_REQUESTED && bps.onCloseCb != nil {
			bps.onCloseCb(bps, pd.Err)
		}
	}()

	// Listen for NMP responses in the background.
	bps.wg.Add(1)
	go func() {
		defer bps.wg.Done()

		for {
			data, ok := <-bps.bf.RxNmpChan()
			if !ok {
				// Disconnected.
				return
			} else {
				bps.d.Dispatch(data)
			}
		}
	}()

	return nil
}

func (bps *BlePlainSesn) Close() error {
	err := bps.bf.Stop()
	if err != nil {
		return err
	}

	// Block until close completes.
	bps.closeBlocker.Wait()
	return nil
}

func (bps *BlePlainSesn) IsOpen() bool {
	return bps.bf.IsOpen()
}

func (bps *BlePlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(m)
}

// Blocking.
func (bps *BlePlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, bps.bf.closedError(
			"Attempt to transmit over closed BLE session")
	}

	nl, err := bps.d.AddListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.d.RemoveListener(msg.Hdr.Seq)

	b, err := bps.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	return bps.bf.TxNmp(b, nl, opt.Timeout)
}

func (bps *BlePlainSesn) MtuIn() int {
	return bps.bf.attMtu - NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

func (bps *BlePlainSesn) MtuOut() int {
	mtu := bps.bf.attMtu - WRITE_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
	return util.IntMin(mtu, BLE_ATT_ATTR_MAX_LEN)
}

func (bps *BlePlainSesn) ConnInfo() (BleConnDesc, error) {
	return bps.bf.connInfo()
}

func (bps *BlePlainSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("BlePlainSesn.GetResourceOnce() unsupported")
}
