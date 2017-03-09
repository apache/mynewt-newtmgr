package nmxutil

import (
	"math/rand"
	"sync"
)

var nextNmpSeq uint8
var beenRead bool
var seqMutex sync.Mutex

func NextNmpSeq() uint8 {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !beenRead {
		nextNmpSeq = uint8(rand.Uint32())
		beenRead = true
	}

	val := nextNmpSeq
	nextNmpSeq++

	return val
}
