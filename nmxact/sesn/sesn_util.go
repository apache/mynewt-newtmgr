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

func GetResource(s Sesn, resType ResourceType, uri string, o TxOptions) (
	coap.COAPCode, []byte, error) {

	retries := o.Tries - 1
	for i := 0; ; i++ {
		code, r, err := s.GetResourceOnce(resType, uri, o)
		if err == nil {
			return code, r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return code, nil, err
		}
	}
}

func PutResource(s Sesn, resType ResourceType, uri string,
	value []byte, o TxOptions) (coap.COAPCode, error) {

	retries := o.Tries - 1
	for i := 0; ; i++ {
		code, err := s.PutResourceOnce(resType, uri, value, o)
		if err == nil {
			return code, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return code, err
		}
	}
}

func PutCborResource(s Sesn, resType ResourceType, uri string,
	value map[string]interface{}, o TxOptions) (coap.COAPCode, error) {

	b, err := nmxutil.EncodeCborMap(value)
	if err != nil {
		return 0, err
	}

	return PutResource(s, resType, uri, b, o)
}
