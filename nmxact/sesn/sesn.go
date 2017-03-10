package sesn

import (
	"time"

	"mynewt.apache.org/newt/nmxact/bledefs"
	"mynewt.apache.org/newt/nmxact/nmp"
	"mynewt.apache.org/newt/nmxact/nmxutil"
)

type MgmtProto int

const (
	MGMT_PROTO_NMP MgmtProto = iota
	MGMT_PROTO_OMP
)

type SesnCfgBle struct {
	OwnAddrType  bledefs.BleAddrType
	Peer         bledefs.BleDev
	CloseTimeout time.Duration
}

type SesnCfg struct {
	// Used with all transport types.
	MgmtProto MgmtProto

	// Only used with BLE transports.
	Ble SesnCfgBle
}

func NewSesnCfg() SesnCfg {
	return SesnCfg{
		Ble: SesnCfgBle{
			CloseTimeout: 5 * time.Second,
		},
	}
}

type TxOptions struct {
	Timeout time.Duration
	Tries   int
}

func (opt *TxOptions) AfterTimeout() <-chan time.Time {
	if opt.Timeout == 0 {
		return nil
	} else {
		return time.After(opt.Timeout)
	}
}

// Represents a communication session with a specific peer.  The particulars
// vary according to protocol and transport. Several Sesn instances can use the
// same Xport.
type Sesn interface {
	// Initiates communication with the peer.  For connection-oriented
	// transports, this creates a connection.
	Open() error

	// Ends communication with the peer.  For connection-oriented transports,
	// this closes the connection.
	Close() error

	// Retrieves the maximum data payload for outgoing NMP requests.
	MtuOut() int

	// Retrieves the maximum data payload for incoming NMP responses.
	MtuIn() int

	// Transmits a single NMP message and listens for the response.  Blocking.
	TxNmpOnce(msg *nmp.NmpMsg, opt TxOptions) (nmp.NmpRsp, error)

	// Stops a receive operation in progress.  This must be called from a
	// separate thread, as sesn receive operations are blocking.
	AbortRx(nmpSeq uint8) error
}

func TxNmp(s Sesn, m *nmp.NmpMsg, o TxOptions) (nmp.NmpRsp, error) {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		r, err := s.TxNmpOnce(m, o)
		if err == nil {
			return r, nil
		}

		if (!nmxutil.IsNmpTimeout(err) && !nmxutil.IsXportTimeout(err)) ||
			i >= retries {

			return nil, err
		}
	}
}
