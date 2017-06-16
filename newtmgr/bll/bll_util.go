package bll

import (
	"encoding/binary"
	"fmt"

	"github.com/currantlabs/ble"

	"mynewt.apache.org/newtmgr/nmxact/bledefs"
)

func UuidFromBllUuid(bllUuid ble.UUID) (bledefs.BleUuid, error) {
	uuid := bledefs.BleUuid{}

	switch len(bllUuid) {
	case 2:
		uuid.U16 = bledefs.BleUuid16(binary.LittleEndian.Uint16(bllUuid))
		return uuid, nil

	case 16:
		for i, b := range bllUuid {
			uuid.U128[15-i] = b
		}
		return uuid, nil

	default:
		return uuid, fmt.Errorf("Invalid UUID: %#v", bllUuid)
	}
}
