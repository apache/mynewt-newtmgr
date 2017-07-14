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
	"mynewt.apache.org/newtmgr/nmxact/oic"
)

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	nmpd   *nmp.Dispatcher
	oicd   *oic.Dispatcher
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewDispatcher(isTcp bool, logDepth int) (*Dispatcher, error) {
	r := &Dispatcher{
		nmpd:   nmp.NewDispatcher(logDepth + 1),
		oicd:   oic.NewDispatcher(isTcp, logDepth+1),
		stopCh: make(chan struct{}),
	}

	// Listen for OMP responses.  This should never fail.
	if err := r.addOmpListener(); err != nil {
		log.Errorf("Unexpected failure to add OMP listener: " + err.Error())
		return nil, err
	}

	return r, nil
}

func (r *Dispatcher) addOmpListener() error {
	// OMP responses are identifiable by the lack of a CoAP token.  Set up a
	// permanent listener to receive these messages.
	ol, err := r.AddOicListener(nil)
	if err != nil {
		return err
	}

	r.wg.Add(1)
	go func() {
		defer r.RemoveOicListener(nil)
		defer r.wg.Done()

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

			case <-r.stopCh:
				return
			}
		}
	}()

	return nil
}

func (r *Dispatcher) Stop() {
	r.stopCh <- struct{}{}
	r.wg.Wait()
}

func (r *Dispatcher) Dispatch(data []byte) bool {
	return r.oicd.Dispatch(data)
}

func (r *Dispatcher) AddOicListener(token []byte) (*oic.Listener, error) {
	return r.oicd.AddListener(token)
}

func (r *Dispatcher) RemoveOicListener(token []byte) *oic.Listener {
	return r.oicd.RemoveListener(token)
}

func (r *Dispatcher) AddNmpListener(seq uint8) (*nmp.Listener, error) {
	return r.nmpd.AddListener(seq)
}

func (r *Dispatcher) RemoveNmpListener(seq uint8) *nmp.Listener {
	return r.nmpd.RemoveListener(seq)
}

func (r *Dispatcher) ErrorOneNmp(seq uint8, err error) error {
	return r.nmpd.ErrorOne(seq, err)
}

func (r *Dispatcher) ErrorAll(err error) {
	r.nmpd.ErrorAll(err)
	r.oicd.ErrorAll(err)
}
