package bll

import (
	"github.com/currantlabs/ble"

	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllSesnCfg struct {
	MgmtProto    sesn.MgmtProto
	AdvFilter    ble.AdvFilter
	PreferredMtu int
}

func NewBllSesnCfg() BllSesnCfg {
	return BllSesnCfg{
		PreferredMtu: 264,
	}
}
