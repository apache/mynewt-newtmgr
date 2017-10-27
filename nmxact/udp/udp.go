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

package udp

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
)

const MAX_PACKET_SIZE = 2048

func Listen(peerString string, dispatchCb func(data []byte)) (
	*net.UDPConn, *net.UDPAddr, error) {

	addr, err := net.ResolveUDPAddr("udp", peerString)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failure resolving name for UDP session: %s",
				err.Error())
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failed to listen for UDP responses: %s", err.Error())
	}

	go func() {
		data := make([]byte, MAX_PACKET_SIZE)

		for {
			nr, srcAddr, err := conn.ReadFromUDP(data)
			if err != nil {
				// Connection closed or read error.
				return
			}

			log.Debugf("Received message from %v %d", srcAddr, nr)
			dispatchCb(data[0:nr])
		}
	}()

	return conn, addr, nil
}
