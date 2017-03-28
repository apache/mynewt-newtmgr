package sesn

import (
	"time"

	"mynewt.apache.org/newt/nmxact/bledefs"
)

type MgmtProto int

const (
	MGMT_PROTO_NMP MgmtProto = iota
	MGMT_PROTO_OMP
)

type OnCloseFn func(s Sesn, err error)

type SesnCfgBle struct {
	OwnAddrType  bledefs.BleAddrType
	Peer         bledefs.BlePeerSpec
	CloseTimeout time.Duration
}

type SesnCfg struct {
	// Used with all transport types.
	MgmtProto MgmtProto
	OnCloseCb OnCloseFn

	// Only used with BLE transports.
	Ble SesnCfgBle
}

func NewSesnCfg() SesnCfg {
	return SesnCfg{
		Ble: SesnCfgBle{
			CloseTimeout: 5 * time.Second,
		},
	}
}
