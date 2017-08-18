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
	if m.scanWait == nil {
		return false
	}

	m.scanWait <- err
	close(m.scanWait)
	m.scanWait = nil
	return true
}

func (m *Master) AcquireConnect(token interface{}) error {
	// Stop the scanner in case it is active; connections take priority.
	m.scanner.Preempt()

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.res.Acquire(token)
}

func (m *Master) AcquireScan(token interface{}) error {
	m.mtx.Lock()

	// If the resource is unused, just acquire it.
	if !m.res.Acquired() {
		err := m.res.Acquire(token)
		m.mtx.Unlock()
		return err
	}

	// Otherwise, wait until no one wants to connect.
	if m.scanWait != nil {
		m.mtx.Unlock()
		return fmt.Errorf("Scanner already waiting for master privileges")
	}
	m.scanWait = make(chan error)

	m.mtx.Unlock()

	// Now we have to wait until someone releases the resource.  When this
	// happens, let the releaser know when the scanner has finished acquiring
	// the resource.  At that time, the call to Release() can unlock and
	// return.
	defer func() { m.scanAcq <- struct{}{} }()

	// Wait for the resource to be released.
	if err := <-m.scanWait; err != nil {
		return err
	}

	return m.res.Acquire(token)
}

func (m *Master) Release() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

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
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.unblockScanner(err)
}

// Releases the resource and clears the wait queue.
func (m *Master) Abort(err error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.unblockScanner(err)
	m.res.Abort(err)
}
