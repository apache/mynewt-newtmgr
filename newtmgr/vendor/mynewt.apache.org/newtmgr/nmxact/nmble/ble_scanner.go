package nmble

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

// Implements scan.Scanner.
type BleScanner struct {
	cfg scan.Cfg

	bx           *BleXport
	discoverer   *Discoverer
	reportedDevs map[BleDev]string
	bos          *BleOicSesn
	enabled      bool

	// Protects accesses to the reported devices map.
	mtx sync.Mutex
}

func NewBleScanner(bx *BleXport) *BleScanner {
	return &BleScanner{
		bx:           bx,
		reportedDevs: map[BleDev]string{},
	}
}

func (s *BleScanner) discover() (*BleDev, error) {
	s.mtx.Lock()
	s.discoverer = NewDiscoverer(DiscovererParams{
		Bx:          s.bx,
		OwnAddrType: s.cfg.SesnCfg.Ble.OwnAddrType,
		Passive:     false,
		Duration:    15 * time.Second,
	})
	s.mtx.Unlock()

	defer func() {
		s.mtx.Lock()
		defer s.mtx.Unlock()
		s.discoverer = nil
	}()

	var dev *BleDev
	advRptCb := func(r BleAdvReport) {
		if s.cfg.Ble.ScanPred(r) {
			dev = &r.Sender
			s.discoverer.Stop()
		}
	}
	if err := s.discoverer.Start(advRptCb); err != nil {
		return nil, err
	}

	return dev, nil
}

func (s *BleScanner) connect(dev BleDev) error {
	s.cfg.SesnCfg.PeerSpec.Ble = dev
	session, err := s.bx.BuildSesn(s.cfg.SesnCfg)
	if err != nil {
		return err
	}

	s.mtx.Lock()
	s.bos = session.(*BleOicSesn)
	s.mtx.Unlock()

	if err := s.bos.Open(); err != nil {
		return err
	}

	return nil
}

func (s *BleScanner) readHwId() (string, error) {
	rsp, err := sesn.GetResource(s.bos, "/mynewt/hwid", sesn.NewTxOptions())
	if err != nil {
		return "", err
	}

	m, err := nmxutil.DecodeCborMap(rsp)
	if err != nil {
		return "", err
	}

	hwid, ok := m["hwid"].(string)
	if !ok {
		return "", fmt.Errorf("device reports invalid hwid: %#v", hwid)
	}

	return hwid, nil
}

func (s *BleScanner) scan() (*scan.ScanPeer, error) {
	// Discover the first device which matches the specified predicate.
	dev, err := s.discover()
	if err != nil {
		return nil, err
	}
	if dev == nil {
		return nil, nil
	}

	s.connect(*dev)
	defer s.bos.Close()

	// Now we are connected (and paired if required).  Read the peer's hardware
	// ID and report it upstream.
	hwId, err := s.readHwId()
	if err != nil {
		return nil, err
	}

	desc, err := s.bos.ConnInfo()
	if err != nil {
		return nil, err
	}

	peer := scan.ScanPeer{
		HwId: hwId,
		PeerSpec: sesn.PeerSpec{
			Ble: BleDev{
				AddrType: desc.PeerIdAddrType,
				Addr:     desc.PeerIdAddr,
			},
		},
	}

	return &peer, nil
}

func (s *BleScanner) Start(cfg scan.Cfg) error {
	if s.enabled {
		return nmxutil.NewAlreadyError("Attempt to start BLE scanner twice")
	}

	// Wrap predicate with logic that discards duplicates.
	innerPred := cfg.Ble.ScanPred
	cfg.Ble.ScanPred = func(adv BleAdvReport) bool {
		// Filter devices that have already been reported.
		s.mtx.Lock()
		seen := s.reportedDevs[adv.Sender] != ""
		s.mtx.Unlock()

		if seen {
			return false
		} else {
			return innerPred(adv)
		}
	}

	s.enabled = true
	s.cfg = cfg

	// Start background scanning.
	go func() {
		for s.enabled {
			p, err := s.scan()
			if err != nil {
				log.Debugf("Scan error: %s", err.Error())
				if nmxutil.IsXport(err) {
					// Transport stopped; abort the scan.
					s.enabled = false
				}
			} else if p != nil {
				s.mtx.Lock()
				s.reportedDevs[p.PeerSpec.Ble] = p.HwId
				s.mtx.Unlock()

				s.cfg.ScanCb(*p)
			}
		}
	}()

	return nil
}

func (s *BleScanner) Stop() error {
	if !s.enabled {
		return nmxutil.NewAlreadyError("Attempt to stop BLE scanner twice")
	}
	s.enabled = false

	s.mtx.Lock()

	discoverer := s.discoverer
	s.discoverer = nil

	bos := s.bos
	s.bos = nil

	s.mtx.Unlock()

	if discoverer != nil {
		discoverer.Stop()
	}

	if bos != nil {
		bos.Close()
	}

	return nil
}

// @return                      true if the specified device was found and
//                                  forgetten;
//                              false if the specified device is unknown.
func (s *BleScanner) ForgetDevice(hwId string) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for discoverer, h := range s.reportedDevs {
		if h == hwId {
			delete(s.reportedDevs, discoverer)
			return true
		}
	}

	return false
}

func (s *BleScanner) ForgetAllDevices() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for discoverer, _ := range s.reportedDevs {
		delete(s.reportedDevs, discoverer)
	}
}
