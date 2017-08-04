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

package bll

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"github.com/runtimeco/go-coap"
	"golang.org/x/net/context"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllPlainSesn struct {
	cfg BllSesnCfg

	cln    ble.Client
	nmpChr *ble.Characteristic
	d      *nmp.Dispatcher
	attMtu int
}

func NewBllPlainSesn(cfg BllSesnCfg) *BllPlainSesn {
	return &BllPlainSesn{
		cfg: cfg,
		d:   nmp.NewDispatcher(1),
	}
}

func (bps *BllPlainSesn) listenDisconnect() {
	go func() {
		<-bps.cln.Disconnected()

		bps.d.ErrorAll(fmt.Errorf("Disconnected"))
		bps.cln = nil
	}()
}

func (bps *BllPlainSesn) connect() error {
	log.Debugf("Connecting to peer")
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		bps.cfg.ConnTimeout))

	var err error
	bps.cln, err = ble.Connect(ctx, bps.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return fmt.Errorf("Failed to connect to peer after %s",
				bps.cfg.ConnTimeout.String())
		} else {
			return err
		}
	}

	bps.listenDisconnect()

	return nil
}

func (bps *BllPlainSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := bps.cln.DiscoverProfile(true)
	if err != nil {
		return err
	}

	svcUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainSvcUuid)
	chrUuid, _ := bledefs.ParseUuid(bledefs.NmpPlainChrUuid)

	for _, s := range p.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return err
		}

		if bledefs.CompareUuids(uuid, svcUuid) == 0 {
			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return err
				}

				if bledefs.CompareUuids(uuid, chrUuid) == 0 {
					bps.nmpChr = c
					return nil
				}
			}
		}
	}

	return fmt.Errorf(
		"Peer doesn't support a suitable service / characteristic")
}

// Subscribes to the peer's characteristic implementing NMP.
func (bps *BllPlainSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		bps.d.Dispatch(data)
	}

	if err := bps.cln.Subscribe(bps.nmpChr, false, onNotify); err != nil {
		return err
	}

	return nil
}

func (bps *BllPlainSesn) exchangeMtu() error {
	mtu, err := exchangeMtu(bps.cln, bps.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	bps.attMtu = mtu
	return nil
}

func (bps *BllPlainSesn) Open() error {
	if bps.IsOpen() {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

	if err := bps.connect(); err != nil {
		return err
	}

	if err := bps.exchangeMtu(); err != nil {
		return err
	}

	if err := bps.discoverAll(); err != nil {
		return err
	}

	if err := bps.subscribe(); err != nil {
		return err
	}

	return nil
}

func (bps *BllPlainSesn) Close() error {
	if !bps.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := bps.cln.CancelConnection(); err != nil {
		return err
	}

	bps.cln = nil

	return nil
}

// Indicates whether the session is currently open.
func (bps *BllPlainSesn) IsOpen() bool {
	return bps.cln != nil
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (bps *BllPlainSesn) MtuOut() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Retrieves the maximum data payload for incoming NMP responses.
func (bps *BllPlainSesn) MtuIn() int {
	return bps.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (bps *BllPlainSesn) AbortRx(nmpSeq uint8) error {
	return bps.d.ErrorOne(nmpSeq, fmt.Errorf("Rx aborted"))
}

func (bps *BllPlainSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return nmp.EncodeNmpPlain(msg)
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (bps *BllPlainSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bps.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	b, err := bps.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	nl, err := bps.d.AddListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bps.d.RemoveListener(msg.Hdr.Seq)

	// Send request.
	if err := bps.cln.WriteCharacteristic(bps.nmpChr, b, true); err != nil {
		return nil, err
	}

	// Now wait for NMP response.
	for {
		select {
		case err := <-nl.ErrChan:
			return nil, err
		case rsp := <-nl.RspChan:
			return rsp, nil
		case <-nl.AfterTimeout(opt.Timeout):
			msg := fmt.Sprintf(
				"NMP timeout; op=%d group=%d id=%d seq=%d",
				b[0], b[4]+b[5]<<8, b[7], b[6])

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bps *BllPlainSesn) GetResourceOnce(resType sesn.ResourceType, uri string,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	return 0, nil,
		fmt.Errorf("Resource API not supported by plain (non-OIC) session")
}

func (bps *BllPlainSesn) PutResourceOnce(resType sesn.ResourceType,
	uri string, value []byte, opt sesn.TxOptions) (coap.COAPCode, error) {

	return 0, fmt.Errorf("BllPlainSesn.PutResourceOnce() unsupported")
}
