/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	path := m.PathString()

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

type CborResourceGetFn func(uri string) (coap.COAPCode, map[string]interface{})

type CborResourcePutFn func(uri string,
	val map[string]interface{}) coap.COAPCode

func NewCborResource(uri string,
	getCb CborResourceGetFn, putCb CborResourcePutFn) Resource {

	return Resource{
		Uri: uri,

		GetCb: func(uri string) (coap.COAPCode, []byte) {
			if getCb == nil {
				return coap.MethodNotAllowed, nil
			}

			code, m := getCb(uri)
			if code >= coap.BadRequest {
				return code, nil
			}
			b, err := nmxutil.EncodeCborMap(m)
			if err != nil {
				return coap.InternalServerError, nil
			}
			return code, b
		},

		PutCb: func(uri string, data []byte) coap.COAPCode {
			if putCb == nil {
				return coap.MethodNotAllowed
			}

			m, err := nmxutil.DecodeCborMap(data)
			if err != nil {
				return coap.BadRequest
			}

			return putCb(uri, m)
		},
	}
}

func NewFixedResource(uri string, val map[string]interface{},
	putCb CborResourcePutFn) Resource {

	return NewCborResource(
		// URI.
		uri,

		// Get.
		func(uri string) (coap.COAPCode, map[string]interface{}) {
			return coap.Content, val
		},

		// Put.
		func(uri string, newVal map[string]interface{}) coap.COAPCode {
			if putCb == nil {
				return coap.MethodNotAllowed
			}

			code := putCb(uri, newVal)
			if code == coap.Created || code == coap.Changed {
				val = newVal
			}
			return code
		},
	)
}
