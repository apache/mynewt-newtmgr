package oic

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type ResGetFn func(uri string) (coap.COAPCode, []byte)
type ResPutFn func(uri string, data []byte) coap.COAPCode

type Resource struct {
	Uri   string
	GetCb ResGetFn
	PutCb ResPutFn
}

type ResMgr struct {
	uriResMap map[string]Resource
}

func NewResMgr() ResMgr {
	return ResMgr{
		uriResMap: map[string]Resource{},
	}
}

func (rm *ResMgr) Add(r Resource) error {
	if _, ok := rm.uriResMap[r.Uri]; ok {
		return fmt.Errorf("Registration of duplicate CoAP resource: %s", r.Uri)
	}

	rm.uriResMap[r.Uri] = r
	return nil
}

func (rm *ResMgr) Access(m coap.Message) (coap.COAPCode, []byte) {
	paths := m.Path()
	if len(paths) == 0 {
		log.Debugf("Incoming CoAP message does not specify a URI path")
		return coap.NotFound, nil
	}
	path := paths[0]

	r, ok := rm.uriResMap[path]
	if !ok {
		log.Debugf("Incoming CoAP message specifies unknown resource: %s", path)
		return coap.NotFound, nil
	}

	switch m.Code() {
	case coap.GET:
		if r.GetCb == nil {
			return coap.MethodNotAllowed, nil
		} else {
			return r.GetCb(path)
		}

	case coap.PUT:
		if r.PutCb == nil {
			return coap.MethodNotAllowed, nil
		} else {
			return r.PutCb(path, m.Payload()), nil
		}

	default:
		log.Debugf("Don't know how to handle CoAP message with code=%d (%s)",
			m.Code(), m.Code().String())
		return coap.MethodNotAllowed, nil
	}
}

type FixedResourceWriteFn func(val map[string]interface{}) coap.COAPCode

func NewFixedResource(uri string, val map[string]interface{},
	writeCb FixedResourceWriteFn) Resource {

	return Resource{
		Uri: uri,

		GetCb: func(uri string) (coap.COAPCode, []byte) {
			b, err := nmxutil.EncodeCborMap(val)
			if err != nil {
				return coap.InternalServerError, nil
			}
			return coap.Content, b
		},

		PutCb: func(uri string, data []byte) coap.COAPCode {
			m, err := nmxutil.DecodeCborMap(data)
			if err != nil {
				return coap.BadRequest
			}

			code := writeCb(m)
			if code == coap.Created ||
				code == coap.Deleted ||
				code == coap.Changed {

				val = m
			}
			return code
		},
	}
}
