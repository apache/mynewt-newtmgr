/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package udp

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

const testTimeout = 5 * time.Second

// fakeUdpDevice is a management server on a loopback UDP socket that
// answers NMP or OMP echo requests.
type fakeUdpDevice struct {
	conn    *net.UDPConn
	proto   sesn.MgmtProto
	respond bool

	mtx  sync.Mutex
	errs []error
	reqs int

	wg sync.WaitGroup
}

func newFakeUdpDevice(t *testing.T, proto sesn.MgmtProto,
	respond bool) *fakeUdpDevice {

	t.Helper()

	conn, err := net.ListenUDP("udp",
		&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP failed: %v", err)
	}

	f := &fakeUdpDevice{
		conn:    conn,
		proto:   proto,
		respond: respond,
	}

	f.wg.Add(1)
	go f.serve()

	t.Cleanup(func() {
		conn.Close()
		f.wg.Wait()

		f.mtx.Lock()
		defer f.mtx.Unlock()
		for _, err := range f.errs {
			t.Errorf("fake device: %v", err)
		}
	})

	return f
}

func (f *fakeUdpDevice) addr() string {
	return f.conn.LocalAddr().String()
}

func (f *fakeUdpDevice) numReqs() int {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	return f.reqs
}

func (f *fakeUdpDevice) serve() {
	defer f.wg.Done()

	buf := make([]byte, 4096)
	for {
		n, src, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		f.mtx.Lock()
		f.reqs++
		f.mtx.Unlock()

		rsp, err := f.process(buf[:n])
		if err != nil {
			f.mtx.Lock()
			f.errs = append(f.errs, err)
			f.mtx.Unlock()
			continue
		}

		if f.respond {
			f.conn.WriteToUDP(rsp, src)
		}
	}
}

func cborDecodeMap(b []byte) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	dec := codec.NewDecoderBytes(b, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func echoRspHdr(seq uint8) nmp.NmpHdr {
	return nmp.NmpHdr{
		Op:    nmp.NMP_OP_WRITE_RSP,
		Group: nmp.NMP_GROUP_DEFAULT,
		Id:    nmp.NMP_ID_DEF_ECHO,
		Seq:   seq,
	}
}

func (f *fakeUdpDevice) process(req []byte) ([]byte, error) {
	if f.proto == sesn.MGMT_PROTO_NMP {
		hdr, err := nmp.DecodeNmpHdr(req)
		if err != nil {
			return nil, err
		}
		body, err := cborDecodeMap(req[nmp.NMP_HDR_SIZE:])
		if err != nil {
			return nil, err
		}
		return nmp.EncodeNmpPlain(&nmp.NmpMsg{
			Hdr:  echoRspHdr(hdr.Seq),
			Body: map[string]interface{}{"r": body["d"], "rc": 0},
		})
	}

	m, err := coap.ParseDgramMessage(req)
	if err != nil {
		return nil, err
	}
	body, err := cborDecodeMap(m.Payload())
	if err != nil {
		return nil, err
	}
	hb, ok := body["_h"].([]byte)
	if !ok {
		return nil, fmt.Errorf("OMP request without NMP header")
	}
	hdr, err := nmp.DecodeNmpHdr(hb)
	if err != nil {
		return nil, err
	}

	rh := echoRspHdr(hdr.Seq)
	var payload []byte
	enc := codec.NewEncoderBytes(&payload, new(codec.CborHandle))
	err = enc.Encode(map[string]interface{}{
		"_h": rh.Bytes(),
		"r":  body["d"],
		"rc": 0,
	})
	if err != nil {
		return nil, err
	}

	rsp := coap.NewDgramMessage(coap.MessageParams{
		Type:      coap.Acknowledgement,
		Code:      coap.Content,
		MessageID: m.MessageID(),
		Token:     m.Token(),
		Payload:   payload,
	})
	return rsp.MarshalBinary()
}

func newTestSesn(t *testing.T, proto sesn.MgmtProto,
	addr string) *UdpSesn {

	t.Helper()

	cfg := sesn.NewSesnCfg()
	cfg.MgmtProto = proto
	cfg.PeerSpec.Udp = addr

	s, err := NewUdpSesn(cfg)
	if err != nil {
		t.Fatalf("NewUdpSesn failed: %v", err)
	}
	return s
}

func echoMsg(payload string) *nmp.NmpMsg {
	r := nmp.NewEchoReq()
	r.Payload = payload
	return r.Msg()
}

func checkEcho(t *testing.T, rsp nmp.NmpRsp, seq uint8, payload string) {
	t.Helper()

	echo, ok := rsp.(*nmp.EchoRsp)
	if !ok {
		t.Fatalf("got %T, want *nmp.EchoRsp", rsp)
	}
	if echo.Payload != payload || echo.Hdr().Seq != seq {
		t.Errorf("got payload %q seq %d, want %q seq %d",
			echo.Payload, echo.Hdr().Seq, payload, seq)
	}
}

var udpProtos = []sesn.MgmtProto{sesn.MGMT_PROTO_NMP, sesn.MGMT_PROTO_OMP}

func TestUdpSesnTxRxMgmt(t *testing.T) {
	for _, proto := range udpProtos {
		t.Run(proto.String(), func(t *testing.T) {
			dev := newFakeUdpDevice(t, proto, true)
			s := newTestSesn(t, proto, dev.addr())

			if s.IsOpen() {
				t.Fatalf("new session is open")
			}
			if s.MgmtProto() != proto {
				t.Errorf("MgmtProto() = %v, want %v", s.MgmtProto(), proto)
			}
			if err := s.Open(); err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			if !s.IsOpen() {
				t.Fatalf("session not open after Open")
			}

			for i := 0; i < 3; i++ {
				req := echoMsg(fmt.Sprintf("ping %d", i))
				rsp, err := s.TxRxMgmt(req, testTimeout)
				if err != nil {
					t.Fatalf("TxRxMgmt failed: %v", err)
				}
				checkEcho(t, rsp, req.Hdr.Seq, fmt.Sprintf("ping %d", i))
			}

			if err := s.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}
			if s.IsOpen() {
				t.Errorf("session open after Close")
			}
		})
	}
}

