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

	"mynewt.apache.org/newtmgr/newtmgr/config"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xport"
	"mynewt.apache.org/newt/util"
)

var globalSesn sesn.Sesn
var globalXport xport.Xport

// These keep track of whether the global interfaces have been assigned.  These
// are necessary to accommodate golang's nil-interface semantics.
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

		globalXport = nmserial.NewSerialXport(sc)
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

	if err := globalXport.Start(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return globalXport, nil
}

func GetXportIfOpen() (xport.Xport, error) {
	if !globalXportSet {
		return nil, fmt.Errorf("xport not initailized")
	}

	return globalXport, nil
}

func buildSesnCfg() (sesn.SesnCfg, error) {
	sc := sesn.NewSesnCfg()

	cp, err := getConnProfile()
	if err != nil {
		return sc, err
	}

	switch cp.Type {
	case config.CONN_TYPE_SERIAL:
		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		return sc, nil

	case config.CONN_TYPE_BLE_PLAIN:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return sc, err
		}

		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		config.FillSesnCfg(bc, &sc)
		return sc, nil

	case config.CONN_TYPE_BLE_OIC:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return sc, err
		}

		sc.MgmtProto = sesn.MGMT_PROTO_OMP
		config.FillSesnCfg(bc, &sc)
		return sc, nil

	default:
		return sc, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}

}

func GetSesn() (sesn.Sesn, error) {
	if globalSesn != nil {
		return globalSesn, nil
	}

	sc, err := buildSesnCfg()
	if err != nil {
		return nil, err
	}

	x, err := GetXport()
	if err != nil {
		return nil, err
	}

	s, err := x.BuildSesn(sc)
	if err != nil {
		return nil, util.ChildNewtError(err)
	}

	globalSesn = s
	if err := globalSesn.Open(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return globalSesn, nil
}

func GetSesnIfOpen() (sesn.Sesn, error) {
	if globalSesn == nil {
		return nil, fmt.Errorf("sesn not initailized")
	}

	return globalSesn, nil
}
