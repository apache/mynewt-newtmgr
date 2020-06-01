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

package nmcoap

import (
	"github.com/runtimeco/go-coap"
	log "github.com/sirupsen/logrus"
)

type Reassembler struct {
	cur []byte
}

func NewReassembler() *Reassembler {
	return &Reassembler{}
}

func (r *Reassembler) RxFrag(frag []byte) *coap.TcpMessage {
	r.cur = append(r.cur, frag...)

	var tm *coap.TcpMessage
	var err error
	tm, r.cur, err = coap.PullTcp(r.cur)
	if err != nil {
		log.Debugf("received invalid CoAP-TCP packet: %s", err.Error())
		return nil
	}

	if tm == nil {
		return nil
	}

	r.cur = nil
	return tm
}
