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
	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"
)

type Receiver struct {
	reassembler *Reassembler
}

func NewReceiver(isTcp bool) Receiver {
	r := Receiver{}

	if isTcp {
		r.reassembler = NewReassembler()
	}

	return r
}

func (r *Receiver) Rx(data []byte) coap.Message {
	if r.reassembler != nil {
		// TCP.
		tm := r.reassembler.RxFrag(data)
		if tm == nil {
			return nil
		}
		return tm
	} else {
		// UDP.
		m, err := coap.ParseDgramMessage(data)
		if err != nil {
			log.Debugf("CoAP parse failure: %s", err.Error())
			return nil
		}

		return m
	}
}
