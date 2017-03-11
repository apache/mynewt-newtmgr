package nmble

import (
	"encoding/json"

	"mynewt.apache.org/newt/nmxact/xport"
)

// Blocking
func connect(x xport.Xport, connChan chan error, r *BleConnectReq) error {
	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	err = <-connChan
	if err != nil {
		return err
	}

	return nil
}

// Blocking
func terminate(x xport.Xport, bl *BleListener, r *BleTerminateReq) error {
	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleTerminateRsp:
				if msg.Status != 0 {
					return FmtBleHostError(
						msg.Status,
						"Terminate response indicates status=%d",
						msg.Status)
				} else {
					return nil
				}

			default:
			}
		}
	}
}

// Blocking.
func discSvcUuid(x xport.Xport, bl *BleListener, r *BleDiscSvcUuidReq) (
	*BleSvc, error) {

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	var svc *BleSvc
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleDiscSvcUuidRsp:
				if msg.Status != 0 {
					return nil, FmtBleHostError(
						msg.Status,
						"DiscSvcUuid response indicates status=%d",
						msg.Status)
				}

			case *BleDiscSvcEvt:
				switch msg.Status {
				case 0:
					svc = &msg.Svc
				case BLE_HS_EDONE:
					if svc == nil {
						return nil, FmtBleHostError(
							msg.Status,
							"Peer doesn't support required service: %s",
							r.Uuid.String())
					}
					return svc, nil
				default:
					return nil, FmtBleHostError(
						msg.Status,
						"DiscSvc event indicates status=%d",
						msg.Status)
				}

			default:
			}
		}
	}
}

// Blocking.
func discAllChrs(x xport.Xport, bl *BleListener, r *BleDiscAllChrsReq) (
	[]*BleChr, error) {

	j, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := x.Tx(j); err != nil {
		return nil, err
	}

	chrs := []*BleChr{}
	for {
		select {
		case err := <-bl.ErrChan:
			return nil, err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleDiscAllChrsRsp:
				if msg.Status != 0 {
					return nil, FmtBleHostError(
						msg.Status,
						"DiscAllChrs response indicates status=%d",
						msg.Status)
				}

			case *BleDiscChrEvt:
				switch msg.Status {
				case 0:
					chrs = append(chrs, &msg.Chr)
				case BLE_HS_EDONE:
					return chrs, nil
				default:
					return nil, FmtBleHostError(
						msg.Status,
						"DiscChr event indicates status=%d",
						msg.Status)
				}

			default:
			}
		}
	}
}

// Blocking.
func writeCmd(x xport.Xport, bl *BleListener, r *BleWriteCmdReq) error {
	j, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err := x.Tx(j); err != nil {
		return err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleWriteCmdRsp:
				if msg.Status != 0 {
					return FmtBleHostError(
						msg.Status,
						"WriteCmd response indicates status=%d",
						msg.Status)
				} else {
					return nil
				}

			default:
			}
		}
	}
}

// Blocking.
func exchangeMtu(x xport.Xport, bl *BleListener, r *BleExchangeMtuReq) (
	int, error) {

	j, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}

	if err := x.Tx(j); err != nil {
		return 0, err
	}

	for {
		select {
		case err := <-bl.ErrChan:
			return 0, err
		case bm := <-bl.BleChan:
			switch msg := bm.(type) {
			case *BleExchangeMtuRsp:
				if msg.Status != 0 {
					return 0, FmtBleHostError(
						msg.Status,
						"ExchangeMtu response indicates status=%d",
						msg.Status)
				}

			case *BleMtuChangeEvt:
				if msg.Status != 0 {
					return 0, FmtBleHostError(
						msg.Status,
						"MtuChange event indicates status=%d",
						msg.Status)
				} else {
					return msg.Mtu, nil
				}

			default:
			}
		}
	}
}
