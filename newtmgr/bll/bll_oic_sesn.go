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
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/currantlabs/ble"
	"github.com/runtimeco/go-coap"
	"golang.org/x/net/context"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllOicSesn struct {
	cfg BllSesnCfg

	cln    ble.Client
	d      *omp.Dispatcher
	mtx    sync.Mutex
	attMtu int

	nmpReqChr     *ble.Characteristic
	nmpRspChr     *ble.Characteristic
	publicReqChr  *ble.Characteristic
	publicRspChr  *ble.Characteristic
	gwReqChr      *ble.Characteristic
	gwRspChr      *ble.Characteristic
	privateReqChr *ble.Characteristic
	privateRspChr *ble.Characteristic
}

func NewBllOicSesn(cfg BllSesnCfg) *BllOicSesn {
	return &BllOicSesn{
		cfg: cfg,
	}
}

func (bls *BllOicSesn) listenDisconnect() {
	go func() {
		<-bls.cln.Disconnected()

		bls.mtx.Lock()
		bls.d.ErrorAll(fmt.Errorf("Disconnected"))
		bls.d.Stop()
		bls.mtx.Unlock()

		bls.cln = nil
	}()
}

func (bls *BllOicSesn) connect() error {
	log.Debugf("Connecting to peer")
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(),
		bls.cfg.ConnTimeout))

	var err error
	bls.cln, err = ble.Connect(ctx, bls.cfg.AdvFilter)
	if err != nil {
		if nmutil.ErrorCausedBy(err, context.DeadlineExceeded) {
			return fmt.Errorf("Failed to connect to peer after %s",
				bls.cfg.ConnTimeout.String())
		} else {
			return err
		}
	}

	bls.listenDisconnect()

	return nil
}

func findChr(profile *ble.Profile, svcUuid bledefs.BleUuid,
	chrUuid bledefs.BleUuid) (*ble.Characteristic, error) {

	for _, s := range profile.Services {
		uuid, err := UuidFromBllUuid(s.UUID)
		if err != nil {
			return nil, err
		}

		if bledefs.CompareUuids(uuid, svcUuid) == 0 {
			for _, c := range s.Characteristics {
				uuid, err := UuidFromBllUuid(c.UUID)
				if err != nil {
					return nil, err
				}

				if bledefs.CompareUuids(uuid, chrUuid) == 0 {
					return c, nil
				}
			}
		}
	}

	return nil, nil
}

func (bls *BllOicSesn) discoverAll() error {
	log.Debugf("Discovering profile")
	p, err := bls.cln.DiscoverProfile(true)
	if err != nil {
		return err
	}

	ompSvcUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecSvcUuid)
	ompReqChrUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecReqChrUuid)
	ompRspChrUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecRspChrUuid)

	gwSvcUuid, _ := bledefs.ParseUuid(bledefs.GwSvcUuid)
	gwReqChrUuid, _ := bledefs.ParseUuid(bledefs.GwReqChrUuid)
	gwRspChrUuid, _ := bledefs.ParseUuid(bledefs.GwRspChrUuid)

	bls.nmpReqChr, err = findChr(p, ompSvcUuid, ompReqChrUuid)
	if err != nil {
		return err
	}

	bls.nmpRspChr, err = findChr(p, ompSvcUuid, ompRspChrUuid)
	if err != nil {
		return err
	}

	bls.gwReqChr, err = findChr(p, gwSvcUuid, gwReqChrUuid)
	if err != nil {
		return err
	}

	bls.gwRspChr, err = findChr(p, gwSvcUuid, gwRspChrUuid)
	if err != nil {
		return err
	}

	return nil
}

// Subscribes to the peer's characteristic implementing NMP.
func (bls *BllOicSesn) subscribe() error {
	log.Debugf("Subscribing to NMP response characteristic")
	onNotify := func(data []byte) {
		bls.d.Dispatch(data)
	}

	if bls.nmpRspChr != nil {
		if err := bls.cln.Subscribe(bls.nmpRspChr, false,
			onNotify); err != nil {

			return err
		}
	}

	if bls.gwRspChr != nil {
		if err := bls.cln.Subscribe(bls.gwRspChr, false,
			onNotify); err != nil {

			return err
		}
	}

	return nil
}

func (bls *BllOicSesn) exchangeMtu() error {
	mtu, err := exchangeMtu(bls.cln, bls.cfg.PreferredMtu)
	if err != nil {
		return err
	}

	bls.attMtu = mtu
	return nil
}

