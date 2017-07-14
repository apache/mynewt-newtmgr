package nmxutil

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/ugorji/go/codec"

	log "github.com/Sirupsen/logrus"
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
	ForceColors:     true,
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

func NextToken() []byte {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !oicSeqBeenRead {
		nextOicSeq = uint8(rand.Uint32())
		oicSeqBeenRead = true
	}

	token := []byte{nextOicSeq}
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

func LogAddListener(parentLevel int, base interface{}, id uint32,
	name string) {

	LogListener(parentLevel, "add-ble-listener",
		fmt.Sprintf("[%d] %s: base=%+v", id, name, base))
}

func LogRemoveListener(parentLevel int, base interface{}, id uint32,
	name string) {

	LogListener(parentLevel, "remove-ble-listener",
		fmt.Sprintf("[%d] %s: base=%+v", id, name, base))
}
