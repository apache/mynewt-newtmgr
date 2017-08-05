package nmble

import (
	"fmt"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newtmgr/nmxact/adv"
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type Advertiser struct {
	bx          *BleXport
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

func NewAdvertiser(bx *BleXport) *Advertiser {
	return &Advertiser{
		bx: bx,
	}
}

func (a *Advertiser) fields(f BleAdvFields) ([]byte, error) {
	r := BleAdvFieldsToReq(f)

	bl, err := a.bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer a.bx.RemoveListener(bl)

	return advFields(a.bx, bl, r)
}

func (a *Advertiser) setAdvData(data []byte) error {
	r := NewBleAdvSetDataReq()
	r.Data = BleBytes{data}

	bl, err := a.bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer a.bx.RemoveListener(bl)

	if err := advSetData(a.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (a *Advertiser) setRspData(data []byte) error {
	r := NewBleAdvRspSetDataReq()
	r.Data = BleBytes{data}

	bl, err := a.bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer a.bx.RemoveListener(bl)

	if err := advRspSetData(a.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (a *Advertiser) advertise(cfg adv.Cfg) (uint16, *Listener, error) {
	r := NewBleAdvStartReq()

	r.OwnAddrType = cfg.Ble.OwnAddrType
	r.DurationMs = 0x7fffffff
	r.ConnMode = cfg.Ble.ConnMode
	r.DiscMode = cfg.Ble.DiscMode
	r.ItvlMin = cfg.Ble.ItvlMin
	r.ItvlMax = cfg.Ble.ItvlMax
	r.ChannelMap = cfg.Ble.ChannelMap
	r.FilterPolicy = cfg.Ble.FilterPolicy
	r.HighDutyCycle = cfg.Ble.HighDutyCycle
	r.PeerAddr = cfg.Ble.PeerAddr

	bl, err := a.bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return 0, nil, err
	}

	connHandle, err := advStart(a.bx, bl, r)
	if err != nil {
		a.bx.RemoveListener(bl)
		if !nmxutil.IsXport(err) {
			// The transport did not restart; always attempt to cancel the
			// advertise operation.  In some cases, the host has already stopped
			// advertising and will respond with an "ealready" error that can be
			// ignored.
			if err := a.stopAdvertising(); err != nil {
				log.Errorf("Failed to cancel advertise in progress: %s",
					err.Error())
			}
		}
		return 0, nil, err
	}

	return connHandle, bl, nil
}

func (a *Advertiser) stopAdvertising() error {
	r := NewBleAdvStopReq()

	bl, err := a.bx.AddListener(SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer a.bx.RemoveListener(bl)

	return advStop(a.bx, bl, r)
}

func (a *Advertiser) buildSesn(cfg adv.Cfg, connHandle uint16, bl *Listener) (
	sesn.Sesn, error) {

	s, err := NewBleSesn(a.bx, cfg.Ble.SesnCfg)
	if err != nil {
		return nil, err
	}

	if err := s.OpenConnected(connHandle, bl); err != nil {
		return nil, err
	}

	return s, nil
}

func (a *Advertiser) Start(cfg adv.Cfg) (sesn.Sesn, error) {
	var advData []byte
	var rspData []byte
	var connHandle uint16
	var bl *Listener
	var err error

	fns := []func() error{
		// Convert advertising fields to data.
		func() error {
			advData, err = a.fields(cfg.Ble.AdvFields)
			return err
		},

		// Set advertising data.
		func() error {
			return a.setAdvData(advData)
		},

		// Convert response fields to data.
		func() error {
			rspData, err = a.fields(cfg.Ble.RspFields)
			return err
		},

		// Set response data.
		func() error {
			return a.setRspData(rspData)
		},

		// Advertise
		func() error {
			connHandle, bl, err = a.advertise(cfg)
			return err
		},
	}

	a.stopChan = make(chan struct{})
	a.stoppedChan = make(chan struct{})

	defer func() {
		a.stopChan = nil
		close(a.stoppedChan)
	}()

	if err := a.bx.AcquireSlave(a); err != nil {
		return nil, err
	}
	defer a.bx.ReleaseSlave()

	for _, fn := range fns {
		// Check for abort before each step.
		select {
		case <-a.stopChan:
			return nil, fmt.Errorf("advertise aborted")
		default:
		}

		if err := fn(); err != nil {
			return nil, err
		}
	}

	return a.buildSesn(cfg, connHandle, bl)
}

func (a *Advertiser) Stop() error {
	stopChan := a.stopChan
	if stopChan == nil {
		return fmt.Errorf("advertiser already stopped")
	}
	close(stopChan)

	a.bx.StopWaitingForSlave(a, fmt.Errorf("advertise aborted"))
	a.stopAdvertising()

	// Block until abort is complete.
	<-a.stoppedChan

	return nil
}
