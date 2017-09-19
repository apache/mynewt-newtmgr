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

func (b *Bcaster) clearNoLock() {
	for _, ch := range b.chs {
		close(ch)
	}

	b.chs = nil
}

func (b *Bcaster) copyChansNoLock() []chan interface{} {
	chans := make([]chan interface{}, len(b.chs))
	for i, _ := range b.chs {
		chans[i] = b.chs[i]
	}

	return chans
}

func (b *Bcaster) sendNoLock(val interface{}) {
	for _, ch := range b.copyChansNoLock() {
		ch <- val
	}
}

func (b *Bcaster) Listen(depth int) chan interface{} {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	ch := make(chan interface{}, depth)
	b.chs = append(b.chs, ch)

	return ch
}

func (b *Bcaster) copyChans() []chan interface{} {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	return b.copyChansNoLock()
}

func (b *Bcaster) Send(val interface{}) {
	for _, ch := range b.copyChans() {
		ch <- val
	}
}

func (b *Bcaster) StopListening(ch chan interface{}) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	for i, c := range b.chs {
		if c == ch {
			close(c)

			b.chs[i] = b.chs[len(b.chs)-1]
			b.chs = b.chs[:len(b.chs)-1]
			break
		}
	}
}

func (b *Bcaster) Clear() {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.clearNoLock()
}

func (b *Bcaster) SendAndClear(val interface{}) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	b.sendNoLock(val)
	b.clearNoLock()
}
