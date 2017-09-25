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

package config

import (
	"fmt"
	"strconv"
	"strings"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newt/util"
)

func einvalSerialConnString(f string, args ...interface{}) error {
	suffix := fmt.Sprintf(f, args)
	return util.FmtNewtError("Invalid serial connstring; %s", suffix)
}

func ParseSerialConnString(cs string) (*nmserial.XportCfg, error) {
	sc := nmserial.NewXportCfg()
	sc.Baud = 115200
	sc.ReadTimeout = nmutil.TxOptions().Timeout

	parts := strings.Split(cs, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		// Handle old-style conn string (single token indicating dev file).
		if len(kv) == 1 {
			kv = []string{"dev", kv[0]}
		}

		k := kv[0]
		v := kv[1]

		switch k {
		case "dev":
			sc.DevPath = v

		case "baud":
			var err error
			sc.Baud, err = strconv.Atoi(v)
			if err != nil {
				return sc, einvalSerialConnString("Invalid baud: %s", v)
			}

		case "mtu":
			var err error
			sc.Mtu, err = strconv.Atoi(v)
			if err != nil {
				return sc, einvalSerialConnString("Invalid mtu: %s", v)
			}

		default:
			return sc, einvalSerialConnString("Unrecognized key: %s", k)
		}
	}

	return sc, nil
}

func BuildSerialXport(sc *nmserial.XportCfg) (*nmserial.SerialXport, error) {
	sx := nmserial.NewSerialXport(sc)
	if err := sx.Start(); err != nil {
		return nil, util.ChildNewtError(err)
	}

	return sx, nil
}
