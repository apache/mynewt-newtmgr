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
	"sync"

	"github.com/runtimeco/go-coap"
)

type ResType int

const (
	RES_TYPE_PUBLIC ResType = iota
	RES_TYPE_GW
	RES_TYPE_PRIVATE
)

var messageIdMtx sync.Mutex
var nextMessageId uint16

func NextMessageId() uint16 {
	messageIdMtx.Lock()
	defer messageIdMtx.Unlock()

	id := nextMessageId
	nextMessageId++
	return id
}

func Encode(m coap.Message) ([]byte, error) {
	b, err := m.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("Failed to encode CoAP: %s\n", err.Error())
	}

	return b, nil
}

func CreateGet(isTcp bool, resUri string, token []byte) (coap.Message, error) {
	if len(token) > 8 {
		return nil,
			fmt.Errorf("Invalid token; len=%d, must be < 8", len(token))
	}

	p := coap.MessageParams{
		Type:  coap.Confirmable,
		Code:  coap.GET,
		Token: token,
	}

	var m coap.Message
	if isTcp {
		m = coap.NewTcpMessage(p)
	} else {
		m = coap.NewDgramMessage(p)
	}
	m.SetPathString(resUri)

	return m, nil
}

func EncodeGet(isTcp bool, resUri string, token []byte) ([]byte, error) {
	m, err := CreateGet(isTcp, resUri, token)
	if err != nil {
		return nil, err
	}

	b, err := Encode(m)
	if err != nil {
		return nil, err
	}

	return b, nil
}
