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
	"mynewt.apache.org/newtmgr/nmxact/adv"
	"mynewt.apache.org/newtmgr/nmxact/nmble"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
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
	nmxutil.Debug = true
	nmxutil.SetLogLevel(log.DebugLevel)
	//nmxutil.SetLogLevel(log.InfoLevel)

	// Initialize the BLE transport.
	params := nmble.NewXportCfg()
	params.SockPath = "/tmp/blehostd-uds"
	params.BlehostdPath = "blehostd"
	params.DevPath = "/dev/cu.usbmodem142141"

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

	configExitHandler(x, nil)

	advertiser, err := x.BuildAdvertiser()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building BLE advertiser: %s\n",
			err.Error())
		os.Exit(1)
	}

	for {
		closeCh := make(chan struct{})

		ac := adv.NewCfg()
		ac.Ble.AdvFields.Flags = new(uint8)
		*ac.Ble.AdvFields.Flags = 6
		ac.Ble.AdvFields.Name = new(string)
		*ac.Ble.AdvFields.Name = "gwadv"

		ac.Ble.SesnCfg.OnCloseCb = func(s sesn.Sesn, err error) {
			close(closeCh)
		}

		_, err := advertiser.Start(ac)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error starting advertise: %s\n",
				err.Error())
			os.Exit(1)
		}

		<-closeCh
	}
}
