package bll

import (
	"fmt"

	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/examples/lib/dev"

	"mynewt.apache.org/newtmgr/nmxact/scan"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type XportCfg struct {
	CtlrName string
}

func NewXportCfg() XportCfg {
	return XportCfg{
		CtlrName: "default",
	}
}

type BllXport struct {
	cfg XportCfg
}

func NewBllXport(cfg XportCfg) *BllXport {
	return &BllXport{
		cfg: cfg,
	}
}

func (bx *BllXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return nil, fmt.Errorf("BllXport.BuildSesn() not supported; " +
		"use BllXport.BuildBllSesn instead")
}

func (bx *BllXport) BuildBllSesn(cfg BllSesnCfg) (sesn.Sesn, error) {
	switch cfg.MgmtProto {
	case sesn.MGMT_PROTO_NMP:
		return NewBllPlainSesn(cfg), nil
	case sesn.MGMT_PROTO_OMP:
		return NewBllOicSesn(cfg), nil
	default:
		return nil, fmt.Errorf(
			"Invalid management protocol: %d; expected NMP or OMP",
			cfg.MgmtProto)
	}
}

func (bx *BllXport) Start() error {
	d, err := dev.NewDevice(bx.cfg.CtlrName)
	if err != nil {
		return err
	}

	ble.SetDefaultDevice(d)

	return nil
}

func (bx *BllXport) Stop() error {
	if err := ble.Stop(); err != nil {
		return err
	}

	return nil
}

func (bx *BllXport) BuildScanner() (scan.Scanner, error) {
	return nil, fmt.Errorf("BllXport.BuildScanner() not supported")
}

func (bx *BllXport) Tx(data []byte) error {
	return fmt.Errorf("BllXport.Tx() not supported")
}
