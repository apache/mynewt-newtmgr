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

package sesn

import (
	"time"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
)

var DfltTxOptions = TxOptions{
	Timeout: 10 * time.Second,
	Tries:   1,
}

type GetNotifyCb func(path string, code coap.COAPCode, value []byte, token []byte)

type TxOptions struct {
	Timeout time.Duration
	Tries   int
}

func NewTxOptions() TxOptions {
	return DfltTxOptions
}

func (opt *TxOptions) AfterTimeout() <-chan time.Time {
	if opt.Timeout == 0 {
		return nil
	} else {
		return time.After(opt.Timeout)
	}
}

// Represents a communication session with a specific peer.  The particulars
// vary according to protocol and transport. Several Sesn instances can use the
// same Xport.
type Sesn interface {
	////// Public interface:

	// Initiates communication with the peer.  For connection-oriented
	// transports, this creates a connection.
	// Returns:
	//     * nil: success.
	//     * nmxutil.SesnAlreadyOpenError: session already open.
	//     * other error
	Open() error

	// Ends communication with the peer.  For connection-oriented transports,
	// this closes the connection.
	//     * nil: success.
	//     * nmxutil.SesnClosedError: session not open.
	//     * other error
	Close() error

	// Indicates whether the session is currently open.
	IsOpen() bool

	// Retrieves the maximum data payload for incoming data packets.
	MtuIn() int

	// Retrieves the maximum data payload for outgoing data packets.
	MtuOut() int

	MgmtProto() MgmtProto

	// Indicates whether the session uses the TCP form of CoAP.
	CoapIsTcp() bool

	// Stops a receive operation in progress.  This must be called from a
	// separate thread, as sesn receive operations are blocking.
	AbortRx(nmpSeq uint8) error

	// XXX AbortResource(seq uint8) error

	RxAccept() (Sesn, *SesnCfg, error)
	RxCoap(opt TxOptions) (coap.Message, error)

	////// Internal to nmxact:

	// Performs a blocking transmit a single NMP message and listens for the
	// response.
	//     * nil: success.
	//     * nmxutil.SesnClosedError: session not open.
	//     * other error
	TxNmpOnce(m *nmp.NmpMsg, opt TxOptions) (nmp.NmpRsp, error)

	TxCoapOnce(m coap.Message, resType ResourceType,
		opt TxOptions) (coap.COAPCode, []byte, error)

	TxCoapObserve(m coap.Message, resType ResourceType,
		opt TxOptions, NotifCb GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error)

	// Returns a transmit and a receive callback used to manipulate CoAP messages
	Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter)
}
