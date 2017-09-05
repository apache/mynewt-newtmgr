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
	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

type BleOicSvr struct {
	s          oic.Server
	x          *BleXport
	rspSvcUuid BleUuid
	rspChrUuid BleUuid
}

func NewBleOicSvr(x *BleXport,
	rspSvcUuid BleUuid, rspChrUuid BleUuid) BleOicSvr {

	return BleOicSvr{
		s:          oic.NewServer(true),
		x:          x,
		rspSvcUuid: rspSvcUuid,
		rspChrUuid: rspChrUuid,
	}
}

func (b *BleOicSvr) AddResource(r oic.Resource) error {
	return b.s.AddResource(r)
}

func (b *BleOicSvr) processCoap(access BleGattAccess) {
	s := b.x.findSesn(access.ConnHandle)
	if s == nil {
		// The sender is no longer connected.
		log.Debugf("Failed to send CoAP response; peer no longer "+
			"connected (conn_handle=%d)", access.ConnHandle)
		return
	}

	ml, err := b.s.Rx(access.Data)
	if err != nil {
		log.Debugf("Error processing incoming CoAP message: %s", err.Error())
		return
	}

	if ml == nil {
		// Partial CoAP message; remainder forthcoming.
		return
	}

	data, err := ml.MarshalBinary()
	if err != nil {
		log.Debugf("Failed to send CoAP response; error marshalling "+
			"response: %s", err.Error())
		return
	}

	_, valHandle, err := FindChrXact(b.x, b.rspSvcUuid, b.rspChrUuid)
	if err != nil {
		log.Debugf("Failed to send CoAP response; cannot find response "+
			"characteristic: (s=%s c=%s)",
			b.rspSvcUuid.String(), b.rspChrUuid.String())
		return
	}

	mtu := s.MtuOut()
	frags := nmxutil.Fragment(data, mtu)
	for i, frag := range frags {
		if err := NotifyXact(b.x, access.ConnHandle, valHandle,
			frag); err != nil {

			log.Debugf("Failed to send CoAP response; failed to send "+
				"fragment %d/%d: %s", err.Error(), i, len(frags))
			return
		}
	}
}

func (b *BleOicSvr) Rx(access BleGattAccess) uint8 {
	// Process the CoAP request and send a response via notification in the
	// background.
	go b.processCoap(access)

	// Indicate success to BLE server.
	return 0
}
