package nmble

import (
	"fmt"
	"sync"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

// Represents the Bluetooth device's "master privileges."  The device can only
// do one of the following actions at a time:
// * initiate connection
// * scan
//
// ("connector": client who wants to connect)
// ("scanner": client who wants to scan)
//
// This struct restricts master privileges to a single client at a time.  It
// uses the following procedure to determine which of several clients to serve:
//     If there is one or more waiting connectors:
//         If a scanner is active, suspend it.
//         Service the connectors in the order of their requests.
//     Else (no waiting connectors):
//         Service waiting scanner if there is one.
type Master struct {
	res      nmxutil.SingleResource
	scanner  *BleScanner
	scanWait chan error
	scanAcq  chan struct{}
	mtx      sync.Mutex
}

func NewMaster(x *BleXport, s *BleScanner) Master {
	return Master{
		res:     nmxutil.NewSingleResource(),
		scanner: s,
		scanAcq: make(chan struct{}),
	}
}

// Unblocks a waiting scanner.
func (m *Master) unblockScanner(err error) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.scanWait == nil {
		return false
	}

	m.scanWait <- err
	close(m.scanWait)
	m.scanWait = nil
	return true
}

func (m *Master) AcquireConnect(token interface{}) error {
	m.mtx.Lock()

	// Append the connector to the wait queue.
	ch := m.res.Acquire(token)

	m.mtx.Unlock()

	// Stop the scanner in case it is active; connections take priority.  We do
	// this in a Goroutine so that this call doesn't block indefinitely.  We
	// need to be reading from the acquisition channel when the scanner frees
	// the resource.
	go func() {
		m.scanner.Preempt()
	}()

	// Unblocks when either:
	// 1. Master resource becomes available.
	// 2. Waiter aborts via call to StopWaitingConnect().
	return <-ch
}

func (m *Master) AcquireScan(token interface{}) error {
	// Gets an acquisition channel in a thread-safe manner.
	// @return chan             Acquisition channel if scanner must wait for
	//                              resource;
	//                          Nil if scanner was able to acquire resource.
	//         error            Error.
	getChan := func() (<-chan error, error) {
		m.mtx.Lock()
		defer m.mtx.Unlock()

		// If the resource is unused, just acquire it.
		if !m.res.Acquired() {
			<-m.res.Acquire(token)
			return nil, nil
		} else {
			// Otherwise, wait until there are no waiting connectors.
			if m.scanWait != nil {
				return nil,
					fmt.Errorf("Scanner already waiting for master resource")
			}
			m.scanWait = make(chan error)
			return m.scanWait, nil
		}
	}

	ch, err := getChan()
	if err != nil {
		return err
	}
	if ch == nil {
		// Resource acquired.
		return nil
	}

	// Otherwise, we have to wait until someone releases the resource.  When
	// this happens, let the releaser know when the scanner has finished
	// acquiring the resource.  At that time, the call to Release() can unlock
	// and return.
	defer func() { m.scanAcq <- struct{}{} }()

	// Wait for the resource to be released.
	if err := <-ch; err != nil {
		return err
	}

	// Grab the resource; shouldn't block. */
	return <-m.res.Acquire(token)
}

func (m *Master) Release() {
	if m.res.Release() {
		// Next waiting connector acquired the resource.
		return
	}

	// No pending connects; hand resource to scanner if it wants it.
	if m.unblockScanner(nil) {
		// Don't return until scanner has fully acquired the resource.
		<-m.scanAcq
	}
}

// Removes the specified connector from the wait queue.
func (m *Master) StopWaitingConnect(token interface{}, err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.res.StopWaiting(token, err)
}

// Removes the specified scanner from the wait queue.
func (m *Master) StopWaitingScan(token interface{}, err error) {
	m.unblockScanner(err)
}

// Releases the resource and clears the wait queue.
func (m *Master) Abort(err error) {
	m.unblockScanner(err)
	m.res.Abort(err)
}
