package nmble

import (
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type DiscovererParams struct {
	Bx          *BleXport
	OwnAddrType BleAddrType
	Passive     bool
	Duration    time.Duration
}

// Listens for advertisements; reports the ones that match the specified
// predicate.
type Discoverer struct {
	params    DiscovererParams
	abortChan chan struct{}
}

func NewDiscoverer(params DiscovererParams) *Discoverer {
	return &Discoverer{
		params: params,
	}
}

func (d *Discoverer) scanCancel() error {
	r := NewBleScanCancelReq()

	base := MsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewListener()
	if err := d.params.Bx.Bd.AddListener(base, bl); err != nil {
		return err
	}
	defer d.params.Bx.Bd.RemoveListener(base)

	if err := scanCancel(d.params.Bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (d *Discoverer) Start(advRptCb BleAdvRptFn) error {
	if d.abortChan != nil {
		return nmxutil.NewAlreadyError("Attempt to start BLE discoverer twice")
	}

	// Scanning requires dedicated master privileges.
	if err := d.params.Bx.AcquireMaster(d); err != nil {
		return err
	}
	defer d.params.Bx.ReleaseMaster()

	r := NewBleScanReq()
	r.OwnAddrType = d.params.OwnAddrType
	r.DurationMs = int(d.params.Duration / time.Millisecond)
	r.FilterPolicy = BLE_SCAN_FILT_NO_WL
	r.Limited = false
	r.Passive = d.params.Passive
	r.FilterDuplicates = true

	base := MsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewListener()
	if err := d.params.Bx.Bd.AddListener(base, bl); err != nil {
		return err
	}
	defer d.params.Bx.Bd.RemoveListener(base)

	d.abortChan = make(chan struct{}, 1)
	defer func() { d.abortChan = nil }()

	err := actScan(d.params.Bx, bl, r, d.abortChan, advRptCb)
	if !nmxutil.IsXport(err) {
		// The transport did not restart; always attempt to cancel the scan
		// operation.  In some cases, the host has already stopped scanning
		// and will respond with an "ealready" error that can be ignored.
		if err := d.scanCancel(); err != nil {
			log.Errorf("Failed to cancel scan in progress: %s",
				err.Error())
		}
	}

	return err
}

func (d *Discoverer) Stop() error {
	ch := d.abortChan

	if ch == nil {
		return nmxutil.NewAlreadyError("Attempt to stop inactive discoverer")
	}

	close(ch)
	return nil
}

// Discovers a single device.  After a device is successfully discovered,
// discovery is stopped.
func DiscoverDevice(
	bx *BleXport,
	ownAddrType BleAddrType,
	duration time.Duration,
	advPred BleAdvPredicate) (*BleDev, error) {

	d := NewDiscoverer(DiscovererParams{
		Bx:          bx,
		OwnAddrType: ownAddrType,
		Passive:     false,
		Duration:    duration,
	})

	var dev *BleDev
	advRptCb := func(adv BleAdvReport) {
		if advPred(adv) {
			dev = &adv.Sender
			d.Stop()
		}
	}

	if err := d.Start(advRptCb); err != nil {
		if !nmxutil.IsScanTmo(err) {
			return nil, err
		}
	}

	return dev, nil
}
