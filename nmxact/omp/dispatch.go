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
	"fmt"
	"sync"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type Listener struct {
	nmpl   *nmp.Listener
	coapl  *nmcoap.Listener
	stopCh chan struct{}
}

// The dispatcher is the owner of the listeners it points to.  Only the
// dispatcher writes to these listeners.
type Dispatcher struct {
	seqListenerMap map[uint8]*Listener
	coapd          *nmcoap.Dispatcher
	wg             sync.WaitGroup
	rxFilter       nmcoap.MsgFilter
	stopped        bool
	logDepth       int
	mtx            sync.Mutex
}

func NewDispatcher(rxFilter nmcoap.MsgFilter, isTcp bool,
	logDepth int) (*Dispatcher, error) {

	d := &Dispatcher{
		seqListenerMap: map[uint8]*Listener{},
		coapd:          nmcoap.NewDispatcher(isTcp, logDepth+1),
		rxFilter:       rxFilter,
		logDepth:       logDepth + 2,
	}

	return d, nil
}

func (d *Dispatcher) addOmpListener(seq uint8) (*Listener, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.seqListenerMap[seq] != nil {
		return nil, fmt.Errorf("duplicate OMP listener; seq=%d", seq)
	}

	mc := nmcoap.MsgCriteria{
		Token: nmxutil.SeqToToken(seq),
		Path:  "",
	}

	ol, err := d.AddCoapListener(mc)
	if err != nil {
		return nil, err
	}

	ompl := &Listener{
		nmpl:   nmp.NewListener(),
		coapl:  ol,
		stopCh: make(chan struct{}),
	}
	d.seqListenerMap[seq] = ompl

	d.wg.Add(1)
	go func() {
		defer d.RemoveCoapListener(mc)
		defer d.wg.Done()

		// Listen for events.  All feedback is sent to the client via the NMP
		// listener channels.  It is done this way so that client code can be
		// OMP/NMP agnostic.
		for {
			select {
			case m := <-ompl.coapl.RspChan:
				rsp, err := DecodeOmp(m, d.rxFilter)
				if err != nil {
					ompl.nmpl.ErrChan <- err
				} else if rsp != nil {
					ompl.nmpl.RspChan <- rsp
				} else {
					/* no error, no response */
				}

			case err := <-ompl.coapl.ErrChan:
				if err != nil {
					ompl.nmpl.ErrChan <- err
				}

			case <-ompl.stopCh:
				return
			}
		}
	}()

	return ompl, nil
}

func (d *Dispatcher) Stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	if d.stopped {
		return
	}
	d.stopped = true

	for seq, ompl := range d.seqListenerMap {
		delete(d.seqListenerMap, seq)
		close(ompl.stopCh)
	}
	d.wg.Wait()
}

func (d *Dispatcher) Dispatch(data []byte) bool {
	return d.coapd.Dispatch(data)
}

func (d *Dispatcher) ProcessCoapReq(data []byte) (coap.Message, error) {
	return d.coapd.ProcessCoapReq(data)
}

func (d *Dispatcher) AddCoapListener(
	mc nmcoap.MsgCriteria) (*nmcoap.Listener, error) {

	return d.coapd.AddListener(mc)
}

func (d *Dispatcher) RemoveCoapListener(
	mc nmcoap.MsgCriteria) *nmcoap.Listener {

	return d.coapd.RemoveListener(mc)
}

func (d *Dispatcher) AddNmpListener(seq uint8) (*nmp.Listener, error) {
	ompl, err := d.addOmpListener(seq)
	if err != nil {
		return nil, err
	}

	nmxutil.LogAddNmpListener(d.logDepth, seq)
	return ompl.nmpl, nil
}

func (d *Dispatcher) RemoveNmpListener(seq uint8) *nmp.Listener {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	ompl := d.seqListenerMap[seq]
	if ompl != nil {
		delete(d.seqListenerMap, seq)
		close(ompl.stopCh)
	}

	nmxutil.LogRemoveNmpListener(d.logDepth, seq)
	return ompl.nmpl
}

func (d *Dispatcher) ErrorOneNmp(seq uint8, err error) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	ompl := d.seqListenerMap[seq]
	if ompl == nil {
		return fmt.Errorf("no nmp listener for seq %d", seq)
	}

	ompl.nmpl.ErrChan <- err
	return nil
}

func (d *Dispatcher) ErrorAll(err error) {
	d.coapd.ErrorAll(err)
}

func (d *Dispatcher) SetRxFilter(rxFilter nmcoap.MsgFilter) {
	d.rxFilter = rxFilter
}

func (d *Dispatcher) RxFilter() nmcoap.MsgFilter {
	return d.rxFilter
}
