// +build !windows

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
	"encoding/binary"
	"fmt"

	"github.com/rigado/ble"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func UuidFromBllUuid(bllUuid ble.UUID) (bledefs.BleUuid, error) {
	uuid := bledefs.BleUuid{}

	switch len(bllUuid) {
	case 2:
		uuid.U16 = bledefs.BleUuid16(binary.LittleEndian.Uint16(bllUuid))
		return uuid, nil

	case 16:
		for i, b := range bllUuid {
			uuid.U128[15-i] = b
		}
		return uuid, nil

	default:
		return uuid, fmt.Errorf("Invalid UUID: %#v", bllUuid)
	}
}
