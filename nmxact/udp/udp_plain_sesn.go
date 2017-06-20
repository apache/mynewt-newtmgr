package udp

import (
	"fmt"
	"net"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type UdpPlainSesn struct {
	cfg  sesn.SesnCfg
	addr *net.UDPAddr
	conn *net.UDPConn
	d    *nmp.Dispatcher
}

func NewUdpPlainSesn(cfg sesn.SesnCfg) *UdpPlainSesn {
	ups := &UdpPlainSesn{
		cfg: cfg,
		d:   nmp.NewDispatcher(1),
	}

	return ups
}

func (ups *UdpPlainSesn) Open() error {
	if ups.conn != nil {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open UDP session")
	}

	conn, addr, err := Listen(ups.cfg.PeerSpec.Udp,
		func(data []byte) {
			ups.d.Dispatch(data)
		})
	if err != nil {
		return err
	}

	ups.addr = addr
	ups.conn = conn
	return nil
}

func (ups *UdpPlainSesn) Close() error {
	if ups.conn != nil {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened UDP session")
	}

	ups.conn.Close()
	ups.conn = nil
	ups.addr = nil
	return nil
}

func (ups *UdpPlainSesn) IsOpen() bool {
	return ups.conn != nil
}

func (ups *UdpPlainSesn) MtuIn() int {
	return MAX_PACKET_SIZE - nmp.NMP_HDR_SIZE
}

func (ups *UdpPlainSesn) MtuOut() int {
	return MAX_PACKET_SIZE - nmp.NMP_HDR_SIZE
}

func (ups *UdpPlainSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (ups *UdpPlainSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !ups.IsOpen() {
		return nil, fmt.Errorf("Attempt to transmit over closed UDP session")
	}

	nl, err := ups.d.AddListener(m.Hdr.Seq)
	if err != nil {
		return nil, err
	}
	defer ups.d.RemoveListener(m.Hdr.Seq)

	b, err := ups.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if _, err := ups.conn.WriteToUDP(b, ups.addr); err != nil {
		return nil, err
	}

	select {
	case err := <-nl.ErrChan:
		return nil, err
	case rsp := <-nl.RspChan:
		return rsp, nil
	case <-nl.AfterTimeout(opt.Timeout):
		msg := fmt.Sprintf(
			"NMP timeout; op=%d group=%d id=%d seq=%d peer=%#v",
			b[0], b[4]+b[5]<<8, b[7], b[6], ups.addr)

		return nil, nmxutil.NewRspTimeoutError(msg)
	}
}

func (ups *UdpPlainSesn) AbortRx(seq uint8) error {
	return ups.d.ErrorOne(seq, fmt.Errorf("Rx aborted"))
}

func (ups *UdpPlainSesn) GetResourceOnce(uri string, opt sesn.TxOptions) (
	[]byte, error) {

	return nil, fmt.Errorf("UdpPlainSesn.GetResourceOnce() unsupported")
}
