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

package nmxutil

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ugorji/go/codec"

	"mynewt.apache.org/newt/util"
)

const DURATION_FOREVER time.Duration = math.MaxInt64

var Debug bool

var nextNmpSeq uint8
var nmpSeqBeenRead bool
var nextOicSeq uint8
var oicSeqBeenRead bool
var seqMutex sync.Mutex

var logFormatter = log.TextFormatter{
	FullTimestamp:   true,
	TimestampFormat: "2006-01-02 15:04:05.999",
}

var ListenLog = &log.Logger{
	Out:       os.Stderr,
	Formatter: &logFormatter,
	Level:     log.DebugLevel,
}

func SetLogLevel(level log.Level) {
	log.SetLevel(level)
	log.SetFormatter(&logFormatter)
	ListenLog.Level = level
}

func Assert(cond bool) {
	if Debug && !cond {
		util.PrintStacks()
		panic("Failed assertion")
	}
}

func NextNmpSeq() uint8 {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !nmpSeqBeenRead {
		nextNmpSeq = uint8(rand.Uint32())
		nmpSeqBeenRead = true
	}

	val := nextNmpSeq
	nextNmpSeq++

	return val
}

func SeqToToken(seq uint8) []byte {
	return []byte{seq}
}

func NextToken() []byte {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !oicSeqBeenRead {
		nextOicSeq = uint8(rand.Uint32())
		oicSeqBeenRead = true
	}

	token := SeqToToken(nextOicSeq)
	nextOicSeq++

	return token
}

func DecodeCborMap(cbor []byte) (map[string]interface{}, error) {
	m := map[string]interface{}{}

	dec := codec.NewDecoderBytes(cbor, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		log.Debugf("Attempt to decode invalid cbor: %#v", cbor)
		return nil, fmt.Errorf("failure decoding cbor; %s", err.Error())
	}

	return m, nil
}

func EncodeCborMap(value map[string]interface{}) ([]byte, error) {
	b := []byte{}
	enc := codec.NewEncoderBytes(&b, new(codec.CborHandle))
	if err := enc.Encode(value); err != nil {
		return nil, fmt.Errorf("failure encoding cbor; %s", err.Error())
	}

	return b, nil
}

func DecodeCbor(cbor []byte) (interface{}, error) {
	var m interface{}

	dec := codec.NewDecoderBytes(cbor, new(codec.CborHandle))
	if err := dec.Decode(&m); err != nil {
		log.Debugf("Attempt to decode invalid cbor: %#v", cbor)
		return nil, fmt.Errorf("failure decoding cbor; %s", err.Error())
	}

	return m, nil
}

func EncodeCbor(value interface{}) ([]byte, error) {
	b := []byte{}
	enc := codec.NewEncoderBytes(&b, new(codec.CborHandle))
	if err := enc.Encode(value); err != nil {
		return nil, fmt.Errorf("failure encoding cbor; %s", err.Error())
	}

	return b, nil
}

func StopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		<-timer.C
	}
}

func Fragment(b []byte, mtu int) [][]byte {
	frags := [][]byte{}

	for off := 0; off < len(b); off += mtu {
		fragEnd := off + mtu
		if fragEnd > len(b) {
			fragEnd = len(b)
		}
		frag := b[off:fragEnd]
		frags = append(frags, frag)
	}

	return frags
}

var nextId uint32

func GetNextId() uint32 {
	return atomic.AddUint32(&nextId, 1) - 1
}

func LogListener(parentLevel int, title string, extra string) {
	_, file, line, _ := runtime.Caller(parentLevel)
	file = path.Base(file)
	ListenLog.Debugf("{%s} [%s:%d] %s", title, file, line, extra)
}

func LogAddNmpListener(parentLevel int, seq uint8) {
	LogListener(parentLevel, "add-nmp-listener", fmt.Sprintf("seq=%d", seq))
}

func LogRemoveNmpListener(parentLevel int, seq uint8) {
	LogListener(parentLevel, "remove-nmp-listener", fmt.Sprintf("seq=%d", seq))
}

func LogAddOicListener(parentLevel int, token []byte) {
	LogListener(parentLevel, "add-oic-listener",
		fmt.Sprintf("token=%+v", token))
}

func LogRemoveOicListener(parentLevel int, token []byte) {
	LogListener(parentLevel, "remove-oic-listener",
		fmt.Sprintf("token=%+v", token))
}

func LogAddListener(parentLevel int, key interface{}, id uint32,
	name string) {

	LogListener(parentLevel, "add-ble-listener",
		fmt.Sprintf("[%d] %s: base=%+v", id, name, key))
}

func LogRemoveListener(parentLevel int, key interface{}, id uint32,
	name string) {

	LogListener(parentLevel, "remove-ble-listener",
		fmt.Sprintf("[%d] %s: base=%+v", id, name, key))
}
