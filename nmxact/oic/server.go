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

package oic

import (
	"fmt"

	"github.com/runtimeco/go-coap"
)

type Server struct {
	rxer  Receiver
	rm    ResMgr
	isTcp bool
}

func NewServer(isTcp bool) Server {
	s := Server{
		rxer:  NewReceiver(isTcp),
		rm:    NewResMgr(),
		isTcp: isTcp,
	}

	return s
}

func (s *Server) AddResource(r Resource) error {
	return s.rm.Add(r)
}

// @return                      Response to send back, if any.
func (s *Server) Rx(data []byte) (coap.Message, error) {
	m := s.rxer.Rx(data)
	if m == nil {
		return nil, nil
	}

	var typ coap.COAPType
	switch m.Type() {
	case coap.Confirmable:
		typ = coap.Acknowledgement

	case coap.NonConfirmable:
		typ = coap.NonConfirmable

	default:
		return nil, fmt.Errorf("Don't know how to handle CoAP message with "+
			"type=%d (%s)", m.Type(), m.Type().String())
	}

	code, payload := s.rm.Access(m)

	p := coap.MessageParams{
		Type:      typ,
		Code:      code,
		MessageID: NextMessageId(),
		Token:     m.Token(),
		Payload:   payload,
	}

	if !s.isTcp {
		return coap.NewDgramMessage(p), nil
	} else {
		return coap.NewTcpMessage(p), nil
	}
}
