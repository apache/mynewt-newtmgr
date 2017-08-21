package nmble

import (
	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

type BleOicSvr struct {
	s          oic.Server
	x          *BleXport
	rspSvcUuid BleUuid
	rspChrUuid BleUuid
}

func NewBleOicSvr(x *BleXport,
	rspSvcUuid BleUuid, rspChrUuid BleUuid) BleOicSvr {

	return BleOicSvr{
		s:          oic.NewServer(true),
		x:          x,
		rspSvcUuid: rspSvcUuid,
		rspChrUuid: rspChrUuid,
	}
}

func (b *BleOicSvr) AddResource(r oic.Resource) error {
	return b.s.AddResource(r)
}

func (b *BleOicSvr) Rx(access BleGattAccess) uint8 {
	ml, err := b.s.Rx(access.Data)
	if err != nil {
		log.Debugf("Error processing incoming CoAP message: %s", err.Error())
		return 0
	}

	if ml == nil {
		// Partial CoAP message; remainder forthcoming.
		return 0
	}

	s := b.x.findSesn(access.ConnHandle)
	if s == nil {
		// The sender is no longer connected.
		log.Debugf("Failed to send CoAP response; peer no longer "+
			"connected (conn_handle=%d)", access.ConnHandle)
		return ERR_CODE_ATT_UNLIKELY
	}

	data, err := ml.MarshalBinary()
	if err != nil {
		log.Debugf("Failed to send CoAP response; error marshalling "+
			"response: %s", err.Error())
		return ERR_CODE_ATT_UNLIKELY
	}

	_, valHandle, err := FindChrXact(b.x, b.rspSvcUuid, b.rspChrUuid)
	if err != nil {
		log.Debugf("Failed to send CoAP response; cannot find response "+
			"characteristic: (s=%s c=%s)",
			b.rspSvcUuid.String(), b.rspChrUuid.String())
		return ERR_CODE_ATT_UNLIKELY
	}

	mtu := s.MtuOut()
	frags := nmxutil.Fragment(data, mtu)
	for _, frag := range frags {
		if err := NotifyXact(b.x, access.ConnHandle, valHandle,
			frag); err != nil {

			log.Debugf("Failed to send CoAP response; failed to send "+
				"fragment: %s", err.Error())
			return ERR_CODE_ATT_UNLIKELY
		}
	}

	return 0
}
