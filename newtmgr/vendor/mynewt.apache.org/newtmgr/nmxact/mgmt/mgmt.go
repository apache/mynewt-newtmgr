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
