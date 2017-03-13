package coap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
)

// TcpMessage is a CoAP Message that can encode itself for TCP
// transport.
type TcpMessage struct {
	Message
}

func (m *TcpMessage) MarshalBinary() ([]byte, error) {
	/*
	   A CoAP TCP message looks like:

	        0                   1                   2                   3
	       0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |  Len  |  TKL  | Extended Length ...
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |      Code     | TKL bytes ...
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |   Options (if any) ...
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	      |1 1 1 1 1 1 1 1|    Payload (if any) ...
	      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	*/

	buf := bytes.Buffer{}

	sort.Stable(&m.Message.opts)
	writeOpts(&buf, m.Message.opts)

	if len(m.Message.Payload) > 0 {
		buf.Write([]byte{0xff})
		buf.Write(m.Message.Payload)
	}

	var lenNib uint8
	var extLenBytes []byte

	if buf.Len() < 13 {
		lenNib = uint8(buf.Len())
	} else if buf.Len() < 269 {
		lenNib = 13
		extLen := buf.Len() - 13
		extLenBytes = []byte{uint8(extLen)}
	} else if buf.Len() < 65805 {
		lenNib = 14
		extLen := buf.Len() - 269
		extLenBytes = make([]byte, 2)
		binary.BigEndian.PutUint16(extLenBytes, uint16(extLen))
	} else {
		lenNib = 15
		extLen := buf.Len() - 65805
		extLenBytes = make([]byte, 4)
		binary.BigEndian.PutUint32(extLenBytes, uint32(extLen))
	}

	hdr := make([]byte, 1+len(extLenBytes)+len(m.Message.Token)+1)
	hdrOff := 0

	// Length and TKL nibbles.
	hdr[hdrOff] = uint8(0xf&len(m.Token)) | (lenNib << 4)
	hdrOff++

	// Extended length, if present.
	if len(extLenBytes) > 0 {
		copy(hdr[hdrOff:hdrOff+len(extLenBytes)], extLenBytes)
		hdrOff += len(extLenBytes)
	}

	// Code.
	hdr[hdrOff] = byte(m.Message.Code)
	hdrOff++

	// Token.
	if len(m.Message.Token) > 0 {
		copy(hdr[hdrOff:hdrOff+len(m.Message.Token)], m.Message.Token)
		hdrOff += len(m.Message.Token)
	}

	return append(hdr, buf.Bytes()...), nil
}

type msgTcpInfo struct {
	tkl       uint8
	code      uint8
	messageId uint16
	totLen    int
	tokenOff  int
}

func parseTcpMsgInfo(data []byte) (msgTcpInfo, bool) {
	mti := msgTcpInfo{}

	if len(data) < 1 {
		return mti, false
	}

	hdrOff := 0

	firstByte := data[hdrOff]
	hdrOff++

	lenNib := (firstByte & 0xf0) >> 4
	mti.tkl = firstByte & 0x0f

	var opLen int
	if lenNib < 13 {
		opLen = int(lenNib)
	} else if lenNib == 13 {
		if len(data) < hdrOff+1 {
			return mti, false
		}
		extLen := uint32(data[hdrOff])
		hdrOff++

		opLen = 13 + int(extLen)
	} else if lenNib == 14 {
		if len(data) < hdrOff+2 {
			return mti, false
		}
		extLen := binary.BigEndian.Uint16(data[hdrOff : hdrOff+2])
		hdrOff += 2

		opLen = 269 + int(extLen)
	} else if lenNib == 15 {
		if len(data) < hdrOff+4 {
			return mti, false
		}
		extLen := binary.BigEndian.Uint32(data[hdrOff : hdrOff+4])
		hdrOff += 4

		opLen = 65805 + int(extLen)
	}

	mti.totLen = hdrOff + 1 + int(mti.tkl) + opLen
	if len(data) < mti.totLen-opLen {
		return mti, false
	}

	mti.code = data[hdrOff]
	hdrOff++

	mti.tokenOff = hdrOff

	return mti, true
}

func (m *TcpMessage) UnmarshalBinary(data []byte) error {
	mti, ok := parseTcpMsgInfo(data)
	if !ok {
		return errors.New("short packet")
	}

	if len(data) != mti.totLen {
		return fmt.Errorf("CoAP length mismatch (hdr=%d pkt=%d)",
			mti.totLen, len(data))
	}

	if mti.tkl > 0 {
		m.Token = make([]byte, mti.tkl)
		copy(m.Message.Token, data[mti.tokenOff:mti.tokenOff+int(mti.tkl)])
	}

	p, o, err := parseOpts(data[mti.tokenOff+int(mti.tkl):])
	if err != nil {
		return err
	}

	m.Code = COAPCode(mti.code)
	m.MessageID = mti.messageId
	m.Payload = p
	m.opts = o

	return nil
}

func PullTcp(data []byte) (*TcpMessage, []byte, error) {
	mti, ok := parseTcpMsgInfo(data)
	if !ok {
		return nil, data, nil
	}

	if len(data) < mti.totLen {
		return nil, data, nil
	}

	mbytes := data[:mti.totLen]
	nextData := data[mti.totLen:]

	m := &TcpMessage{}
	err := m.UnmarshalBinary(mbytes)
	return m, nextData, err
}

// Decode reads a single message from its input.
func Decode(r io.Reader) (*TcpMessage, error) {
	var ln uint16
	err := binary.Read(r, binary.BigEndian, &ln)
	if err != nil {
		return nil, err
	}

	packet := make([]byte, ln)
	_, err = io.ReadFull(r, packet)
	if err != nil {
		return nil, err
	}

	m := TcpMessage{}

	err = m.UnmarshalBinary(packet)
	return &m, err
}
