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
	"sync/atomic"

	"github.com/runtimeco/go-coap"
	log "github.com/sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	nmpd     *nmp.Dispatcher
	coapd    *nmcoap.Dispatcher
	stopCh   chan struct{}
	wg       sync.WaitGroup
	rxFilter nmcoap.MsgFilter
	stopped  uint32
}

func NewDispatcher(rxFilter nmcoap.MsgFilter, isTcp bool,
	logDepth int) (*Dispatcher, error) {

	d := &Dispatcher{
		nmpd:     nmp.NewDispatcher(logDepth + 1),
		coapd:    nmcoap.NewDispatcher(isTcp, logDepth+1),
		stopCh:   make(chan struct{}),
		rxFilter: rxFilter,
	}

	// Listen for OMP responses.  This should never fail.
	if err := d.addOmpListener(); err != nil {
		log.Errorf("Unexpected failure to add OMP listener: " + err.Error())
		return nil, err
	}

	return d, nil
}

func (d *Dispatcher) addOmpListener() error {
	// OMP responses are identifiable by the lack of a CoAP token.  Set up a
	// permanent listener to receive these messages.
	mc := nmcoap.MsgCriteria{
		Token: []byte{},
		Path:  "",
	}

	ol, err := d.AddOicListener(mc)
	if err != nil {
		return err
	}

	d.wg.Add(1)
	go func() {
		defer d.RemoveOicListener(mc)
		defer d.wg.Done()

		for {
			select {
			case m := <-ol.RspChan:
				rsp, err := DecodeOmp(m, d.rxFilter)
				if err != nil {
					log.Debugf("OMP decode failure: %s", err.Error())
				} else if rsp != nil {
					d.nmpd.DispatchRsp(rsp)
				} else {
					/* no error, no response */
				}

			case err := <-ol.ErrChan:
				if err != nil {
					log.Debugf("OIC error: %s", err.Error())
				}

			case <-d.stopCh:
				return
			}
		}
	}()

	return nil
}

func (d *Dispatcher) Stop() {
	if !atomic.CompareAndSwapUint32(&d.stopped, 0, 1) {
		return
	}

	close(d.stopCh)
	d.wg.Wait()
}

func (d *Dispatcher) Dispatch(data []byte) bool {
	return d.coapd.Dispatch(data)
}

func (d *Dispatcher) ProcessCoapReq(data []byte) (coap.Message, error) {
	return d.coapd.ProcessCoapReq(data)
}

func (d *Dispatcher) AddOicListener(
	mc nmcoap.MsgCriteria) (*nmcoap.Listener, error) {

	return d.coapd.AddListener(mc)
}

func (d *Dispatcher) RemoveOicListener(
	mc nmcoap.MsgCriteria) *nmcoap.Listener {

	return d.coapd.RemoveListener(mc)
}

func (d *Dispatcher) AddNmpListener(seq uint8) (*nmp.Listener, error) {
	return d.nmpd.AddListener(seq)
}

func (d *Dispatcher) RemoveNmpListener(seq uint8) *nmp.Listener {
	return d.nmpd.RemoveListener(seq)
}

func (d *Dispatcher) ErrorOneNmp(seq uint8, err error) error {
	return d.nmpd.ErrorOne(seq, err)
}

func (d *Dispatcher) ErrorAll(err error) {
	d.nmpd.ErrorAll(err)
	d.coapd.ErrorAll(err)
}

func (d *Dispatcher) SetRxFilter(rxFilter nmcoap.MsgFilter) {
	d.rxFilter = rxFilter
}

func (d *Dispatcher) RxFilter() nmcoap.MsgFilter {
	return d.rxFilter
}
