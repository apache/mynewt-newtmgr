package nmble

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"

	"mynewt.apache.org/newt/nmxact/nmxutil"
)

func bhdTimeoutError(rspType MsgType) error {
	str := fmt.Sprintf("Timeout waiting for blehostd to send %s response",
		MsgTypeToString(rspType))

	log.Debug(str)
	return nmxutil.NewXportTimeoutError(str)
}

func statusError(op MsgOp, msgType MsgType, status int) error {
	str := fmt.Sprintf("%s %s indicates error: %s (%d)",
		MsgOpToString(op),
		MsgTypeToString(msgType),
		ErrCodeToString(status),
		status)

	log.Debug(str)
	return nmxutil.NewBleHostError(status, str)
}

// Blocking
func connect(x *BleXport, connChan chan error, r *BleConnectReq) error {
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
func terminate(x *BleXport, bl *BleListener, r *BleTerminateReq) error {
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
				bl.acked = true
				if msg.Status != 0 {
					return statusError(MSG_OP_RSP,
						MSG_TYPE_TERMINATE,
						msg.Status)
				} else {
					return nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return bhdTimeoutError(MSG_TYPE_TERMINATE)
		}
	}
}

func connCancel(x *BleXport, bl *BleListener, r *BleConnCancelReq) error {
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
			case *BleConnCancelRsp:
				bl.acked = true
				if msg.Status != 0 {
					return nmxutil.FmtBleHostError(
						msg.Status,
						"Conn cancel response indicates status=%d",
						msg.Status)
				} else {
					return nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return bhdTimeoutError(MSG_TYPE_TERMINATE)
		}
	}
}

// Blocking.
func discSvcUuid(x *BleXport, bl *BleListener, r *BleDiscSvcUuidReq) (
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
				bl.acked = true
				if msg.Status != 0 {
					return nil, nmxutil.FmtBleHostError(
						msg.Status,
						"DiscSvcUuid response indicates status=%d",
						msg.Status)
				}

			case *BleDiscSvcEvt:
				switch msg.Status {
				case 0:
					svc = &msg.Svc
				case ERR_CODE_EDONE:
					if svc == nil {
						return nil, nmxutil.FmtBleHostError(
							msg.Status,
							"Peer doesn't support required service: %s",
							r.Uuid.String())
					}
					return svc, nil
				default:
					return nil, nmxutil.FmtBleHostError(
						msg.Status,
						"DiscSvc event indicates status=%d",
						msg.Status)
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return nil, bhdTimeoutError(MSG_TYPE_DISC_SVC_UUID)
		}
	}
}

// Blocking.
func discAllChrs(x *BleXport, bl *BleListener, r *BleDiscAllChrsReq) (
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
				bl.acked = true
				if msg.Status != 0 {
					return nil, nmxutil.FmtBleHostError(
						msg.Status,
						"DiscAllChrs response indicates status=%d",
						msg.Status)
				}

			case *BleDiscChrEvt:
				switch msg.Status {
				case 0:
					chrs = append(chrs, &msg.Chr)
				case ERR_CODE_EDONE:
					return chrs, nil
				default:
					return nil, nmxutil.FmtBleHostError(
						msg.Status,
						"DiscChr event indicates status=%d",
						msg.Status)
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return nil, bhdTimeoutError(MSG_TYPE_DISC_ALL_CHRS)
		}
	}
}

// Blocking.
func writeCmd(x *BleXport, bl *BleListener, r *BleWriteCmdReq) error {
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
				bl.acked = true
				if msg.Status != 0 {
					return nmxutil.FmtBleHostError(
						msg.Status,
						"WriteCmd response indicates status=%d",
						msg.Status)
				} else {
					return nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return bhdTimeoutError(MSG_TYPE_WRITE_CMD)
		}
	}
}

// Blocking.
func exchangeMtu(x *BleXport, bl *BleListener, r *BleExchangeMtuReq) (
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
				bl.acked = true
				if msg.Status != 0 {
					return 0, nmxutil.FmtBleHostError(
						msg.Status,
						"ExchangeMtu response indicates status=%d",
						msg.Status)
				}

			case *BleMtuChangeEvt:
				if msg.Status != 0 {
					return 0, nmxutil.FmtBleHostError(
						msg.Status,
						"MtuChange event indicates status=%d",
						msg.Status)
				} else {
					return msg.Mtu, nil
				}

			default:
			}

		case <-bl.AfterTimeout(x.rspTimeout):
			return 0, bhdTimeoutError(MSG_TYPE_EXCHANGE_MTU)
		}
	}
}
