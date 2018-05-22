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

package mtech_lora

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/joaojeronimo/go-crc16"
	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/lora"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmcoap"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type LoraSesn struct {
	cfg           sesn.SesnCfg
	txvr          *mgmt.Transceiver
	isOpen        bool
	mtu           int
	xport         *LoraXport
	msgListener   *Listener
	reassListener *Listener
	tgtListener   *Listener
	wg            sync.WaitGroup
	stopChan      chan struct{}

	txFilterCb nmcoap.MsgFilter
	rxFilterCb nmcoap.MsgFilter
}

type mtechLoraTx struct {
	Port uint8  `json:"port"`
	Data string `json:"data"`
	Ack  bool   `json:"ack"`
}

func NewLoraSesn(cfg sesn.SesnCfg, lx *LoraXport) (*LoraSesn, error) {
	if cfg.Lora.Addr != "" || cfg.MgmtProto != sesn.MGMT_PROTO_COAP_SERVER {
		// Allow server to bind to wildcard address
		addr, err := NormalizeAddr(cfg.Lora.Addr)
		if err != nil {
			return nil, fmt.Errorf("Invalid Lora address %s\n", cfg.Lora.Addr)
		}
		cfg.Lora.Addr = addr
	}

	if cfg.Lora.Port < 1 || cfg.Lora.Port > 223 {
		return nil, fmt.Errorf("Invalid Lora Port %d\n", cfg.Lora.Addr)
	}
	s := &LoraSesn{
		cfg:        cfg,
		xport:      lx,
		mtu:        0,
		txFilterCb: cfg.TxFilterCb,
		rxFilterCb: cfg.RxFilterCb,
	}

	return s, nil
}

