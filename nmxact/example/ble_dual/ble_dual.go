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
	"os/signal"
	"sync"
	"syscall"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xact"
	"mynewt.apache.org/newtmgr/nmxact/xport"
)

func configExitHandler(x xport.Xport) {
	onExit := func() {
		x.Stop()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	go func() {
		for {
			s := <-sigChan
			switch s {
			case os.Interrupt, syscall.SIGTERM:
				onExit()
				os.Exit(0)
			case syscall.SIGQUIT:
				util.PrintStacks()
			}
		}
	}()
}

func sendOne(s sesn.Sesn) {
	// Repeatedly:
	//     * Connect to peer if unconnected.
	//     * Send an echo command to peer.
	//
	// If blehostd crashes or the controller is unplugged, nmxact should
	// recover on the next connect attempt.
	if !s.IsOpen() {
		// Connect to the peer (open the session).
		if err := s.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "error starting BLE session: %s (%+v)\n",
				err.Error(), err)
			return
		}
	}

	// Send an echo command to the peer.
	c := xact.NewEchoCmd()
	c.Payload = fmt.Sprintf("hello %p", s)

	res, err := c.Run(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error executing echo command: %s\n",
			err.Error())
		return
	}

	if res.Status() != 0 {
		fmt.Printf("Peer responded negatively to echo command; status=%d\n",
			res.Status())
	}

	eres := res.(*xact.EchoResult)
	fmt.Printf("Peer echoed back: %s\n", eres.Rsp.Payload)

	s.Close()
}

func main() {
	nmxutil.SetLogLevel(log.InfoLevel)

	// Initialize the BLE transport.
	params := nmble.NewXportCfg()
	params.SockPath = "/tmp/blehostd-uds"
	params.BlehostdPath = "blehostd.elf"
	params.DevPath = "/dev/cu.usbmodem14221"

	x, err := nmble.NewBleXport(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating BLE transport: %s1\n",
			err.Error())
		os.Exit(1)
	}

	// Start the BLE transport.
	if err := x.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting BLE transport: %s1\n",
			err.Error())
		os.Exit(1)
	}
	defer x.Stop()

	configExitHandler(x)

	peerNames := []string{
		"ccollins",
		"ccollins2",
		"ccollins3",
	}

	sesns := []sesn.Sesn{}
	for _, n := range peerNames {
		// Prepare a BLE session:
		//     * Plain NMP (not tunnelled over OIC).
		//     * We use a random address.
		//     * Peer has name "nimble-bleprph".
		sc := sesn.NewSesnCfg()
		sc.MgmtProto = sesn.MGMT_PROTO_NMP
		sc.Ble.OwnAddrType = bledefs.BLE_ADDR_TYPE_RANDOM
		sc.Ble.PeerSpec = sesn.BlePeerSpecName(n)

		s, err := x.BuildSesn(sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating BLE session: %s\n",
				err.Error())
			os.Exit(1)
		}

		sesns = append(sesns, s)
	}

	var wg sync.WaitGroup

	for _, s := range sesns {
		wg.Add(1)
		go func(x sesn.Sesn) {
			for {
				sendOne(x)
			}
		}(s)
	}

	wg.Wait()
}
