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

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/bll"
	"mynewt.apache.org/newtmgr/newtmgr/config"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/udp"
	"mynewt.apache.org/newtmgr/nmxact/xport"
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
	case config.CONN_TYPE_SERIAL_PLAIN, config.CONN_TYPE_SERIAL_OIC:
		sc, err := config.ParseSerialConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}

		globalXport = nmserial.NewSerialXport(sc)

	case config.CONN_TYPE_BLL_PLAIN, config.CONN_TYPE_BLL_OIC:
		bc, err := config.ParseBllConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}

		cfg := bll.NewXportCfg()
		if bc.CtlrName != "" {
			cfg.CtlrName = bc.CtlrName
		}
		globalXport = bll.NewBllXport(cfg)

	case config.CONN_TYPE_BLE_PLAIN, config.CONN_TYPE_BLE_OIC:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return nil, err
		}
		globalXport, err = config.BuildBleXport(bc)
		if err != nil {
			return nil, err
		}

	case config.CONN_TYPE_UDP_PLAIN, config.CONN_TYPE_UDP_OIC:
		globalXport = udp.NewUdpXport()

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
	case config.CONN_TYPE_SERIAL_PLAIN:
		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		return sc, nil

	case config.CONN_TYPE_SERIAL_OIC:
		sc.MgmtProto = sesn.MGMT_PROTO_OMP
		return sc, nil

	case config.CONN_TYPE_BLE_PLAIN:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return sc, err
		}

		x, err := GetXport()
		if err != nil {
			return sc, err
		}
		bx := x.(*nmble.BleXport)

		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		if err := config.FillSesnCfg(bx, bc, &sc); err != nil {
			return sc, err
		}

		return sc, nil

	case config.CONN_TYPE_BLE_OIC:
		bc, err := config.ParseBleConnString(cp.ConnString)
		if err != nil {
			return sc, err
		}

		x, err := GetXport()
		if err != nil {
			return sc, err
		}
		bx := x.(*nmble.BleXport)

		sc.MgmtProto = sesn.MGMT_PROTO_OMP
		if err := config.FillSesnCfg(bx, bc, &sc); err != nil {
			return sc, err
		}

		return sc, nil

	case config.CONN_TYPE_UDP_PLAIN:
		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		sc.PeerSpec.Udp = cp.ConnString

		return sc, nil

	case config.CONN_TYPE_UDP_OIC:
		sc.MgmtProto = sesn.MGMT_PROTO_OMP
		sc.PeerSpec.Udp = cp.ConnString

		return sc, nil

	default:
		return sc, util.FmtNewtError("Unknown connection type: %s (%d)",
			config.ConnTypeToString(cp.Type), int(cp.Type))
	}

}

func buildBllSesn(cp *config.ConnProfile) (sesn.Sesn, error) {
	bc, err := config.ParseBllConnString(cp.ConnString)
	if err != nil {
		return nil, err
	}

	x, err := GetXport()
	if err != nil {
		return nil, err
	}
	bx := x.(*bll.BllXport)

	sc, err := config.BuildBllSesnCfg(bc)
	if err != nil {
		return nil, err
	}

	switch cp.Type {
	case config.CONN_TYPE_BLL_PLAIN:
		sc.MgmtProto = sesn.MGMT_PROTO_NMP

	case config.CONN_TYPE_BLL_OIC:
		sc.MgmtProto = sesn.MGMT_PROTO_OMP

	default:
		return nil, util.NewNewtError("ERROR")
	}

	s, err := bx.BuildBllSesn(sc)
	if err != nil {
		return nil, util.ChildNewtError(err)
	}

	return s, nil
}

func GetSesn() (sesn.Sesn, error) {
	if globalSesn != nil {
		return globalSesn, nil
	}

	cp, err := getConnProfile()
	if err != nil {
		return nil, err
	}

	var s sesn.Sesn
	if cp.Type == config.CONN_TYPE_BLL_PLAIN ||
		cp.Type == config.CONN_TYPE_BLL_OIC {

		s, err = buildBllSesn(cp)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}
	} else {
		sc, err := buildSesnCfg()
		if err != nil {
			return nil, err
		}

		x, err := GetXport()
		if err != nil {
			return nil, err
		}

		s, err = x.BuildSesn(sc)
		if err != nil {
			return nil, util.ChildNewtError(err)
		}
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
