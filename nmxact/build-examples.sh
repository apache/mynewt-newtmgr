#!/bin/sh
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.


read -r -d '' exdirs <<-'EOS'
    ble_loop
    ble_plain
    serial_plain
EOS

rc=0

for d in $exdirs
do
    if ! (cd example/"$d" && go build)
    then
        printf '*** Failed to build %s\n' "$d" >&2
        rc=1
    fi
done

exit "$rc"
