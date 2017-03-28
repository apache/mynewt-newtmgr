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
	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

type OmpDispatcher struct {
	nd          *nmp.NmpDispatcher
	reassembler *Reassembler
}

func NewOmpDispatcher() *OmpDispatcher {
	return &OmpDispatcher{
		nd:          nmp.NewNmpDispatcher(),
		reassembler: NewReassembler(),
	}
}

func (od *OmpDispatcher) AddListener(seq uint8, rl *nmp.NmpListener) error {
	return od.nd.AddListener(seq, rl)
}

func (od *OmpDispatcher) RemoveListener(seq uint8) *nmp.NmpListener {
	return od.nd.RemoveListener(seq)
}

func (od *OmpDispatcher) FakeRxError(seq uint8, err error) error {
	return od.nd.FakeRxError(seq, err)
}

// Returns true if the response was dispatched.
func (om *OmpDispatcher) Dispatch(data []byte) bool {
	tm := om.reassembler.RxFrag(data)
	if tm == nil {
		return false
	}

	r, err := DecodeOmpTcp(tm)
	if err != nil {
		log.Printf("OMP decode failure: %s", err.Error())
		return false
	}

	return om.nd.DispatchRsp(r)
}
