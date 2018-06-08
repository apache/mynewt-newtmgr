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

	log "github.com/Sirupsen/logrus"
        "github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	nmpd       *nmp.Dispatcher
	oicd       *nmcoap.Dispatcher
	stopCh     chan struct{}
	wg         sync.WaitGroup
	stopped    uint32
	txFilterCb nmcoap.MsgFilter
	rxFilterCb nmcoap.MsgFilter
}

func NewDispatcher(txFilterCb, rxFilterCb nmcoap.MsgFilter, isTcp bool, logDepth int) (*Dispatcher, error) {
	d := &Dispatcher{
		nmpd:       nmp.NewDispatcher(logDepth + 1),
		oicd:       nmcoap.NewDispatcher(isTcp, logDepth+1),
		stopCh:     make(chan struct{}),
		txFilterCb: txFilterCb,
		rxFilterCb: rxFilterCb,
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
	ol, err := d.AddOicListener(nil)
	if err != nil {
		return err
	}

	d.wg.Add(1)
	go func() {
		defer d.RemoveOicListener(nil)
		defer d.wg.Done()

		for {
			select {
			case m := <-ol.RspChan:
				rsp, err := DecodeOmp(m, d.rxFilterCb)
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
	return d.oicd.Dispatch(data)
}

func (d *Dispatcher) ProcessCoapReq(data []byte) (coap.Message, error) {
	return d.oicd.ProcessCoapReq(data)
}

func (d *Dispatcher) AddOicListener(token []byte) (*nmcoap.Listener, error) {
	return d.oicd.AddListener(token)
}

func (d *Dispatcher) RemoveOicListener(token []byte) *nmcoap.Listener {
	return d.oicd.RemoveListener(token)
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
	d.oicd.ErrorAll(err)
}

func (d *Dispatcher) Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter) {
	return d.txFilterCb, d.rxFilterCb
}
