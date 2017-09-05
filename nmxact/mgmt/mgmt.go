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

func MtuNmp(rawMtu int) int {
	return rawMtu - nmp.NMP_HDR_SIZE
}

func MtuOmp(rawMtu int) int {
	return rawMtu - omp.OMP_MSG_OVERHEAD - nmp.NMP_HDR_SIZE
}

func MtuMgmt(rawMtu int, mgmtProto sesn.MgmtProto) (int, error) {
	switch mgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return MtuNmp(rawMtu), nil

	case sesn.MGMT_PROTO_OMP:
		return MtuOmp(rawMtu), nil

	default:
		return 0, fmt.Errorf("invalid management protocol: %+v", mgmtProto)
	}
}

func EncodeMgmt(mgmtProto sesn.MgmtProto, m *nmp.NmpMsg) ([]byte, error) {
	switch mgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return nmp.EncodeNmpPlain(m)

	case sesn.MGMT_PROTO_OMP:
		return omp.EncodeOmpTcp(m)

	default:
		return nil,
			fmt.Errorf("invalid management protocol: %+v", mgmtProto)
	}
}
