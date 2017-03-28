package nmserial

import (
	"bytes"
)

type Packet struct {
	expectedLen uint16
	buffer      *bytes.Buffer
}

func NewPacket(expectedLen uint16) (*Packet, error) {
	pkt := &Packet{
		expectedLen: expectedLen,
		buffer:      bytes.NewBuffer([]byte{}),
	}

	return pkt, nil
}

func (pkt *Packet) AddBytes(bytes []byte) bool {
	pkt.buffer.Write(bytes)
	if pkt.buffer.Len() >= int(pkt.expectedLen) {
		return true
	} else {
		return false
	}
}

func (pkt *Packet) GetBytes() []byte {
	return pkt.buffer.Bytes()
}

func (pkt *Packet) TrimEnd(count int) {

	if pkt.buffer.Len() < count {
		count = pkt.buffer.Len()
	}
	pkt.buffer.Truncate(pkt.buffer.Len() - count)
}
