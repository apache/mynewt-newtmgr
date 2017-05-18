package scan

import (
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type ScanPeer struct {
	HwId   []byte
	Opaque interface{}
}

type ScanFn func(peer ScanPeer)

type Cfg struct {
	ScanCb  ScanFn
	SesnCfg sesn.SesnCfg
}

type Scanner interface {
	Start(cfg Cfg) error
	Stop() error
}

// Constructs a scan configuration suitable for discovery of OMP
// (Newtmgr-over-CoAP) Mynewt devices.
func BleOmpScanCfg(ScanCb ScanFn) Cfg {
	sc := sesn.NewSesnCfg()
	sc.MgmtProto = sesn.MGMT_PROTO_OMP
	sc.Ble.OwnAddrType = bledefs.BLE_ADDR_TYPE_RANDOM
	sc.Ble.Encrypt = bledefs.BLE_ENCRYPT_PRIV_ONLY
	sc.Ble.PeerSpec = sesn.BlePeerSpec{
		ScanPred: func(adv bledefs.BleAdvReport) bool {
			for _, u := range adv.Uuids16 {
				if u == bledefs.OmpSvcUuid {
					return true
				}
			}

			return false
		},
	}

	cfg := Cfg{
		ScanCb:  ScanCb,
		SesnCfg: sc,
	}

	return cfg
}
