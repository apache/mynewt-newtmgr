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

package nmxutil

import (
	"sync"
)

type Bcaster struct {
	chs [](chan interface{})
	mtx sync.Mutex
}

func (b *Bcaster) Listen() chan interface{} {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	ch := make(chan interface{})
	b.chs = append(b.chs, ch)

	return ch
}

func (b *Bcaster) Send(val interface{}) {
	b.mtx.Lock()
	chs := b.chs
	b.mtx.Unlock()

	for _, ch := range chs {
		ch <- val
		close(ch)
	}
}

func (b *Bcaster) Clear() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.chs = nil
}

func (b *Bcaster) SendAndClear(val interface{}) {
	b.Send(val)
	b.Clear()
}
