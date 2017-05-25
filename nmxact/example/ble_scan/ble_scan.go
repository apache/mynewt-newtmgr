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
	"syscall"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"mynewt.apache.org/newtmgr/nmxact/xport"
)

func configExitHandler(x xport.Xport, s sesn.Sesn) {
	onExit := func() {
		if s != nil && s.IsOpen() {
			s.Close()
		}

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

func main() {
	//nmxutil.SetLogLevel(log.DebugLevel)
	nmxutil.SetLogLevel(log.InfoLevel)

	// Initialize the BLE transport.
	params := nmble.NewXportCfg()
	params.SockPath = "/tmp/blehostd-uds"
	params.BlehostdPath = "blehostd.elf"
	params.DevPath = "/dev/cu.usbmodem14221"

	x, err := nmble.NewBleXport(params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating BLE transport: %s\n",
			err.Error())
		os.Exit(1)
	}

	// Start the BLE transport.
	if err := x.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting BLE transport: %s\n",
			err.Error())
		os.Exit(1)
	}
	defer x.Stop()

	scanChan := make(chan scan.ScanPeer)
	scanCb := func(peer scan.ScanPeer) {
		fmt.Printf("discovered peer: %#v\n", peer)
		scanChan <- peer
	}

	configExitHandler(x, nil)

	scanner, err := x.BuildScanner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building BLE scanner: %s\n",
			err.Error())
		os.Exit(1)
	}

	for {
		sc := scan.BleOmpScanCfg(scanCb)
		sc.Ble.ScanPred = func(adv bledefs.BleAdvReport) bool {
			return adv.Name == "c5"
		}
		if err := scanner.Start(sc); err != nil {
			fmt.Fprintf(os.Stderr, "error starting scan: %s\n", err.Error())
			os.Exit(1)
		}

		p := <-scanChan

		// Found a peer; stop scanning.
		if err := scanner.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to stop scan: %s\n", err.Error())
			os.Exit(1)
		}

		fmt.Printf("Connecting to %#v\n", p)
		c := sesn.NewSesnCfg()
		c.MgmtProto = sesn.MGMT_PROTO_OMP
		c.Ble.OwnAddrType = bledefs.BLE_ADDR_TYPE_RANDOM
		c.PeerSpec = p.PeerSpec

		s, err := x.BuildSesn(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating BLE session: %s\n",
				err.Error())
			os.Exit(1)
		}

		if err := s.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "error opening BLE session: %s\n",
				err.Error())
			os.Exit(1)
		}

		fmt.Printf("Connected\n")
		fmt.Printf("Closing\n")
		s.Close()
	}
}
