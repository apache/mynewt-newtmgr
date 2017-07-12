package scan

import (
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type ScanPeer struct {
	HwId     string
	PeerSpec sesn.PeerSpec
}

type ScanFn func(peer ScanPeer)

type CfgBle struct {
	ScanPred bledefs.BleAdvPredicate
}

type Cfg struct {
	// General configuration.
	ScanCb  ScanFn
	SesnCfg sesn.SesnCfg

	// Transport-specific configuration.
	Ble CfgBle
}

type Scanner interface {
	Start(cfg Cfg) error
	Stop() error

	// @return                      true if the specified device was found and
	//                                  forgetten;
	//                              false if the specified device is unknown.
	ForgetDevice(hwId string) bool

	ForgetAllDevices()
}

// Constructs a scan configuration suitable for discovery of OMP
// (Newtmgr-over-CoAP) Mynewt devices.
func BleOmpScanCfg(ScanCb ScanFn) Cfg {
	sc := sesn.NewSesnCfg()
	sc.MgmtProto = sesn.MGMT_PROTO_OMP
	sc.Ble.IsCentral = true
	sc.Ble.EncryptWhen = bledefs.BLE_ENCRYPT_PRIV_ONLY
	sc.Ble.OwnAddrType = bledefs.BLE_ADDR_TYPE_RANDOM

	cfg := Cfg{
		ScanCb:  ScanCb,
		SesnCfg: sc,
		Ble: CfgBle{
			ScanPred: func(adv bledefs.BleAdvReport) bool {
				for _, u := range adv.Fields.Uuids16 {
					if u == bledefs.OmpSecSvcUuid {
						return true
					}
				}

				iotUuid, _ := bledefs.ParseUuid(bledefs.OmpUnsecSvcUuid)
				for _, u128 := range adv.Fields.Uuids128 {
					u := bledefs.BleUuid{U128: u128}
					if bledefs.CompareUuids(u, iotUuid) == 0 {
						return true
					}
				}

				return false
			},
		},
	}
	return cfg
}
