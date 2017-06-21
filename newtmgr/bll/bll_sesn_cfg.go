package bll

import (
	"time"

	"github.com/currantlabs/ble"

	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type BllSesnCfg struct {
	MgmtProto    sesn.MgmtProto
	AdvFilter    ble.AdvFilter
	PreferredMtu int
	ConnTimeout  time.Duration
}

func NewBllSesnCfg() BllSesnCfg {
	return BllSesnCfg{
		PreferredMtu: 527,
		ConnTimeout:  10 * time.Second,
	}
}
