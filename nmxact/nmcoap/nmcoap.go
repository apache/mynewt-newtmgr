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
)

type MsgFilter func(msg coap.Message) (coap.Message, error)

var messageIdMtx sync.Mutex
var nextMessageId uint16

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

func CreateGet(isTcp bool, resUri string, observe int, token []byte) (coap.Message, error) {
	var q []string

	if err := validateToken(token); err != nil {
		return nil, err
	}

	p := coap.MessageParams{
		Type:  coap.Confirmable,
		Code:  coap.GET,
		Token: token,
	}

	m := buildMessage(isTcp, p)

	q = strings.SplitN(resUri, "?", 2)
	m.SetPathString(q[0])
	if len(q) > 1 {
		m.SetURIQuery(q[1])
	}

	if observe >= 0 {
		m.SetObserve(observe)
	}

	return m, nil
}

func CreatePut(isTcp bool, resUri string, token []byte,
	val []byte) (coap.Message, error) {

	if err := validateToken(token); err != nil {
		return nil, err
	}

	p := coap.MessageParams{
		Type:    coap.Confirmable,
		Code:    coap.PUT,
		Token:   token,
		Payload: val,
	}

	m := buildMessage(isTcp, p)
	m.SetPathString(resUri)

	return m, nil
}

func CreatePost(isTcp bool, resUri string, token []byte,
	val []byte) (coap.Message, error) {

	if err := validateToken(token); err != nil {
		return nil, err
	}

	p := coap.MessageParams{
		Type:    coap.Confirmable,
		Code:    coap.POST,
		Token:   token,
		Payload: val,
	}

	m := buildMessage(isTcp, p)
	m.SetPathString(resUri)

	return m, nil
}

func CreateDelete(isTcp bool, resUri string, token []byte,
	val []byte) (coap.Message, error) {

	if err := validateToken(token); err != nil {
		return nil, err
	}

	p := coap.MessageParams{
		Type:    coap.Confirmable,
		Code:    coap.DELETE,
		Token:   token,
		Payload: val,
	}

	m := buildMessage(isTcp, p)
	m.SetPathString(resUri)

	return m, nil
}
