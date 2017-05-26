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

type SesnCfgBleCentral struct {
	ConnTries int
	// XXX: Missing fields.
}

type SesnCfgBlePeriph struct {
	Duration      time.Duration
	ConnMode      bledefs.BleAdvConnMode
	DiscMode      bledefs.BleAdvDiscMode
	ItvlMin       uint16
	ItvlMax       uint16
	ChannelMap    uint8
	FilterPolicy  bledefs.BleAdvFilterPolicy
	HighDutyCycle bool
	AdvFields     bledefs.BleAdvFields
	RspFields     bledefs.BleAdvFields
}

type SesnCfgBle struct {
	// General configuration.
	IsCentral    bool
	OwnAddrType  bledefs.BleAddrType
	EncryptWhen  bledefs.BleEncryptWhen
	CloseTimeout time.Duration

	// Central configuration.
	Central SesnCfgBleCentral

	// Peripheral configuration.
	Periph SesnCfgBlePeriph
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
			IsCentral:    true,
			OwnAddrType:  bledefs.BLE_ADDR_TYPE_RANDOM,
			CloseTimeout: 30 * time.Second,

			Central: SesnCfgBleCentral{
				ConnTries: 3,
			},
			Periph: SesnCfgBlePeriph{},
		},
	}
}
