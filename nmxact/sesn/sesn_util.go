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

package sesn

import (
	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

func TxNmp(s Sesn, m *nmp.NmpMsg, o TxOptions) (nmp.NmpRsp, error) {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		r, err := s.TxNmpOnce(m, o)
		if err == nil {
			return r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return nil, err
		}
	}
}

func getResourceOnce(s Sesn, uri string,
	opt TxOptions) (coap.COAPCode, []byte, error) {

	req, err := nmcoap.CreateGet(s.CoapIsTcp(), uri, -1, nmxutil.NextToken())
	if err != nil {
		return 0, nil, err
	}

	return s.TxCoapOnce(req, opt)
}

func getResource(s Sesn, uri string, observe int,
	token []byte, opt TxOptions, NotifCb GetNotifyCb,
	stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {

	var req coap.Message
	var err error

	if observe == 1 {
		req, err = nmcoap.CreateGet(s.CoapIsTcp(), uri, observe, token)
	} else {
		req, err = nmcoap.CreateGet(s.CoapIsTcp(), uri, observe, nmxutil.NextToken())
	}
	if err != nil {
		return 0, nil, nil, err
	}

	if observe == 0 {
		return s.TxCoapObserve(req, opt, NotifCb, stopsignal)
	}

	c, r, err := s.TxCoapOnce(req, opt)
	return c, r, nil, err
}

func putResourceOnce(s Sesn, uri string, value []byte,
	opt TxOptions) (coap.COAPCode, []byte, error) {

	req, err := nmcoap.CreatePut(s.CoapIsTcp(), uri, nmxutil.NextToken(),
		value)
	if err != nil {
		return 0, nil, err
	}

	return s.TxCoapOnce(req, opt)
}

func postResourceOnce(s Sesn, uri string, value []byte,
	opt TxOptions) (coap.COAPCode, []byte, error) {

	req, err := nmcoap.CreatePost(s.CoapIsTcp(), uri, nmxutil.NextToken(),
		value)
	if err != nil {
		return 0, nil, err
	}

	return s.TxCoapOnce(req, opt)
}

func deleteResourceOnce(s Sesn, uri string, value []byte,
	opt TxOptions) (coap.COAPCode, []byte, error) {

	req, err := nmcoap.CreateDelete(s.CoapIsTcp(), uri, nmxutil.NextToken(),
		value)
	if err != nil {
		return 0, nil, err
	}

	return s.TxCoapOnce(req, opt)
}

func txCoap(txCb func() (coap.COAPCode, []byte, error),
	tries int) (coap.COAPCode, []byte, error) {

	retries := tries - 1
	for i := 0; ; i++ {
		code, r, err := txCb()
		if err == nil {
			return code, r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return code, nil, err
		}
	}
}

func txCoapObserve(txCb func() (coap.COAPCode, []byte, []byte, error),
	tries int) (coap.COAPCode, []byte, []byte, error) {

	retries := tries - 1
	for i := 0; ; i++ {
		code, r, t, err := txCb()
		if err == nil {
			return code, r, t, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return code, nil, nil, err
		}
	}
}

func GetResourceObserve(s Sesn, uri string, o TxOptions, notifCb GetNotifyCb,
	stopsignal chan int, observe int, token []byte) (
	coap.COAPCode, []byte, []byte, error) {

	return txCoapObserve(func() (coap.COAPCode, []byte, []byte, error) {
		return getResource(s, uri, observe, token, o, notifCb, stopsignal)
	}, o.Tries)
}

func GetResource(s Sesn, uri string, o TxOptions) (
	coap.COAPCode, []byte, error) {

	return txCoap(func() (coap.COAPCode, []byte, error) {
		return getResourceOnce(s, uri, o)
	}, o.Tries)
}

func PutResource(s Sesn, uri string,
	value []byte, o TxOptions) (coap.COAPCode, []byte, error) {

	return txCoap(func() (coap.COAPCode, []byte, error) {
		return putResourceOnce(s, uri, value, o)
	}, o.Tries)
}

func PostResource(s Sesn, uri string,
	value []byte, o TxOptions) (coap.COAPCode, []byte, error) {

	return txCoap(func() (coap.COAPCode, []byte, error) {
		return postResourceOnce(s, uri, value, o)
	}, o.Tries)
}

func DeleteResource(s Sesn, uri string, value []byte,
	o TxOptions) (coap.COAPCode, []byte, error) {

	return txCoap(func() (coap.COAPCode, []byte, error) {
		return deleteResourceOnce(s, uri, value, o)
	}, o.Tries)
}

func PutCborResource(s Sesn, uri string, value map[string]interface{},
	o TxOptions) (coap.COAPCode, map[string]interface{}, error) {

	b, err := nmxutil.EncodeCborMap(value)
	if err != nil {
		return 0, nil, err
	}

	code, r, err := PutResource(s, uri, b, o)
	m, err := nmxutil.DecodeCborMap(r)
	if err != nil {
		return 0, nil, err
	}

	return code, m, nil
}
