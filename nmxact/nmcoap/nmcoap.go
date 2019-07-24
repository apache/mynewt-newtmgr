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
	"fmt"
	"strings"
	"sync"

	"github.com/runtimeco/go-coap"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type ObserveCode int

// These observe codes differ from those specified in the CoAP spec.  It is
// done this way so that the default value (0) implies no observe action.
const (
	OBSERVE_NONE ObserveCode = iota
	OBSERVE_START
	OBSERVE_STOP
)

type MsgFilter func(msg coap.Message) (coap.Message, error)

type MsgParams struct {
	Code    coap.COAPCode
	Uri     string
	Observe ObserveCode
	Token   []byte
	Payload []byte
}

var messageIdMtx sync.Mutex
var nextMessageId uint16

var opNameMap = map[coap.COAPCode]string{
	coap.GET:    "GET",
	coap.PUT:    "PUT",
	coap.POST:   "POST",
	coap.DELETE: "DELETE",
}

func ParseOp(op string) (coap.COAPCode, error) {
	for c, name := range opNameMap {
		if strings.ToLower(op) == strings.ToLower(name) {
			return c, nil
		}
	}

	return 0, fmt.Errorf("invalid CoAP op: \"%s\"", op)
}

func (o ObserveCode) Spec() int {
	switch o {
	case OBSERVE_START:
		return 0
	case OBSERVE_STOP:
		return 1
	default:
		return -1
	}
}

func NextMessageId() uint16 {
	messageIdMtx.Lock()
	defer messageIdMtx.Unlock()

	id := nextMessageId
	nextMessageId++
	return id
}

func validateToken(t []byte) error {
	if len(t) > 8 {
		return fmt.Errorf("Invalid token; len=%d, must be <= 8", len(t))
	}

	return nil
}

func buildMessage(isTcp bool, p coap.MessageParams) coap.Message {
	if isTcp {
		return coap.NewTcpMessage(p)
	} else {
		return coap.NewDgramMessage(p)
	}
}

func Encode(m coap.Message) ([]byte, error) {
	b, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("Failed to encode CoAP: %s\n", err.Error())
	}

	return b, nil
}

func CreateMsg(isTcp bool, mp MsgParams) (coap.Message, error) {
	if mp.Token == nil {
		mp.Token = nmxutil.NextToken()
	}

	p := coap.MessageParams{
		Type:    coap.Confirmable,
		Code:    mp.Code,
		Token:   mp.Token,
		Payload: mp.Payload,
	}

	m := buildMessage(isTcp, p)

	q := strings.SplitN(mp.Uri, "?", 2)
	m.SetPathString(q[0])
	if len(q) > 1 {
		m.SetURIQuery(q[1])
	}

	if mp.Observe != OBSERVE_NONE {
		m.SetObserve(mp.Observe.Spec())
	}

	return m, nil
}