func (bls *BllOicSesn) Open() error {
	if bls.IsOpen() {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open bll session")
	}

	d, err := omp.NewDispatcher(true, 3)
	if err != nil {
		return err
	}
	bls.d = d

	if err := bls.connect(); err != nil {
		return err
	}

	if err := bls.exchangeMtu(); err != nil {
		return err
	}

	if err := bls.discoverAll(); err != nil {
		return err
	}

	if err := bls.subscribe(); err != nil {
		return err
	}

	return nil
}

func (bls *BllOicSesn) Close() error {
	if !bls.IsOpen() {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened bll session")
	}

	if err := bls.cln.CancelConnection(); err != nil {
		return err
	}

	bls.cln = nil

	return nil
}

// Indicates whether the session is currently open.
func (bls *BllOicSesn) IsOpen() bool {
	return bls.cln != nil
}

// Retrieves the maximum data payload for outgoing NMP requests.
func (bls *BllOicSesn) MtuOut() int {
	return bls.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Retrieves the maximum data payload for incoming NMP responses.
func (bls *BllOicSesn) MtuIn() int {
	return bls.attMtu - nmble.NOTIFY_CMD_BASE_SZ - nmp.NMP_HDR_SIZE
}

// Stops a receive operation in progress.  This must be called from a
// separate thread, as sesn receive operations are blocking.
func (bls *BllOicSesn) AbortRx(nmpSeq uint8) error {
	return bls.d.ErrorOneNmp(nmpSeq, fmt.Errorf("Rx aborted"))
}

func (bls *BllOicSesn) EncodeNmpMsg(msg *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpTcp(msg)
}

// Performs a blocking transmit a single NMP message and listens for the
// response.
//     * nil: success.
//     * nmxutil.SesnClosedError: session not open.
//     * other error
func (bls *BllOicSesn) TxNmpOnce(msg *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !bls.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed BLE session")
	}

	if bls.nmpReqChr == nil || bls.nmpRspChr == nil {
		return nil, fmt.Errorf("Cannot send NMP request; peer doesn't " +
			"support request or response characteristic")
	}

	b, err := bls.EncodeNmpMsg(msg)
	if err != nil {
		return nil, err
	}

	nl, err := bls.d.AddNmpListener(msg.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer bls.d.RemoveNmpListener(msg.Hdr.Seq)

	// Send request.
	if err := bls.cln.WriteCharacteristic(bls.nmpReqChr, b, true); err != nil {
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
				msg.Hdr.Op, msg.Hdr.Group, msg.Hdr.Id, msg.Hdr.Seq)

			return nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}

func (bls *BllOicSesn) resReqChr(resType sesn.ResourceType) (
	*ble.Characteristic, error) {

	m := map[sesn.ResourceType]*ble.Characteristic{
		sesn.RES_TYPE_PUBLIC: bls.publicReqChr,
		sesn.RES_TYPE_UNAUTH: bls.gwReqChr,
		sesn.RES_TYPE_SECURE: bls.privateReqChr,
	}

	chr := m[resType]
	if chr == nil {
		return nil, fmt.Errorf("BLE session not configured with "+
			"characteristic for %s resources", resType)
	}

	return chr, nil
}

func (bls *BllOicSesn) GetResourceOnce(resType sesn.ResourceType, uri string,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	chr, err := bls.resReqChr(resType)
	if err != nil {
		return 0, nil, err
	}

	token := nmxutil.NextToken()

	ol, err := bls.d.AddOicListener(token)
	if err != nil {
		return 0, nil, err
	}
	defer bls.d.RemoveOicListener(token)

	req, err := oic.EncodeGet(true, uri, token)
	if err != nil {
		return 0, nil, err
	}

	// Send request.
	if err := bls.cln.WriteCharacteristic(chr, req, true); err != nil {
		return 0, nil, err
	}

	// Now wait for CoAP response.
	for {
		select {
		case err := <-ol.ErrChan:
			return 0, nil, err
		case rsp := <-ol.RspChan:
			return rsp.Code(), rsp.Payload(), nil
		case <-ol.AfterTimeout(opt.Timeout):
			msg := fmt.Sprintf("CoAP timeout; uri=%s", uri)
			return 0, nil, nmxutil.NewRspTimeoutError(msg)
		}
	}
}
