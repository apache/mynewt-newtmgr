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

package config

import (
	"testing"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
)

// setTimeout overrides nmutil.Timeout for the duration of a test.
func setTimeout(t *testing.T, secs float64) {
	t.Helper()
	old := nmutil.Timeout
	nmutil.Timeout = secs
	t.Cleanup(func() { nmutil.Timeout = old })
}

func setHciIdx(t *testing.T, idx int) {
	t.Helper()
	old := nmutil.HciIdx
	nmutil.HciIdx = idx
	t.Cleanup(func() { nmutil.HciIdx = old })
}

func setDeviceName(t *testing.T, name string) {
	t.Helper()
	old := nmutil.DeviceName
	nmutil.DeviceName = name
	t.Cleanup(func() { nmutil.DeviceName = old })
}

func setBleWriteRsp(t *testing.T, v bool) {
	t.Helper()
	old := nmutil.BleWriteRsp
	nmutil.BleWriteRsp = v
	t.Cleanup(func() { nmutil.BleWriteRsp = old })
}
