<!--
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
#  KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
-->

# nmxact

## Summary

nmxact (newtmgr transact) is a Go library which enables remote management of Mynewt devices.  Management is performed via the following supported protocols:

    * Newtmgr protocol (NMP)
    * OIC management protocol (OMP)

nmxact can communicate with Mynewt devices via the following transports:

    * Bluetooth Low Energy (BLE)
    * UART 

## Concepts

_xport.Xport:_ Implements a low-level transport. An Xport instance performs the transmitting and receiving of raw data.

_sesn.Sesn:_ Represents a communication session with a specific peer.  The particulars vary according to protocol and transport. Several Sesn instances can use the same Xport.

_xact.Cmd:_ Represents a high-level command.  Executing a Cmd typically results in the exchange of one or more request response pairs with the target peer. Cmd execution blocks until completion.  Execute a command with the `Run()` member function; cancel a running command from another thread with the `Abort()` member function.

_xact.Result:_ The outcome of executing a Cmd. Retrieve the status code in the form of an NMP error code with the `Status()` member function. Specific implementors of the xact.Result interface typically contain all the management responses received during command execution.

## Examples

nmxact comes with the following simple examples:

* _BLE, plain:_ nmxact/example/ble_plain/ble_plain.go
* _serial, plain:_ nmxact/example/ble_plain/serial_plain.go
