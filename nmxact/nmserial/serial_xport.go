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

package nmserial

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/joaojeronimo/go-crc16"
	"github.com/runtimeco/go-coap"
	"github.com/tarm/serial"

	"mynewt.apache.org/newt/util"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
)

type XportCfg struct {
	DevPath     string
	Baud        int
	Mtu         int
	ReadTimeout time.Duration
}

var errTimeout error = errors.New("Timeout reading from serial connection")

func NewXportCfg() *XportCfg {
	return &XportCfg{
		ReadTimeout: 10 * time.Second,
		Mtu:         512,
	}
}

type SerialXport struct {
	cfg     *XportCfg
	port    *serial.Port
	scanner *bufio.Scanner

	wg sync.WaitGroup
	sync.Mutex
	closing bool

	reqSesn    *SerialSesn
	acceptSesn *SerialSesn
	rspSesn    *SerialSesn

	pkt *Packet
}

func NewSerialXport(cfg *XportCfg) *SerialXport {
	return &SerialXport{
		cfg: cfg,
	}
}

func (sx *SerialXport) BuildSesn(cfg sesn.SesnCfg) (sesn.Sesn, error) {
	return NewSerialSesn(sx, cfg)
}

func (sx *SerialXport) acceptServerSesn(sl *SerialSesn) (*SerialSesn, error) {
	sc := sesn.NewSesnCfg()
	sc.MgmtProto = sesn.MGMT_PROTO_COAP_SERVER
	sc.TxFilterCb = sl.cfg.TxFilterCb
	sc.RxFilterCb = sl.cfg.RxFilterCb

	s, err := NewSerialSesn(sx, sc)
	if err != nil {
		return nil, fmt.Errorf("NewSesn():%v", err)
	}
	err = s.Open()
	if err != nil {
		return nil, fmt.Errorf("Open():%v", err)
	}
	sl.connChan <- s
	sx.setReqSesn(s)

	return s, nil
}

func (sx *SerialXport) Start() error {
	c := &serial.Config{
		Name:        sx.cfg.DevPath,
		Baud:        sx.cfg.Baud,
		ReadTimeout: sx.cfg.ReadTimeout,
	}

	var err error
	sx.port, err = serial.OpenPort(c)
	if err != nil {
		return err
	}

	err = sx.port.Flush()
	if err != nil {
		return err
	}

	sx.wg.Add(1)
	go func() {
		defer sx.wg.Done()

		// Most of the reading will be done line by line, use the
		// bufio.Scanner to do this
		sx.scanner = bufio.NewScanner(sx.port)

		for {
			msg, err := sx.Rx()
			sx.Lock()
			if err != nil {
				if sx.rspSesn != nil {
					sx.rspSesn.errChan <- err
				}
			}
			if sx.closing {
				sx.Unlock()
				return
			}
			if msg == nil {
				sx.Unlock()
				continue
			}
			if len(msg) >= 4 {
				if sx.reqSesn != nil || sx.acceptSesn != nil {
					msgType := coap.COAPCode(msg[1])
					if msgType <= coap.DELETE {
						if sx.reqSesn != nil {
							sx.reqSesn.msgChan <- msg
							sx.Unlock()
							continue
						}
						if sx.acceptSesn != nil {
							s, err := sx.acceptServerSesn(
								sx.acceptSesn)
							if err != nil {
								log.Errorf("Cannot create server sesn: %v", err)
								sx.Unlock()
								continue
							}
							s.msgChan <- msg
							sx.Unlock()
							continue
						}
					}
				}
			}
			if sx.rspSesn != nil {
				sx.rspSesn.msgChan <- msg
			}
			sx.Unlock()
		}

	}()
	return nil
}

func (sx *SerialXport) setRspSesn(s *SerialSesn) error {
	sx.Lock()
	defer sx.Unlock()

	if sx.closing {
		return fmt.Errorf("Transport closed")
	}
	if s != nil && sx.rspSesn != nil {
		return fmt.Errorf("Transport busy")
	}
	sx.rspSesn = s
	return nil
}

func (sx *SerialXport) setAcceptSesn(s *SerialSesn) error {
	sx.Lock()
	defer sx.Unlock()

	if sx.closing {
		return fmt.Errorf("Transport closed")
	}
	if sx.acceptSesn != nil && s != sx.acceptSesn {
		return fmt.Errorf("Transport busy")
	}
	sx.acceptSesn = s
	return nil
}

