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
	"fmt"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/JuulLabs-OSS/ble"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func exchangeMtu(cln ble.Client, preferredMtu uint16) (uint16, error) {
	log.Debugf("Exchanging MTU")

	// We loop three times here to workaround an library issue with macOS.  In
	// macOS, the request to exchange MTU doesn't actually do anything.  The
	// BLE library relies on the assumption that the OS already exchanged MTUs
	// on its own.  If this assumption is incorrect, the number that was
	// returned is the default out of date value (23 or for some strange reason,
	// on some versions of mac 17). In this case, sleep and retry.
	var mtu int
	for i := 0; i < 3; i++ {
		var err error
		mtu, err = cln.ExchangeMTU(int(preferredMtu))
		if err != nil {
			return 0, err
		}

		// If this isn't macOS, accept the MTU the library reported.
		if runtime.GOOS != "darwin" {
			break
		}

		// If macOS returned a value higher than 23, then MTU exchange has
		// completed.
		if mtu > bledefs.BLE_ATT_MTU_DFLT {
			break
		}

		// Otherwise, give the OS some time to perform the exchange.
		log.Debugf("macOS reports an MTU of <=23.  " +
			"Assume exchange hasn't completed; wait and requery.")
		time.Sleep(time.Second)
	}

	// On some versions of mac (e.g. monterey) workaround with
	// looping three times seems to not work anymore. This allows to use
	// "-m <mtu value>" flag to set MTU from command line.
	if nmutil.MtuOverride != 0 {
		if nmutil.MtuOverride < 23 {
			return 0, fmt.Errorf("MTU should be at least 23")
		} else {
			mtu = nmutil.MtuOverride
		}
	}

	// If no MTU value was specified in the command line
	// and performing three loops didn't help in performing exchange, MTU is still invalid (< 23).
	// In this case we hardcode it to 185.
	if mtu < bledefs.BLE_ATT_MTU_DFLT {
		mtu = 185
	}

	log.Debugf("Exchanged MTU; ATT MTU = %d", mtu)
	return uint16(mtu), nil
}