func (s *LoraSesn) Open() error {
	if s.isOpen == true {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open Lora session")
	}

	txFilterCb, rxFilterCb := s.Filters()
	txvr, err := mgmt.NewTransceiver(txFilterCb, rxFilterCb, false,
		s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr
	s.stopChan = make(chan struct{})

	msgType := "rsp"
	if s.cfg.MgmtProto == sesn.MGMT_PROTO_COAP_SERVER {
		msgType = "req"
	}

	s.xport.Lock()

	msgKey := TgtPortKey(s.cfg.Lora.Addr, s.cfg.Lora.Port, msgType)
	s.msgListener = NewListener()
	err = s.xport.msgMap.AddListener(msgKey, s.msgListener)
	if err != nil {
		s.xport.Unlock()
		s.txvr.Stop()
		return err
	}

	if s.cfg.Lora.Addr != "" {
		msgKey, l := s.xport.reassMap.FindListener(s.cfg.Lora.Addr,
			s.cfg.Lora.Port, "reass")
		if l != nil {
			l.RefCnt++
		} else {
			msgKey = TgtPortKey(s.cfg.Lora.Addr, s.cfg.Lora.Port, "reass")
			l = NewListener()
			s.xport.reassMap.AddListener(msgKey, l)
		}
		s.reassListener = l

		tgtKey := TgtKey(s.cfg.Lora.Addr, "rx")
		s.tgtListener = NewListener()
		err = s.xport.tgtMap.AddListener(tgtKey, s.tgtListener)
		if err != nil {
			s.xport.Unlock()
			s.closeListeners()
			s.txvr.Stop()
			return err
		}
	}

	s.xport.Unlock()

	if s.cfg.MgmtProto == sesn.MGMT_PROTO_COAP_SERVER {
		s.isOpen = true
		return nil
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.closeListeners()

		for {
			select {
			case msg, ok := <-s.msgListener.MsgChan:
				if ok {
					s.txvr.DispatchCoap(msg)
				}
			case mtu, ok := <-s.tgtListener.MtuChan:
				if ok {
					if s.mtu != mtu {
						log.Debugf("Setting mtu for %s %d",
							s.cfg.Lora.Addr, mtu)
					}
					s.mtu = mtu
				}
			case <-s.stopChan:
				return
			}
		}
	}()
	s.isOpen = true
	return nil
}

func (s *LoraSesn) closeListeners() {
	s.xport.Lock()
	s.xport.msgMap.RemoveListener(s.msgListener)
	if s.tgtListener != nil {
		s.xport.tgtMap.RemoveListener(s.tgtListener)
	}
	if s.reassListener != nil {
		s.reassListener.Close()
		if s.reassListener.RefCnt == 0 {
			s.xport.reassMap.RemoveListener(s.reassListener)
		}
	}
	s.xport.Unlock()
}

func (s *LoraSesn) Close() error {
	if s.isOpen == false {
		return nmxutil.NewSesnClosedError(
			"Attempt to close an unopened Lora session")
	}

	s.isOpen = false
	s.txvr.ErrorAll(fmt.Errorf("manual close"))
	s.txvr.Stop()
	close(s.stopChan)
	s.msgListener.Close()
	if s.tgtListener != nil {
		s.tgtListener.Close()
	}
	s.wg.Wait()

	if s.cfg.Lora.Addr != "" {
		s.xport.Tx([]byte(fmt.Sprintf("lora/%s/flush",
			DenormalizeAddr(s.cfg.Lora.Addr))))
	}
	s.stopChan = nil
	s.txvr = nil

	if s.cfg.MgmtProto == sesn.MGMT_PROTO_COAP_SERVER {
		s.closeListeners()
	}

	return nil
}

func (s *LoraSesn) Mtu() int {
	if s.cfg.Lora.SegSz != 0 {
		return s.cfg.Lora.SegSz
	}
	if s.mtu != 0 {
		return s.mtu
	}
	return s.xport.minMtu()
}

func (s *LoraSesn) IsOpen() bool {
	return s.isOpen
}

func (s *LoraSesn) MtuIn() int {
	return MAX_PACKET_SIZE_IN - omp.OMP_MSG_OVERHEAD - nmp.NMP_HDR_SIZE
}

func (s *LoraSesn) MtuOut() int {
	// We want image upload to use chunk size which fits inside a single
	// lora segment, when possible. If the datarate is low enough, then we have
	// to fragment, but try to avoid it if possible.
	mtu := MAX_PACKET_SIZE_OUT
	if s.mtu > mtu {
		mtu = s.mtu
	}
	return mtu - omp.OMP_MSG_OVERHEAD - nmp.NMP_HDR_SIZE
}

func (s *LoraSesn) sendFragments(b []byte) error {
	segSz := s.Mtu()
	crc := crc16.Crc16(b)
	idx := 0
	for off := 0; off < len(b); {
		var seg bytes.Buffer
		var blkLen int
		if off == 0 {
			hdr := lora.CoapLoraFragStart{
				FragNum: 0,
				Crc:     crc,
			}
			blkLen = segSz - 4
			if blkLen >= len(b) {
				blkLen = len(b)
				hdr.FragNum |= lora.COAP_LORA_LAST_FRAG
			}
			binary.Write(&seg, binary.LittleEndian, hdr)
			seg.Write(b[0:blkLen])
		} else {
			hdr := lora.CoapLoraFrag{
				FragNum: uint8(idx),
			}
			blkLen = segSz - 1
			if blkLen >= len(b)-off {
				blkLen = len(b) - off
				hdr.FragNum |= lora.COAP_LORA_LAST_FRAG
			}
			binary.Write(&seg, binary.LittleEndian, hdr)
			seg.Write(b[off : off+blkLen])
		}
		off += blkLen
		idx++

		seg64 := make([]byte, base64.StdEncoding.EncodedLen(len(seg.Bytes())))
		base64.StdEncoding.Encode(seg64, seg.Bytes())

		msg := mtechLoraTx{
			Port: s.cfg.Lora.Port,
			Ack:  s.cfg.Lora.ConfirmedTx,
			Data: string(seg64),
		}

		payload := []byte{}
		enc := codec.NewEncoderBytes(&payload, new(codec.JsonHandle))
		enc.Encode(msg)

		var outData bytes.Buffer

		outData.Write([]byte(fmt.Sprintf("lora/%s/down %s",
			DenormalizeAddr(s.cfg.Lora.Addr), payload)))
		err := s.xport.Tx(outData.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *LoraSesn) TxNmpOnce(m *nmp.NmpMsg, opt sesn.TxOptions) (
	nmp.NmpRsp, error) {

	if !s.IsOpen() {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed Lora session")
	}

	txFunc := func(b []byte) error {
		return s.sendFragments(b)
	}
	return s.txvr.TxNmp(txFunc, m, s.MtuOut(), opt.Timeout)
}

func (s *LoraSesn) AbortRx(seq uint8) error {
	s.txvr.ErrorAll(fmt.Errorf("Rx aborted"))
	return nil
}

func (s *LoraSesn) TxCoapOnce(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions) (coap.COAPCode, []byte, error) {

	if !s.IsOpen() {
		return 0, nil, nmxutil.NewSesnClosedError(
			"Attempt to transmit over closed Lora session")
	}
	txFunc := func(b []byte) error {
		return s.sendFragments(b)
	}
	rsp, err := s.txvr.TxOic(txFunc, m, s.MtuOut(), opt.Timeout)
	if err != nil {
		return 0, nil, err
	} else if rsp == nil {
		return 0, nil, nil
	} else {
		return rsp.Code(), rsp.Payload(), nil
	}
}

func (s *LoraSesn) TxCoapObserve(m coap.Message, resType sesn.ResourceType,
	opt sesn.TxOptions, NotifCb sesn.GetNotifyCb, stopsignal chan int) (coap.COAPCode, []byte, []byte, error) {
	return 0, nil, nil, nil
}

func (s *LoraSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *LoraSesn) CoapIsTcp() bool {
	return false
}

func (s *LoraSesn) RxAccept() (sesn.Sesn, *sesn.SesnCfg, error) {
	if !s.isOpen {
		return nil, nil, nmxutil.NewSesnClosedError(
			"Attempt to listen for data from closed connection")
	}
	if s.cfg.MgmtProto != sesn.MGMT_PROTO_COAP_SERVER {
		return nil, nil, fmt.Errorf("Invalid operation for %s", s.cfg.MgmtProto)
	}
	if s.tgtListener != nil {
		return nil, nil, fmt.Errorf("RxAccept() only wildcard destinations")
	}

	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case cl_s, ok := <-s.msgListener.ConnChan:
			if !ok {
				continue
			}
			cl_s.cfg.Lora.ConfirmedTx = s.cfg.Lora.ConfirmedTx
			cl_s.cfg.Lora.SegSz = s.cfg.Lora.SegSz
			cl_s.cfg.TxFilterCb = s.cfg.TxFilterCb
			cl_s.cfg.RxFilterCb = s.cfg.RxFilterCb
			return cl_s, &cl_s.cfg, nil
		case <-s.stopChan:
			return nil, nil, fmt.Errorf("Session closed")
		}
	}
}

func (s *LoraSesn) RxCoap(opt sesn.TxOptions) (coap.Message, error) {
	if !s.isOpen {
		return nil, nmxutil.NewSesnClosedError(
			"Attempt to listen for data from closed connection")
	}
	if s.cfg.MgmtProto != sesn.MGMT_PROTO_COAP_SERVER {
		return nil, fmt.Errorf("Invalid operation for %s", s.cfg.MgmtProto)
	}
	if s.tgtListener == nil {
		return nil, fmt.Errorf("RxCoap() only connected sessions")
	}

	waitTmoChan := time.After(opt.Timeout)
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case data, ok := <-s.msgListener.MsgChan:
			if !ok {
				continue
			}
			msg, err := s.txvr.ProcessCoapReq(data)
			if err != nil {
				return nil, err
			}
			if msg != nil {
				return msg, nil
			}
		case mtu, ok := <-s.tgtListener.MtuChan:
			if ok {
				if s.mtu != mtu {
					log.Debugf("Setting mtu for %s %d",
						s.cfg.Lora.Addr, mtu)
				}
				s.mtu = mtu
			}
		case _, ok := <-waitTmoChan:
			waitTmoChan = nil
			if ok {
				return nil, nmxutil.NewRspTimeoutError(
					"RxCoap() timed out")
			}
		case <-s.stopChan:
			return nil, fmt.Errorf("Session closed")
		}
	}
}

func (s *LoraSesn) Filters() (nmcoap.MsgFilter, nmcoap.MsgFilter) {
	return s.txFilterCb, s.rxFilterCb
}
