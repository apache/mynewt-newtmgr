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

package main

import (
	"fmt"
	"os"
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmserial"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

func main() {
	// Initialize the serial transport.
	cfg := nmserial.NewXportCfg()
	cfg.DevPath = "/dev/cu.usbserial-A600ANJ1"
	cfg.Baud = 115200
	cfg.ReadTimeout = 3 * time.Second

	x := nmserial.NewSerialXport(cfg)

	// Start the serial transport.
	if err := x.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting serial transport: %s\n",
			err.Error())
		os.Exit(1)
	}
	defer x.Stop()

	// Create and open a session for connected Mynewt device.
	sc := sesn.NewSesnCfg()
	sc.MgmtProto = sesn.MGMT_PROTO_NMP

	s, err := x.BuildSesn(sc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating serial session: %s\n",
			err.Error())
		os.Exit(1)
	}

	if err := s.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting serial session: %s\n",
			err.Error())
		os.Exit(1)
	}
	defer s.Close()

	// Send an echo command to the device.
	c := xact.NewEchoCmd()
	c.Payload = "hello"

	res, err := c.Run(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error executing echo command: %s\n",
			err.Error())
		os.Exit(1)
	}

	if res.Status() != 0 {
		fmt.Printf("Device responded negatively to echo command; status=%d\n",
			res.Status())
	}

	eres := res.(*xact.EchoResult)
	fmt.Printf("Device echoed back: %s\n", eres.Rsp.Payload)
}
