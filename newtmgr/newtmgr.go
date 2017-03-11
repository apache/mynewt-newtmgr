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

	"mynewt.apache.org/newt/newtmgr/cli"
	"mynewt.apache.org/newt/newtmgr/config"
	"mynewt.apache.org/newt/nmxact/nmserial"
	"mynewt.apache.org/newt/util"
)

func main() {
	if err := config.InitGlobalConnProfileMgr(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	onExit := func() {
		s, err := cli.GetSesnIfOpen()
		if err == nil {
			s.Close()
		}

		x, err := cli.GetXportIfOpen()
		if err == nil {
			// Don't attempt to close a serial transport.  Attempting to close
			// the serial port while a read is in progress (in MacOS) just
			// blocks until the read completes.  Instead, let the OS close the
			// port on termination.
			if _, ok := x.(*nmserial.SerialXport); !ok {
				x.Stop()
			}
		}
	}
	defer onExit()
	cli.NmSetOnExit(onExit)

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

	cli.Commands().Execute()
}
