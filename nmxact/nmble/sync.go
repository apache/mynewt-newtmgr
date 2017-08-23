package nmble

import (
	"fmt"
	"sync"
	"time"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

const syncPollRate = time.Second

type Syncer struct {
	x           *BleXport
	stopCh      chan struct{}
	wg          sync.WaitGroup
	synced      bool
	syncBlocker nmxutil.Blocker
	mtx         sync.Mutex

	resetBcaster nmxutil.Bcaster
	syncBcaster  nmxutil.Bcaster
}

func (s *Syncer) Refresh() (bool, error) {
	r := NewSyncReq()
	bl, err := s.x.AddListener(SeqKey(r.Seq))
	if err != nil {
		return false, err
	}
	defer s.x.RemoveListener(bl)

	synced, err := checkSync(s.x, bl, r)
	if err != nil {
		return false, err
	}

	s.setSynced(synced)
	return synced, nil
}

func (s *Syncer) Synced() bool {
	return s.synced
}

func (s *Syncer) checkSyncLoop() {
	doneCh := make(chan struct{})

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		s.BlockUntilSynced(nmxutil.DURATION_FOREVER, s.stopCh)
		close(doneCh)
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			s.Refresh()

			select {
			case <-doneCh:
				return

			case <-s.stopCh:
				return

			case <-time.After(syncPollRate):
			}
		}
	}()
}

func (s *Syncer) setSynced(synced bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if synced == s.synced {
		return
	}

	s.synced = synced
	if s.synced {
		s.syncBlocker.Unblock(nil)
	} else {
		s.syncBlocker.Start()

		// Listen for sync loss and reset in the background.
		s.checkSyncLoop()
	}
	s.syncBcaster.Send(s.synced)
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
					s.resetBcaster.Send(msg.Reason)
				}

			case <-s.stopCh:
				return
			}
		}
	}()

	return <-errChan
}

func (s *Syncer) Start(x *BleXport) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.x = x
	s.stopCh = make(chan struct{})
	s.syncBlocker.Start()
	s.checkSyncLoop()
	return s.listen()
}

func (s *Syncer) Stop() error {
	initiate := func() error {
		s.mtx.Lock()
		defer s.mtx.Unlock()

		if s.stopCh == nil {
			return fmt.Errorf("Syncer already stopped")
		}
		close(s.stopCh)
		return nil
	}

	if err := initiate(); err != nil {
		return err
	}
	s.wg.Wait()

	s.syncBcaster.Clear()
	s.resetBcaster.Clear()
	s.syncBlocker.Unblock(nil)

	s.stopCh = nil

	return nil
}

func (s *Syncer) BlockUntilSynced(timeout time.Duration,
	stopChan <-chan struct{}) error {

	_, err := s.syncBlocker.Wait(timeout, stopChan)
	return err
}

func (s *Syncer) ListenSync() chan interface{} {
	return s.syncBcaster.Listen()
}

func (s *Syncer) ListenReset() chan interface{} {
	return s.resetBcaster.Listen()
}
