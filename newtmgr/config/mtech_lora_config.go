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
	"strconv"
	"strings"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
	"mynewt.apache.org/newtmgr/nmxact/lora"
	"mynewt.apache.org/newtmgr/nmxact/mtech_lora"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func NewMtechLoraConfig() *mtech_lora.LoraConfig {
	return &mtech_lora.LoraConfig{
		Addr:        "",
		SegSz:       0,
		ConfirmedTx: false,
		Port:        lora.COAP_LORA_PORT,
	}
}

func ParseMtechLoraConnString(cs string) (*mtech_lora.LoraConfig, error) {
	mc := NewMtechLoraConfig()

	if len(cs) == 0 {
		return mc, nil
	}
	parts := strings.Split(cs, ",")
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return nil, util.FmtNewtError("expected comma-separated "+
				"key=value pairs; no '=' in: %s", p)
		}

		k := kv[0]
		v := kv[1]

		switch k {
		case "addr":
			mc.Addr = v
		case "segsz":
			var err error
			mc.SegSz, err = strconv.Atoi(v)
			if err != nil {
				return mc, util.FmtNewtError("Invalid SegSz: %s", v)
			}
		case "confirmedtx":
			var err error
			mc.ConfirmedTx, err = strconv.ParseBool(v)
			if err != nil {
				return mc, util.FmtNewtError("Invalid confirmedtx: %s", v)
			}
		case "port":
			port, err := strconv.ParseUint(v, 10, 8)
			if err != nil {
				return mc, util.FmtNewtError("Invalid port number: %s", v)
			}
			mc.Port = uint8(port)
		default:
			return nil, util.FmtNewtError("Unrecognized key: %s", k)
		}
	}

	return mc, nil
}

func FillMtechLoraSesnCfg(mc *mtech_lora.LoraConfig, sc *sesn.SesnCfg) error {
	sc.Lora.Addr = mc.Addr
	sc.Lora.SegSz = mc.SegSz
	sc.Lora.ConfirmedTx = mc.ConfirmedTx
	sc.Lora.Port = mc.Port
	if nmutil.DeviceName != "" {
		sc.Lora.Addr = nmutil.DeviceName
	}
	return nil
}
