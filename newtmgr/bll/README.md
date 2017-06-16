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

# Bll (BLE library)

This package implements a sesn and xport suitable for speaking NMP over BLE.  The implementation uses the currantlabs BLE library (https://github.com/currantlabs/ble).  This contrasts with the nmxact "nmble" package which is implemented using blehostd, a NimBLE host running in a simulated OS.

This package was created to address the following complications associated with the nmble package:

    * Inability to use the built-in Bluetooth controller in MacOS.
    * Hassle of setting up and configuring blehostd.
