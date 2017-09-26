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

package nmble

import (
	"fmt"
	"sync"
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

const syncPollRate = time.Second

type Syncer struct {
	x       *BleXport
	synced  bool
	enabled bool

	stopCh  chan struct{}
	syncCh  chan bool
	resetCh chan int

	wg sync.WaitGroup

	// Protects synced and enabled.
	mtx sync.Mutex
}

func (s *Syncer) Synced() bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.synced
}

func (s *Syncer) setSyncedNoLock(synced bool) {
	if synced == s.synced {
		return
	}

	s.synced = synced
	s.syncCh <- synced
}

func (s *Syncer) setSynced(synced bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.setSyncedNoLock(synced)
}

func (s *Syncer) Refresh() (bool, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if !s.enabled {
		return false, fmt.Errorf(
			"attempt to refresh sync state using disabled syncer")
	}

	synced, err := SyncXact(s.x)
	if err != nil {
		return false, err
	}

	s.setSyncedNoLock(synced)
	return synced, nil
}

func (s *Syncer) addSyncListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_SYNC_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "sync")
	return s.x.AddListener(key)
}

func (s *Syncer) addResetListener() (*Listener, error) {
	key := TchKey(MSG_TYPE_RESET_EVT, -1)
	nmxutil.LogAddListener(3, key, 0, "reset")
	return s.x.AddListener(key)
}

func (s *Syncer) listen() error {
	errChan := make(chan error)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Initial actions can cause an error to be returned.
		syncl, err := s.addSyncListener()
		if err != nil {
			errChan <- err
			close(errChan)
			return
		}
		defer s.x.RemoveListener(syncl)

		resetl, err := s.addResetListener()
		if err != nil {
			errChan <- err
			close(errChan)
			return
		}
		defer s.x.RemoveListener(resetl)

		// Initial actions complete.
		close(errChan)

		for {
			select {
			case <-syncl.ErrChan:
				// XXX
			case bm := <-syncl.MsgChan:
				switch msg := bm.(type) {
				case *BleSyncEvt:
					s.setSynced(msg.Synced)
				}

			case <-resetl.ErrChan:
				// XXX
			case bm := <-resetl.MsgChan:
				switch msg := bm.(type) {
				case *BleResetEvt:
					s.setSynced(false)
					s.resetCh <- msg.Reason
				}

			case <-s.stopCh:
				// It is OK to strand the two listeners.  Their deferred
				// removal will drain them.
				return
			}
		}
	}()

	return <-errChan
}

func (s *Syncer) Start(x *BleXport) (<-chan bool, <-chan int, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.x = x
	s.stopCh = make(chan struct{})
	s.syncCh = make(chan bool)
	s.resetCh = make(chan int)

	s.synced = false

	if err := s.listen(); err != nil {
		return nil, nil, err
	}

	s.enabled = true
	return s.syncCh, s.resetCh, nil
}

func (s *Syncer) Stop() error {
	initiate := func() error {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		if !s.enabled {
			return fmt.Errorf("Syncer already stopped")
		}

		s.enabled = false
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}

	s.synced = false

	close(s.stopCh)

	close(s.syncCh)
	for {
		if _, ok := <-s.syncCh; !ok {
			break
		}
	}

	close(s.resetCh)
	for {
		if _, ok := <-s.resetCh; !ok {
			break
		}
	}

	s.wg.Wait()

	return nil
}
