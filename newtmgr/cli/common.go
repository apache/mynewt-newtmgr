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

package cli

import (
	"fmt"

	"mynewt.apache.org/newt/newtmgr/config"
	"mynewt.apache.org/newt/newtmgr/nmutil"
	"mynewt.apache.org/newt/nmxact/nmble"
	"mynewt.apache.org/newt/nmxact/nmserial"
	"mynewt.apache.org/newt/nmxact/sesn"
	"mynewt.apache.org/newt/nmxact/xport"
	"mynewt.apache.org/newt/util"
)

var globalSesn sesn.Sesn
var globalXport xport.Xport

// These keep track of whether the global interfaces have been assigned.  These
// are necessary to accommodate golang's nil-interface semantics.
var globalSesnSet bool
var globalXportSet bool

func getConnProfile() (*config.ConnProfile, error) {
	return config.GlobalConnProfileMgr().GetConnProfile(nmutil.ConnProfile)
}

func GetXport() (xport.Xport, error) {
	if globalXport != nil {
		return globalXport, nil
	}

	cp, err := getConnProfile()
	if err != nil {
		return nil, err
	}

	switch cp.Type {
	case config.CONN_TYPE_SERIAL:
		sc, err := config.ParseSerialConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}
		globalXport, err = config.BuildSerialXport(sc)
		if err != nil {
			return nil, err
		}
	case config.CONN_TYPE_BLE_PLAIN, config.CONN_TYPE_BLE_OIC:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}
		globalXport, err = config.BuildBleXport(bc)
		if err != nil {
			return nil, err
		}
	default:
		return nil, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}

	globalXportSet = true
	return globalXport, nil
}

func GetXportIfOpen() (xport.Xport, error) {
	if !globalXportSet {
		return nil, fmt.Errorf("xport not initailized")
	}

	return globalXport, nil
}

func GetSesn() (sesn.Sesn, error) {
	if globalSesn != nil {
		return globalSesn, nil
	}

	cp, err := getConnProfile()
	if err != nil {
		return nil, err
	}

	x, err := GetXport()
	if err != nil {
		return nil, err
	}

	switch cp.Type {
	case config.CONN_TYPE_SERIAL:
		globalSesn, err = config.BuildSerialPlainSesn(x.(*nmserial.SerialXport))
		if err != nil {
			return nil, err
		}

	case config.CONN_TYPE_BLE_PLAIN:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}

		globalSesn, err = config.BuildBlePlainSesn(x.(*nmble.BleXport), bc)
		if err != nil {
			return nil, err
		}

	case config.CONN_TYPE_BLE_OIC:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}

		globalSesn, err = config.BuildBleOicSesn(x.(*nmble.BleXport), bc)
		if err != nil {
			return nil, err
		}

	default:
		return nil, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}

	globalSesnSet = true
	return globalSesn, nil
}

func GetSesnIfOpen() (sesn.Sesn, error) {
	if !globalSesnSet {
		return nil, fmt.Errorf("sesn not initailized")
	}

	return globalSesn, nil
}
