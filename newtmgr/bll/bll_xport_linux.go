// +build linux

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
	"github.com/rigado/ble"
	"github.com/rigado/ble/linux"
	"github.com/rigado/ble/linux/hci/cmd"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func BllXportSetConnParams(dev ble.Device, ownAddrType bledefs.BleAddrType) error {
	ldev := dev.(*linux.Device)

	cc := cmd.LECreateConnection{
		LEScanInterval:        0x0010, // 0x0004 - 0x4000; N * 0.625 msec
		LEScanWindow:          0x0010, // 0x0004 - 0x4000; N * 0.625 msec
		InitiatorFilterPolicy: 0x00,   // White list is not used
		OwnAddressType:        uint8(ownAddrType),
		ConnIntervalMin:       0x0006, // 0x0006 - 0x0C80; N * 1.25 msec
		ConnIntervalMax:       0x0006, // 0x0006 - 0x0C80; N * 1.25 msec
		ConnLatency:           0x0000, // 0x0000 - 0x01F3; N * 1.25 msec
		SupervisionTimeout:    0x0048, // 0x000A - 0x0C80; N * 10 msec
		MinimumCELength:       0x0000, // 0x0000 - 0xFFFF; N * 0.625 msec
		MaximumCELength:       0x0000, // 0x0000 - 0xFFFF; N * 0.625 msec

		// Specified at connect time.
		PeerAddressType: 0x00,      // Public Device Address
		PeerAddress:     [6]byte{}, //
	}

	opt := ble.OptConnParams(cc)
	if err := ldev.HCI.Option(opt); err != nil {
		return util.FmtNewtError("error setting connection parameters: %s",
			err.Error())
	}

	return nil
}
