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
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newtmgr/nmxact/lora"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type LoraConfig struct {
	Addr        string
	SegSz       int
	ConfirmedTx bool
}

type LoraJoinedCb func(dev LoraConfig)

type LoraXport struct {
	sync.Mutex
	cfg       *LoraXportCfg
	started   bool
	rxConn    *net.UDPConn
	txConn    *net.UDPConn
	listenMap *ListenerMap
	exitChan  chan int
	joinCb    LoraJoinedCb
}

type LoraXportCfg struct {
}

type LoraData struct {
	Data string `codec:"data"`
	Port int    `codec:"port"`
}

type LoraPacketSent struct {
	DataRate string `codec:"datr"`
}

/*
 * This maps datarate string into a max mtu we can use
 */
var LoraDataRateMapUS = map[string]int{
	"SF12BW500": 33,
	"SF11BW500": 109,
	"SF10BW500": 222,
	"SF9BW500":  222,
	"SF8BW500":  222,
	"SF7BW500":  222,
}

const MAX_PACKET_SIZE_IN = 2048
const MAX_PACKET_SIZE_OUT = 128
const UDP_RX_PORT = 1784
const UDP_TX_PORT = 1786
const OIC_LORA_PORT = 0xbb

func NewXportCfg() *LoraXportCfg {
	return &LoraXportCfg{}
}

func NewLoraXport(cfg *LoraXportCfg) *LoraXport {
	return &LoraXport{
		cfg:       cfg,
		listenMap: NewListenerMap(),
	}
}

func (lx *LoraXport) minMtu() int {
	return 33
}

func NormalizeAddr(addr string) (string, error) {
	a := strings.Replace(addr, ":", "", -1)
	a = strings.Replace(addr, "-", "", -1)
	// XXX check that there's 8 components, 2 chars each, which are in [0-9,a-f]
	if len(a) != 16 {
		return "", fmt.Errorf("Invalid addr")
	}
	return a, nil
}

func DenormalizeAddr(addr string) string {
	if len(addr) != 16 {
		return "<invalid>"
	}
	rc := ""
	for i := 0; i < 16; i+=2 {
		rc += addr[i : i+2]
		if 16-i > 2 {
			rc += "-"
		}
	}
	return rc
}

func (lx *LoraXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	if cfg.Lora.Addr == "" {
		return nil, fmt.Errorf("Need an address of endpoint")
	}
	return NewLoraSesn(cfg, lx)
}

func (lx *LoraXport) reass(dev string, data []byte) {
	lx.Lock()
	defer lx.Unlock()

	_, l := lx.listenMap.FindListener(dev, "rx")
	if l == nil {
		log.Debugf("No LoRa listener for %s", dev)
		return
	}

	str := hex.Dump(data)

	log.Debugf("Raw data: %s", str)
	bufR := bytes.NewReader(data)

	var frag lora.CoapLoraFrag
	var sFrag lora.CoapLoraFragStart

	err := binary.Read(bufR, binary.LittleEndian, &frag)
	if err != nil {
		log.Debugf("Can't read header")
		return
	}

	fragNum := frag.FragNum &^ lora.COAP_LORA_LAST_FRAG
	if l.Data.Len() == 0 {
		if fragNum != 0 {
			log.Debugf("frag num != 0 with empty queue")
			return
		}
		bufR.Seek(0, 0)
		err = binary.Read(bufR, binary.LittleEndian, &sFrag)
		if err != nil {
			log.Debugf("Can't read in start header")
			return
		}
		l.Crc = sFrag.Crc
		l.Data.Write(data[3:])
		l.NextFrag = 1
	} else {
		if fragNum != l.NextFrag {
			log.Debugf("Frag out of sequence")
			l.Data.Reset()
			return
		}
		l.Data.Write(data[1:])
		l.NextFrag++
	}

	if (frag.FragNum & lora.COAP_LORA_LAST_FRAG) != 0 {
		l.MsgChan <- l.Data.Bytes()
		l.Data.Reset()
	}
}

