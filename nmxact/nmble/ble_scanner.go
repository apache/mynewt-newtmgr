package nmble

import (
	"encoding/base64"
	"fmt"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/xact"
)

// Implements scan.Scanner.
type BleScanner struct {
	cfg scan.Cfg

	bx           *BleXport
	reportedDevs map[BleDev][]byte
	bos          *BleOicSesn
	od           *omp.OmpDispatcher
	enabled      bool
}

func NewBleScanner(bx *BleXport) *BleScanner {
	return &BleScanner{
		bx:           bx,
		reportedDevs: map[BleDev][]byte{},
	}
}

func (s *BleScanner) scan() (scan.ScanPeer, error) {
	if err := s.bos.Open(); err != nil {
		return scan.ScanPeer{}, err
	}
	defer s.bos.Close()

	// Now we are connected and paired.  Read the peer's hardware ID and report
	// it upstream.

	desc, err := s.bos.ConnInfo()
	if err != nil {
		return scan.ScanPeer{}, err
	}

	c := xact.NewConfigReadCmd()
	c.Name = "id/hwid"

	res, err := c.Run(s.bos)
	if err != nil {
		return scan.ScanPeer{}, err
	}
	if res.Status() != 0 {
		return scan.ScanPeer{},
			fmt.Errorf("failed to read hardware ID; NMP status=%d",
				res.Status())
	}
	cres := res.(*xact.ConfigReadResult)

	rawId, err := base64.StdEncoding.DecodeString(cres.Rsp.Val)
	if err != nil {
		return scan.ScanPeer{},
			fmt.Errorf("failed to decode hardware ID; undecoded=%s",
				cres.Rsp.Val)
	}

	peer := scan.ScanPeer{
		HwId: rawId,
		Opaque: BleDev{
			AddrType: desc.PeerIdAddrType,
			Addr:     desc.PeerIdAddr,
		},
	}

	return peer, nil
}

func (s *BleScanner) Start(cfg scan.Cfg) error {
	if s.enabled {
		return fmt.Errorf("Attempt to start BLE scanner twice")
	}

	// Wrap predicate with logic that discards duplicates.
	innerPred := cfg.SesnCfg.Ble.PeerSpec.ScanPred
	cfg.SesnCfg.Ble.PeerSpec.ScanPred = func(adv BleAdvReport) bool {
		// Filter devices that have already been reported.
		if s.reportedDevs[adv.Sender] != nil {
			return false
		}
		return innerPred(adv)
	}

	session, err := s.bx.BuildSesn(cfg.SesnCfg)
	if err != nil {
		return err
	}

	s.enabled = true
	s.cfg = cfg
	s.bos = session.(*BleOicSesn)

	// Start background scanning.
	go func() {
		for s.enabled {
			p, err := s.scan()
			if err != nil {
				log.Debugf("Scan error: %s", err.Error())
			} else {
				s.reportedDevs[p.Opaque.(BleDev)] = p.HwId
				s.cfg.ScanCb(p)
			}
		}
	}()

	return nil
}

func (s *BleScanner) Stop() error {
	if !s.enabled {
		return fmt.Errorf("Attempt to stop BLE scanner twice")
	}
	s.enabled = false

	s.bos.Close()
	return nil
}
