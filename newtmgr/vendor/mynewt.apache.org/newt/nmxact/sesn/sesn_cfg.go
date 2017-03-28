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

type BleOnCloseFn func(s Sesn, peer bledefs.BleDev, err error)

// Specifies the BLE peer to connect to.
type BlePeerSpec struct {
	// This is filled in if you know the address of the peer to connect to.
	Dev bledefs.BleDev

	// Otherwise, we must scan for a peer to connect to.  This points to a
	// function that indicates whether we should connect to the sender of the
	// specified advertisement.  This function gets called each time an
	// incoming advertisement is received.  If it returns true, the session
	// will connect to the sender of the corresponding advertisement.  Set this
	// to nil if you populate the Dev field.
	ScanPred bledefs.BleAdvPredicate
}

func BlePeerSpecDev(dev bledefs.BleDev) BlePeerSpec {
	return BlePeerSpec{Dev: dev}
}

func BlePeerSpecName(name string) BlePeerSpec {
	return BlePeerSpec{
		ScanPred: func(r bledefs.BleAdvReport) bool {
			return r.Name == name
		},
	}
}

type SesnCfgBle struct {
	OwnAddrType  bledefs.BleAddrType
	PeerSpec     BlePeerSpec
	CloseTimeout time.Duration
	OnCloseCb    BleOnCloseFn
}

type SesnCfg struct {
	// Used with all transport types.
	MgmtProto MgmtProto

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
