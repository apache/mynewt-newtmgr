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
	"strings"
	"sync"

	"github.com/joaojeronimo/go-crc16"
	"github.com/runtimeco/go-coap"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/lora"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/omp"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type LoraSesn struct {
	cfg      sesn.SesnCfg
	txvr     *mgmt.Transceiver
	isOpen   bool
	xport    *LoraXport
	listener *Listener
	wg       sync.WaitGroup
	stopChan chan struct{}
}

type mtechLoraTx struct {
	Port uint16 `json:"port"`
	Data string `json:"data"`
	Ack  bool   `json:"ack"`
}

func normalizeAddr(addr string) (string, error) {
	a := strings.Replace(addr, ":", "-", -1)
	// XXX check that there's 8 components, 2 chars each, which are in [0-9,a-f]
	return a, nil
}

func NewLoraSesn(cfg sesn.SesnCfg, lx *LoraXport) (*LoraSesn, error) {
	addr, err := normalizeAddr(cfg.Lora.Addr)
	if err != nil {
		return nil, fmt.Errorf("Invalid Lora address %s\n", cfg.Lora.Addr)
	}
	cfg.Lora.Addr = addr
	s := &LoraSesn{
		cfg:   cfg,
		xport: lx,
	}

	return s, nil
}

func (s *LoraSesn) Open() error {
	if s.isOpen == true {
		return nmxutil.NewSesnAlreadyOpenError(
			"Attempt to open an already-open Lora session")
	}

	key := TgtKey(s.cfg.Lora.Addr, "rx")
	s.xport.Lock()

	txvr, err := mgmt.NewTransceiver(false, s.cfg.MgmtProto, 3)
	if err != nil {
		return err
	}
	s.txvr = txvr
	s.stopChan = make(chan struct{})
	s.listener = NewListener()

	err = s.xport.listenMap.AddListener(key, s.listener)
	if err != nil {
		s.txvr.Stop()
		return err
	}
	s.xport.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.xport.listenMap.RemoveListener(s.listener)

		for {
			select {
			case msg, ok := <-s.listener.MsgChan:
				if ok {
					s.txvr.DispatchCoap(msg)
				}
			case <-s.stopChan:
				return
			}
		}
	}()
	s.isOpen = true
	return nil
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
	s.listener.Close()
	s.wg.Wait()
	s.stopChan = nil
	s.txvr = nil

	return nil
}

func (s *LoraSesn) IsOpen() bool {
	return s.isOpen
}

func (s *LoraSesn) MtuIn() int {
	return MAX_PACKET_SIZE - omp.OMP_MSG_OVERHEAD - nmp.NMP_HDR_SIZE
}

func (s *LoraSesn) MtuOut() int {
	return MAX_PACKET_SIZE - omp.OMP_MSG_OVERHEAD - nmp.NMP_HDR_SIZE
}

func (s *LoraSesn) send_fragments(b []byte) error {
	segSz := s.xport.minMtu()
	if segSz < s.cfg.Lora.SegSz {
		segSz = s.cfg.Lora.SegSz
	}
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
			Port: OIC_LORA_PORT,
			Ack:  s.cfg.Lora.ConfirmedTx,
			Data: string(seg64),
		}

		payload := []byte{}
		enc := codec.NewEncoderBytes(&payload, new(codec.JsonHandle))
		enc.Encode(msg)

		var outData bytes.Buffer

		outData.Write([]byte(fmt.Sprintf("lora/%s/down %s\n",
			s.cfg.Lora.Addr, payload)))
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
		return nil, fmt.Errorf("Attempt to transmit over closed Lora session")
	}

	txFunc := func(b []byte) error {
		return s.send_fragments(b)
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
		return 0, nil, fmt.Errorf("Attempt to transmit over closed Lora session")
	}
	txFunc := func(b []byte) error {
		return s.send_fragments(b)
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

func (s *LoraSesn) MgmtProto() sesn.MgmtProto {
	return s.cfg.MgmtProto
}

func (s *LoraSesn) CoapIsTcp() bool {
	return false
}
