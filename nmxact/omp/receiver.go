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

package omp

import (
	"sync"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

type Receiver struct {
	nmpd *nmp.NmpDispatcher
	oicd *oic.OicDispatcher
	nls  map[*nmp.NmpListener]struct{}
	ols  map[*oic.OicListener]struct{}

	mtx sync.Mutex
}

func NewReceiver(isTcp bool) *Receiver {
	r := &Receiver{
		nmpd: nmp.NewNmpDispatcher(),
		oicd: oic.NewOicDispatcher(isTcp),
		nls:  map[*nmp.NmpListener]struct{}{},
		ols:  map[*oic.OicListener]struct{}{},
	}

	// Listen for OMP responses.  This should never fail.
	if err := r.addOmpListener(); err != nil {
		panic("Unexpected failure to add OMP listener: " + err.Error())
	}

	return r
}

func (r *Receiver) addOmpListener() error {
	ol, err := r.AddOicListener(nil)
	if err != nil {
		return err
	}

	go func() {
		defer r.RemoveOicListener(nil)

		for {
			select {
			case m := <-ol.RspChan:
				rsp, err := DecodeOmp(m)
				if err != nil {
					log.Debugf("OMP decode failure: %s", err.Error())
				} else {
					r.nmpd.DispatchRsp(rsp)
				}

			case err := <-ol.ErrChan:
				log.Debugf("OIC error: %s", err.Error())
				return
			}
		}
	}()

	return nil
}

func (r *Receiver) Rx(data []byte) bool {
	return r.oicd.Dispatch(data)
}

func (r *Receiver) AddOicListener(token []byte) (*oic.OicListener, error) {
	nmxutil.LogAddOicListener(3, token)

	ol := oic.NewOicListener()
	if err := r.oicd.AddListener(token, ol); err != nil {
		return nil, err
	}

	r.mtx.Lock()
	r.ols[ol] = struct{}{}
	r.mtx.Unlock()

	return ol, nil
}

func (r *Receiver) RemoveOicListener(token []byte) *oic.OicListener {
	nmxutil.LogRemoveOicListener(3, token)

	listener := r.oicd.RemoveListener(token)
	if listener != nil {
		r.mtx.Lock()
		delete(r.ols, listener)
		r.mtx.Unlock()
	}

	return listener
}

func (r *Receiver) AddNmpListener(seq uint8) (*nmp.NmpListener, error) {
	nmxutil.LogAddNmpListener(3, seq)

	nl := nmp.NewNmpListener()
	if err := r.nmpd.AddListener(seq, nl); err != nil {
		return nil, err
	}

	r.mtx.Lock()
	r.nls[nl] = struct{}{}
	r.mtx.Unlock()

	return nl, nil
}

func (r *Receiver) RemoveNmpListener(seq uint8) *nmp.NmpListener {
	nmxutil.LogRemoveNmpListener(3, seq)

	listener := r.nmpd.RemoveListener(seq)
	if listener != nil {
		r.mtx.Lock()
		delete(r.nls, listener)
		r.mtx.Unlock()
	}

	return listener
}

func (r *Receiver) FakeNmpError(seq uint8, err error) error {
	return r.nmpd.FakeRxError(seq, err)
}

func (r *Receiver) ErrorAll(err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for nl, _ := range r.nls {
		nl.ErrChan <- err
	}

	for ol, _ := range r.ols {
		ol.ErrChan <- err
	}
}
