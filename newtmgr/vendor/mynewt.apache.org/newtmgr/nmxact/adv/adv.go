package adv

import (
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type CfgBle struct {
	// Mandatory
	OwnAddrType   bledefs.BleAddrType
	ConnMode      bledefs.BleAdvConnMode
	DiscMode      bledefs.BleAdvDiscMode
	ItvlMin       uint16
	ItvlMax       uint16
	ChannelMap    uint8
	FilterPolicy  bledefs.BleAdvFilterPolicy
	HighDutyCycle bool
	AdvFields     bledefs.BleAdvFields
	RspFields     bledefs.BleAdvFields
	SesnCfg       sesn.SesnCfg

	// Only required for direct advertisements
	PeerAddr *bledefs.BleAddr
}

type Cfg struct {
	Ble CfgBle
}

type Advertiser interface {
	Start(cfg Cfg) (sesn.Sesn, error)
	Stop() error
}

func NewCfg() Cfg {
	return Cfg{
		Ble: CfgBle{
			OwnAddrType: bledefs.BLE_ADDR_TYPE_RANDOM,
			ConnMode:    bledefs.BLE_ADV_CONN_MODE_UND,
			DiscMode:    bledefs.BLE_ADV_DISC_MODE_GEN,
		},
	}
}
