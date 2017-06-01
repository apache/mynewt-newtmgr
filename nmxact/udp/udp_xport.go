package udp

import (
	"fmt"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type UdpXport struct {
	started bool
}

func NewUdpXport() *UdpXport {
	return &UdpXport{}
}

func (ux *UdpXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	switch cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return NewUdpPlainSesn(cfg), nil
	case sesn.MGMT_PROTO_OMP:
		return NewUdpOicSesn(cfg), nil
	default:
		return nil, fmt.Errorf(
			"Invalid management protocol: %d; expected NMP or OMP",
			cfg.MgmtProto)
	}
}

func (ux *UdpXport) BuildScanner() (scan.Scanner, error) {
	return nil, fmt.Errorf("Attempt to build UDP scanner")
}

func (ux *UdpXport) Start() error {
	if ux.started {
		return nmxutil.NewXportError("UDP xport started twice")
	}
	ux.started = true
	return nil
}

func (ux *UdpXport) Stop() error {
	if !ux.started {
		return nmxutil.NewXportError("UDP xport stopped twice")
	}
	ux.started = false
	return nil
}

func (ux *UdpXport) Tx(bytes []byte) error {
	return fmt.Errorf("unsupported")
}
