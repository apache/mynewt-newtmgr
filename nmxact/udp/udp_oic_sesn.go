package udp

import (
	"fmt"
	"net"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type UdpOicSesn struct {
	cfg  sesn.SesnCfg
	addr *net.UDPAddr
	conn *net.UDPConn
	od   *omp.OmpDispatcher
}

func NewUdpOicSesn(cfg sesn.SesnCfg) *UdpOicSesn {
	uos := &UdpOicSesn{
		cfg: cfg,
		od:  omp.NewOmpDispatcher(false),
	}

	return uos
}

func (uos *UdpOicSesn) Open() error {
	if uos.conn != nil {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open UDP session")
	}

	conn, addr, err := Listen(uos.cfg.PeerSpec.Udp,
		func(data []byte) {
			uos.od.Dispatch(data)
		})
	if err != nil {
		return err
	}

	uos.addr = addr
	uos.conn = conn
	return nil
}

func (uos *UdpOicSesn) Close() error {
	if uos.conn == nil {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened UDP session")
	}

	uos.conn.Close()
	uos.conn = nil
	uos.addr = nil
	return nil
}

func (uos *UdpOicSesn) IsOpen() bool {
	return uos.conn != nil
}

func (uos *UdpOicSesn) MtuIn() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (uos *UdpOicSesn) MtuOut() int {
	return MAX_PACKET_SIZE -
		omp.OMP_MSG_OVERHEAD -
		nmp.NMP_HDR_SIZE
}

func (uos *UdpOicSesn) EncodeNmpMsg(m *nmp.NmpMsg) ([]byte, error) {
	return omp.EncodeOmpDgram(m)
}

func (uos *UdpOicSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !uos.IsOpen() {
		return nil, fmt.Errorf("Attempt to transmit over closed UDP session")
	}

	nl := nmp.NewNmpListener()
	if err := uos.od.AddListener(m.Hdr.Seq, nl); err != nil {
		return nil, err
	}
	defer uos.od.RemoveListener(m.Hdr.Seq)

	b, err := uos.EncodeNmpMsg(m)
	if err != nil {
		return nil, err
	}

	if _, err := uos.conn.WriteToUDP(b, uos.addr); err != nil {
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
			b[0], b[4]+b[5]<<8, b[7], b[6], uos.addr)

		return nil, nmxutil.NewNmpTimeoutError(msg)
	}
}

func (uos *UdpOicSesn) AbortRx(seq uint8) error {
	return uos.od.FakeRxError(seq, fmt.Errorf("Rx aborted"))
}
