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
	"time"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// TxRxMgmt sends a management command (NMP / OMP) and listens for the
// response.
func TxRxMgmt(s Sesn, m *nmp.NmpMsg, o TxOptions) (nmp.NmpRsp, error) {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		r, err := s.TxRxMgmt(m, o.Timeout)
		if err == nil {
			return r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return nil, err
		}
	}
}

func TxRxMgmtAsync(s Sesn, m *nmp.NmpMsg, o TxOptions, ch chan nmp.NmpRsp, errc chan error) error {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		err := s.TxRxMgmtAsync(m, o.Timeout, ch, errc)
		if err == nil {
			return nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return err
		}
	}
}

// TxCoap transmits a single CoAP message over the provided session.
func TxCoap(s Sesn, mp nmcoap.MsgParams) error {
	msg, err := nmcoap.CreateMsg(s.CoapIsTcp(), mp)
	if err != nil {
		return err
	}

	return s.TxCoap(msg)
}

// RxCoap performs a blocking receive of a CoAP message.  It returns a nil
// message if the specified listener is closed while the function is running.
func RxCoap(cl *nmcoap.Listener, timeout time.Duration) (coap.Message, error) {
	if timeout != 0 {
		for {
			select {
			case err := <-cl.ErrChan:
				return nil, err
			case rsp := <-cl.RspChan:
				return rsp, nil
			case _, ok := <-cl.AfterTimeout(timeout):
				if ok {
					return nil, nmxutil.NewRspTimeoutError("CoAP timeout")
				}
			}
		}
	} else {
		select {
		case err := <-cl.ErrChan:
			return nil, err
		case rsp := <-cl.RspChan:
			return rsp, nil
		}
	}
}

// TxRxMgmt sends a CoAP request and listens for the response.
func TxRxCoap(s Sesn, mp nmcoap.MsgParams,
	opts TxOptions) (coap.Message, error) {

	mc := nmcoap.MsgCriteria{Token: mp.Token}
	cl, err := s.ListenCoap(mc)
	if err != nil {
		return nil, err
	}
	defer s.StopListenCoap(mc)

	listenOnce := func() (coap.Message, error) {
		return RxCoap(cl, opts.Timeout)
	}

	retries := opts.Tries - 1
	for i := 0; ; i++ {
		if err := TxCoap(s, mp); err != nil {
			return nil, err
		}

		rsp, err := listenOnce()
		if err == nil {
			return rsp, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return nil, err
		}
	}
}
