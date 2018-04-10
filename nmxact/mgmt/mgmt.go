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

package mgmt

import (
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

func EncodeMgmt(s sesn.Sesn, m *nmp.NmpMsg) ([]byte, error) {
	switch s.MgmtProto() {
	case sesn.MGMT_PROTO_NMP:
		return nmp.EncodeNmpPlain(m)

	case sesn.MGMT_PROTO_OMP:
		txCb, _ := s.Filters()
		if s.CoapIsTcp() == true {
			return omp.EncodeOmpTcp(txCb, m)
		} else {
			return omp.EncodeOmpDgram(txCb, m)
		}

	default:
		return nil,
			fmt.Errorf("invalid management protocol: %+v", s.MgmtProto())
	}
}