func TestUdpSesnOpenCloseTwice(t *testing.T) {
	dev := newFakeUdpDevice(t, sesn.MGMT_PROTO_NMP, true)
	s := newTestSesn(t, sesn.MGMT_PROTO_NMP, dev.addr())

	if err := s.Close(); !nmxutil.IsSesnClosed(err) {
		t.Errorf("Close of unopened session: got %v", err)
	}
	if err := s.Open(); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := s.Open(); !nmxutil.IsSesnAlreadyOpen(err) {
		t.Errorf("second Open: got %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := s.Close(); !nmxutil.IsSesnClosed(err) {
		t.Errorf("second Close: got %v", err)
	}

	if _, err := s.TxRxMgmt(echoMsg("x"), testTimeout); err == nil {
		t.Errorf("TxRxMgmt on closed session succeeded")
	}
}

func TestUdpSesnReopen(t *testing.T) {
	for _, proto := range udpProtos {
		t.Run(proto.String(), func(t *testing.T) {
			dev := newFakeUdpDevice(t, proto, true)
			s := newTestSesn(t, proto, dev.addr())

			for i := 0; i < 2; i++ {
				if err := s.Open(); err != nil {
					t.Fatalf("Open %d failed: %v", i, err)
				}
				req := echoMsg("again")
				rsp, err := s.TxRxMgmt(req, testTimeout)
				if err != nil {
					t.Fatalf("TxRxMgmt %d failed: %v", i, err)
				}
				checkEcho(t, rsp, req.Hdr.Seq, "again")
				if err := s.Close(); err != nil {
					t.Fatalf("Close %d failed: %v", i, err)
				}
			}
		})
	}
}

// Regression test for 8473977: TxRxMgmtAsync must deliver the response
// on ch (UDP sessions implement it synchronously).
func TestUdpSesnTxRxMgmtAsync(t *testing.T) {
	for _, proto := range udpProtos {
		t.Run(proto.String(), func(t *testing.T) {
			dev := newFakeUdpDevice(t, proto, true)
			s := newTestSesn(t, proto, dev.addr())
			if err := s.Open(); err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			ch := make(chan nmp.NmpRsp, 1)
			errc := make(chan error, 1)
			req := echoMsg("async")

			if err := s.TxRxMgmtAsync(req, testTimeout, ch, errc); err != nil {
				t.Fatalf("TxRxMgmtAsync failed: %v", err)
			}

			select {
			case rsp := <-ch:
				checkEcho(t, rsp, req.Hdr.Seq, "async")
			case err := <-errc:
				t.Fatalf("unexpected error: %v", err)
			case <-time.After(testTimeout):
				t.Fatalf("timeout waiting for async response")
			}
		})
	}
}

func TestUdpSesnTimeout(t *testing.T) {
	for _, proto := range udpProtos {
		t.Run(proto.String(), func(t *testing.T) {
			dev := newFakeUdpDevice(t, proto, false)
			s := newTestSesn(t, proto, dev.addr())
			if err := s.Open(); err != nil {
				t.Fatal(err)
			}
			defer s.Close()

			_, err := s.TxRxMgmt(echoMsg("x"), 50*time.Millisecond)
			if !nmxutil.IsRspTimeout(err) {
				t.Fatalf("got %v (%T), want RspTimeoutError", err, err)
			}

			// The async variant reports the timeout on errc.
			ch := make(chan nmp.NmpRsp, 1)
			errc := make(chan error, 1)
			s.TxRxMgmtAsync(echoMsg("y"), 50*time.Millisecond, ch, errc)
			select {
			case err := <-errc:
				if !nmxutil.IsRspTimeout(err) {
					t.Errorf("got %v, want RspTimeoutError", err)
				}
			case rsp := <-ch:
				t.Fatalf("unexpected response %+v", rsp)
			case <-time.After(testTimeout):
				t.Fatalf("timeout waiting for async error")
			}

			if n := dev.numReqs(); n != 2 {
				t.Errorf("device saw %d requests, want 2", n)
			}
		})
	}
}

func TestUdpSesnRequestTooBig(t *testing.T) {
	dev := newFakeUdpDevice(t, sesn.MGMT_PROTO_NMP, true)
	s := newTestSesn(t, sesn.MGMT_PROTO_NMP, dev.addr())
	if err := s.Open(); err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	req := echoMsg(strings.Repeat("x", MAX_PACKET_SIZE))
	if _, err := s.TxRxMgmt(req, testTimeout); err == nil {
		t.Errorf("oversized request succeeded")
	}
}

func TestUdpSesnBadPeer(t *testing.T) {
	s := newTestSesn(t, sesn.MGMT_PROTO_NMP, "not an address")
	if err := s.Open(); err == nil {
		s.Close()
		t.Fatalf("Open with invalid peer succeeded")
	}
	if s.IsOpen() {
		t.Errorf("session open after failed Open")
	}
}

func TestUdpXportStartStop(t *testing.T) {
	x := NewUdpXport()

	if err := x.Stop(); err == nil {
		t.Errorf("Stop of unstarted xport succeeded")
	}
	if err := x.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := x.Start(); err == nil {
		t.Errorf("second Start succeeded")
	}
	if err := x.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := x.Tx([]byte{1}); err == nil {
		t.Errorf("Tx succeeded")
	}

	cfg := sesn.NewSesnCfg()
	cfg.PeerSpec.Udp = "127.0.0.1:1"
	s, err := x.BuildSesn(cfg)
	if err != nil {
		t.Fatalf("BuildSesn failed: %v", err)
	}
	if _, ok := s.(*UdpSesn); !ok {
		t.Errorf("BuildSesn returned %T, want *UdpSesn", s)
	}
}

// The dispatch callback may hand the received data to another goroutine
// (OMP responses are decoded asynchronously and the CoAP payload refers to
// the input buffer), so each datagram must get its own buffer rather than
// being overwritten by the next one.
func TestListenBufferNotReused(t *testing.T) {
	dev, err := net.ListenUDP("udp",
		&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()

	rx := make(chan []byte, 2)
	conn, _, err := Listen(dev.LocalAddr().String(), func(data []byte) {
		rx <- data
	})
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer conn.Close()

	dst := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: conn.LocalAddr().(*net.UDPAddr).Port,
	}
	for _, s := range []string{"first", "SECND"} {
		if _, err := dev.WriteToUDP([]byte(s), dst); err != nil {
			t.Fatal(err)
		}
	}

	var bufs [][]byte
	for i := 0; i < 2; i++ {
		select {
		case b := <-rx:
			bufs = append(bufs, b)
		case <-time.After(testTimeout):
			t.Fatalf("timeout waiting for datagram %d", i)
		}
	}

	// Both datagrams have been received; the first slice must still hold
	// the first datagram.
	if string(bufs[0]) != "first" {
		t.Errorf("first datagram = %q, want %q", bufs[0], "first")
	}
	if string(bufs[1]) != "SECND" {
		t.Errorf("second datagram = %q", bufs[1])
	}
}
