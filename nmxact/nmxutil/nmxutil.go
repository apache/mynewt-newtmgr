package nmxutil

import (
	"math/rand"
	"sync"
)

var nextSeq uint8
var beenRead bool
var seqMutex sync.Mutex

func NextSeq() uint8 {
	seqMutex.Lock()
	defer seqMutex.Unlock()

	if !beenRead {
		nextSeq = uint8(rand.Uint32())
		beenRead = true
	}

	val := nextSeq
	nextSeq++

	return val
}