func (sx *SerialXport) setReqSesn(s *SerialSesn) error {
	if sx.closing {
		return fmt.Errorf("Transport closed")
	}
	if sx.reqSesn != nil && s != sx.reqSesn {
		return fmt.Errorf("Transport busy")
	}
	sx.reqSesn = s
	return nil
}

func (sx *SerialXport) Stop() error {
	sx.closing = true

	err := sx.port.Close()
	sx.wg.Wait()
	return err
}

func (sx *SerialXport) txRaw(bytes []byte) error {
	log.Debugf("Tx serial\n%s", hex.Dump(bytes))

	_, err := sx.port.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}

func (sx *SerialXport) Tx(bytes []byte) error {
	log.Debugf("Base64 encoding request:\n%s", hex.Dump(bytes))

	pktData := make([]byte, 2)

	crc := crc16.Crc16(bytes)
	binary.BigEndian.PutUint16(pktData, crc)
	bytes = append(bytes, pktData...)

	dLen := uint16(len(bytes))
	binary.BigEndian.PutUint16(pktData, dLen)
	pktData = append(pktData, bytes...)

	base64Data := make([]byte, base64.StdEncoding.EncodedLen(len(pktData)))

	base64.StdEncoding.Encode(base64Data, pktData)

	written := 0
	totlen := len(base64Data)

	for written < totlen {
		/* write the packet stat designators. They are
		 * different whether we are starting a new packet or continuing one */
		if written == 0 {
			sx.txRaw([]byte{6, 9})
		} else {
			/* slower platforms take some time to process each segment
			 * and have very small receive buffers.  Give them a bit of
			 * time here */
			time.Sleep(20 * time.Millisecond)
			sx.txRaw([]byte{4, 20})
		}

		/* ensure that the total frame fits into 128 bytes.
		 * base 64 is 3 ascii to 4 base 64 byte encoding.  so
		 * the number below should be a multiple of 4.  Also,
		 * we need to save room for the header (2 byte) and
		 * carriage return (and possibly LF 2 bytes), */

		/* all totaled, 124 bytes should work */
		writeLen := util.Min(124, totlen-written)

		writeBytes := base64Data[written : written+writeLen]
		sx.txRaw(writeBytes)
		sx.txRaw([]byte{'\n'})

		written += writeLen
	}

	return nil
}

// Blocking receive.
func (sx *SerialXport) Rx() ([]byte, error) {
	for sx.scanner.Scan() {
		line := []byte(sx.scanner.Text())

		for {
			if len(line) > 1 && line[0] == '\r' {
				line = line[1:]
			} else {
				break
			}
		}
		log.Debugf("Rx serial:\n%s", hex.Dump(line))
		if len(line) < 2 || ((line[0] != 4 || line[1] != 20) &&
			(line[0] != 6 || line[1] != 9)) {
			continue
		}

		base64Data := string(line[2:])

		data, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("Couldn't decode base64 string:"+
				" %s\nPacket hex dump:\n%s",
				base64Data, hex.Dump(line))
		}

		if line[0] == 6 && line[1] == 9 {
			if len(data) < 2 {
				continue
			}

			pktLen := binary.BigEndian.Uint16(data[0:2])
			sx.pkt, err = NewPacket(pktLen)
			if err != nil {
				return nil, err
			}
			data = data[2:]
		}

		if sx.pkt == nil {
			continue
		}

		full := sx.pkt.AddBytes(data)
		if full {
			if crc16.Crc16(sx.pkt.GetBytes()) != 0 {
				return nil, fmt.Errorf("CRC error")
			}

			/*
			 * Trim away the 2 bytes of CRC
			 */
			sx.pkt.TrimEnd(2)
			b := sx.pkt.GetBytes()
			sx.pkt = nil

			log.Debugf("Decoded input:\n%s", hex.Dump(b))
			return b, nil
		}
	}

	err := sx.scanner.Err()
	if err == nil {
		// Scanner hit EOF, so we'll need to create a new one.  This only
		// happens on timeouts.
		err = errTimeout
		sx.scanner = bufio.NewScanner(sx.port)
	}
	return nil, err
}
