package oic

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"
)

type ResWriteFn func(data []byte) coap.COAPCode
type ResReadFn func(data []byte) (coap.COAPCode, []byte)

type Resource struct {
	Name    string
	WriteCb ResWriteFn
	ReadCb  ResReadFn
}

type ResMgr struct {
	nameResMap map[string]Resource
}

func NewResMgr() ResMgr {
	return ResMgr{
		nameResMap: map[string]Resource{},
	}
}

func (rm *ResMgr) Add(r Resource) error {
	if _, ok := rm.nameResMap[r.Name]; ok {
		return fmt.Errorf("Registration of duplicate CoAP resource: %s",
			r.Name)
	}

	rm.nameResMap[r.Name] = r
	return nil
}

func (rm *ResMgr) Access(m coap.Message) (coap.COAPCode, []byte) {
	paths := m.Path()
	if len(paths) == 0 {
		log.Debugf("Incoming CoAP message does not specify a URI path")
		return coap.NotFound, nil
	}
	path := paths[0]

	r, ok := rm.nameResMap[path]
	if !ok {
		log.Debugf("Incoming CoAP message specifies unknown resource: %s", path)
		return coap.NotFound, nil
	}

	switch m.Code() {
	case coap.GET:
		if r.ReadCb == nil {
			return coap.MethodNotAllowed, nil
		} else {
			return r.ReadCb(m.Payload())
		}

	case coap.PUT:
		if r.WriteCb == nil {
			return coap.MethodNotAllowed, nil
		} else {
			return r.WriteCb(m.Payload()), nil
		}

	default:
		log.Debugf("Don't know how to handle CoAP message with code=%d (%s)",
			m.Code(), m.Code().String())
		return coap.MethodNotAllowed, nil
	}
}