func (lx *LoraXport) dataRateSeen(dev string, dataRate string) {
	lx.Lock()
	defer lx.Unlock()

	_, l := lx.listenMap.FindListener(dev, "rx")
	if l == nil {
		return
	}
	mtu, ok := LoraDataRateMapUS[dataRate]
	if !ok {
		mtu = lx.minMtu()
	}
	l.MtuChan <- mtu
}

/*
 * lora/00-13-50-04-04-50-13-00/up
 */
func (lx *LoraXport) processData(data string) {
	if strings.HasPrefix(data, "lora/") == false {
		return
	}
	splitMsg := strings.Fields(data)
	if len(splitMsg) == 0 {
		return
	}
	splitHdr := strings.Split(splitMsg[0], "/")
	if len(splitHdr) != 3 {
		return
	}
	dev, _ := NormalizeAddr(splitHdr[1])
	switch splitHdr[2] {
	case "joined":
		log.Debugf("loraxport rx: %s", data)
		log.Debugf("%s joined", dev)
		lx.reportJoin(dev)
	case "up":
		var msg LoraData

		log.Debugf("loraxport rx: %s", data)
		pload := []byte(splitMsg[1])
		err := codec.NewDecoderBytes(pload, new(codec.JsonHandle)).Decode(&msg)
		if err != nil {
			log.Debugf("loraxport rx: error decoding json: %v", err)
			return
		}

		if msg.Port != OIC_LORA_PORT {
			log.Debugf("loraxport rx: ignoring data to port %d", msg.Port)
			return
		}
		dec, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			log.Debugf("loraxport rx: error decoding base64: %v", err)
			return
		}
		lx.reass(dev, dec)
	case "packet_sent":
		var sent LoraPacketSent

		log.Debugf("loraxport rx: %s", data)
		pload := []byte(splitMsg[1])
		err := codec.NewDecoderBytes(pload, new(codec.JsonHandle)).Decode(&sent)
		if err != nil {
			log.Debugf("loraxport rx: error decoding json: %v", err)
			return
		}

		lx.dataRateSeen(dev, sent.DataRate)
	}
}

func (lx *LoraXport) Start() error {
	if lx.started {
		return nmxutil.NewXportError("Lora xport started twice")
	}

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1784")
	if err != nil {
		return fmt.Errorf("Failure resolving name for UDP session: %s",
			err.Error())
	}

	// XXX need so_reuseport
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Failed to open RX to lora-network-server %s", addr)
	}
	lx.rxConn = conn

	addr, err = net.ResolveUDPAddr("udp", "127.0.0.1:1786")
	if err != nil {
		return fmt.Errorf("Failure resolving name for UDP session: %s",
			err.Error())
	}
	conn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		lx.rxConn.Close()
		lx.rxConn = nil
		return fmt.Errorf("Failed to open TX to lora-network-server")
	}
	lx.txConn = conn

	lx.started = true

	go func() {
		data := make([]byte, MAX_PACKET_SIZE_IN*4/3+512)
		for {
			nr, _, err := lx.rxConn.ReadFromUDP(data)
			if err != nil {
				return
			}
			lx.processData(string(data[0:nr]))
		}
	}()

	return nil
}

func (lx *LoraXport) reportJoin(dev string) {
	lx.Lock()
	if lx.joinCb != nil {
		dev := LoraConfig{
			Addr: dev,
		}
		lx.Unlock()
		lx.joinCb(dev)
	} else {
		lx.Unlock()
	}
}

func (lx *LoraXport) SetJoinCb(joinCb LoraJoinedCb) {
	lx.Lock()
	defer lx.Unlock()

	lx.joinCb = joinCb
}

func (lx *LoraXport) Stop() error {
	if !lx.started {
		return nmxutil.NewXportError("Lora xport stopped twice")
	}
	lx.rxConn.Close()
	lx.rxConn = nil
	lx.txConn.Close()
	lx.txConn = nil
	lx.started = false
	lx.joinCb = nil
	return nil
}

func (lx *LoraXport) Tx(bytes []byte) error {
	log.Debugf("loraxport tx: %s", bytes)
	_, err := lx.txConn.Write(bytes)
	return err
}