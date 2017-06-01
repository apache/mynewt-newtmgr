package sesn

import (
	"time"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type MgmtProto int

const (
	MGMT_PROTO_NMP MgmtProto = iota
	MGMT_PROTO_OMP
)

type OnCloseFn func(s Sesn, err error)

type PeerSpec struct {
	Ble bledefs.BleDev
	Udp string
}

type SesnCfgBle struct {
	OwnAddrType  bledefs.BleAddrType
	ConnTries    int
	CloseTimeout time.Duration

	EncryptWhen bledefs.BleEncryptWhen
}

type SesnCfg struct {
	// General configuration.
	MgmtProto MgmtProto
	PeerSpec  PeerSpec
	OnCloseCb OnCloseFn

	// Transport-specific configuration.
	Ble SesnCfgBle
}

func NewSesnCfg() SesnCfg {
	return SesnCfg{
		// XXX: For now, assume an own address type of random static.  In the
		// future, there will need to be some global default, or something that
		// gets read from blehostd.
		Ble: SesnCfgBle{
			OwnAddrType:  bledefs.BLE_ADDR_TYPE_RANDOM,
			ConnTries:    3,
			CloseTimeout: 30 * time.Second,
		},
	}
}
