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

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type masterState int

const (
	MASTER_STATE_IDLE masterState = iota
	MASTER_STATE_SECONDARY
	MASTER_STATE_PRIMARY
	MASTER_STATE_PRIMARY_SECONDARY_PENDING
)

func (s masterState) String() string {
	m := map[masterState]string{
		MASTER_STATE_IDLE:                      "idle",
		MASTER_STATE_SECONDARY:                 "secondary",
		MASTER_STATE_PRIMARY:                   "primary",
		MASTER_STATE_PRIMARY_SECONDARY_PENDING: "primary_secondary_pending",
	}

	return m[s]
}

type primary struct {
	token interface{}
	ch    chan error
}

type Preemptable interface {
	Preempt() error
}

// Represents the Bluetooth device's "master privileges."  The device can only
// do one of the following actions at a time:
// * initiate connection
// * scan
//
// Clients are divided into two groups:
// * primary
// * secondary
//
// This struct restricts master privileges to a single client at a time.  It
// uses the following procedure to determine which of several clients to serve:
//     If there is one or more waiting primaries:
//         If a secondary is active, preempt it.
//         Service the primaries in the order of their requests.
//     Else (no waiting primaries):
//         Service waiting secondary if there is one.
type Master struct {
	primaries        []primary
	secondary        Preemptable
	state            masterState
	secondaryReadyCh chan error

	mtx sync.Mutex
}

func NewMaster(x *BleXport) Master {
	return Master{
		secondaryReadyCh: make(chan error),
	}
}

func (m *Master) GetSecondary() Preemptable {
	return m.secondary
}

func (m *Master) SetSecondary(s Preemptable) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state == MASTER_STATE_SECONDARY ||
		m.state == MASTER_STATE_PRIMARY_SECONDARY_PENDING {

		return fmt.Errorf("cannot replace master secondary while it is in use")
	}

	m.secondary = s
	return nil
}

func (m *Master) setState(s masterState) {
	log.Debugf("Master state change: %s --> %s", m.state, s)
	m.state = s
}

func (m *Master) appendPrimary(token interface{}) primary {
	c := primary{
		token: token,
		ch:    make(chan error),
	}
	m.primaries = append(m.primaries, c)

	return c
}

func (m *Master) AcquirePrimary(token interface{}) error {
	initiate := func() (*primary, error) {
		m.mtx.Lock()
		defer m.mtx.Unlock()

		switch m.state {
		case MASTER_STATE_IDLE:
			m.setState(MASTER_STATE_PRIMARY)
			return nil, nil

		case MASTER_STATE_SECONDARY:
			c := m.appendPrimary(token)
			go m.secondary.Preempt()
			return &c, nil

		case MASTER_STATE_PRIMARY, MASTER_STATE_PRIMARY_SECONDARY_PENDING:
			c := m.appendPrimary(token)
			return &c, nil

		default:
			nmxutil.Assert(false)
			return nil, fmt.Errorf("internal error: invalid master state=%+v",
				m.state)
		}
	}

	c, err := initiate()
	if err != nil {
		return err
	}

	if c == nil {
		return nil
	}

	return <-c.ch
}

func (m *Master) AcquireSecondary() error {
	initiate := func() (chan error, error) {
		m.mtx.Lock()
		defer m.mtx.Unlock()

		switch m.state {
		case MASTER_STATE_IDLE:
			// The resource is unused; just acquire it.
			m.setState(MASTER_STATE_SECONDARY)
			return nil, nil

		case MASTER_STATE_SECONDARY, MASTER_STATE_PRIMARY_SECONDARY_PENDING:
			nmxutil.Assert(false)
			return nil, fmt.Errorf("Attempt to perform more than one " +
				"secondary master procedure")

		case MASTER_STATE_PRIMARY:
			m.setState(MASTER_STATE_PRIMARY_SECONDARY_PENDING)
			return m.secondaryReadyCh, nil

		default:
			nmxutil.Assert(false)
			return nil, fmt.Errorf(
				"internal error: invalid master state=%+v", m.state)
		}
	}

	ch, err := initiate()
	if err != nil {
		return err
	}

	if ch == nil {
		return nil
	}

	return <-ch
}

func (m *Master) serviceSecondary(err error) {
	m.secondaryReadyCh <- err
}

func (m *Master) servicePrimary(err error) {
	nmxutil.Assert(len(m.primaries) > 0)

	next := m.primaries[0]
	m.primaries = m.primaries[1:]

	next.ch <- err
}

func (m *Master) Release() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	switch m.state {
	case MASTER_STATE_IDLE:
		nmxutil.Assert(false)

	case MASTER_STATE_SECONDARY:
		if len(m.primaries) == 0 {
			m.setState(MASTER_STATE_IDLE)
		} else {
			m.setState(MASTER_STATE_PRIMARY)
			m.servicePrimary(nil)
		}

	case MASTER_STATE_PRIMARY:
		if len(m.primaries) == 0 {
			m.setState(MASTER_STATE_IDLE)
		} else {
			m.servicePrimary(nil)
		}

	case MASTER_STATE_PRIMARY_SECONDARY_PENDING:
		if len(m.primaries) == 0 {
			m.setState(MASTER_STATE_SECONDARY)
			m.serviceSecondary(nil)
		} else {
			m.servicePrimary(nil)
		}

	default:
		nmxutil.Assert(false)
	}
}

func (m *Master) findPrimaryIdx(token interface{}) int {
	for i, c := range m.primaries {
		if c.token == token {
			return i
		}
	}

	return -1
}

// Removes the specified primary from the wait queue.
func (m *Master) StopWaitingPrimary(token interface{}, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	idx := m.findPrimaryIdx(token)
	if idx == -1 {
		return
	}

	m.primaries = append(
		m.primaries[0:idx], m.primaries[idx+1:len(m.primaries)]...)
}

// Removes the specified secondary from the wait queue.
func (m *Master) StopWaitingSecondary(err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state == MASTER_STATE_PRIMARY_SECONDARY_PENDING {
		m.setState(MASTER_STATE_PRIMARY)
		m.serviceSecondary(fmt.Errorf("secondary aborted master acquisition"))
	}
}

// Releases the resource and clears the wait queue.
func (m *Master) Abort(err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	// Set state such that there are no waiters.  We can't clear the current
	// owner of the master; it must release on its own.
	switch m.state {
	case MASTER_STATE_IDLE:
	case MASTER_STATE_SECONDARY:
		go m.secondary.Preempt()
	case MASTER_STATE_PRIMARY:
	case MASTER_STATE_PRIMARY_SECONDARY_PENDING:
		m.serviceSecondary(err)
		m.setState(MASTER_STATE_PRIMARY)
	}

	for len(m.primaries) > 0 {
		m.servicePrimary(err)
	}
}
