package nmble

import (
	"github.com/runtimeco/go-coap"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

func GwService(x *BleXport) (BleSvc, error) {
	svcUuid, _ := ParseUuid(UnauthSvcUuid)
	reqChrUuid, _ := ParseUuid(UnauthReqChrUuid)
	rspChrUuid, _ := ParseUuid(UnauthRspChrUuid)

	resources := []oic.Resource{
		oic.NewFixedResource(
			"mynewt.yourmom",
			map[string]interface{}{"yourmom": "fat"},
			func(val map[string]interface{}) coap.COAPCode {
				return coap.Changed
			},
		)}

	return GenCoapService(x, svcUuid, reqChrUuid, rspChrUuid, resources)
}

func SetAllServices(x *BleXport) error {
	gwSvc, err := GwService(x)
	if err != nil {
		return nmxutil.NewXportError(err.Error())
	}

	svcs := []BleSvc{
		GapService("gwadv"),
		GattService(),
		gwSvc,
	}
	if err := x.cm.SetServices(x, svcs); err != nil {
		return nmxutil.NewXportError(err.Error())
	}

	return nil
}
