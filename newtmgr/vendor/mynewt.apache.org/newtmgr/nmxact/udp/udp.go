package udp

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
)

const MAX_PACKET_SIZE = 512

func Listen(peerString string, dispatchCb func(data []byte)) (
	*net.UDPConn, *net.UDPAddr, error) {

	addr, err := net.ResolveUDPAddr("udp", peerString)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failure resolving name for UDP session: %s",
				err.Error())
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failed to listen for UDP responses: %s", err.Error())
	}

	go func() {
		data := make([]byte, MAX_PACKET_SIZE)

		for {
			nr, srcAddr, err := conn.ReadFromUDP(data)
			if err != nil {
				// Connection closed or read error.
				return
			}

			log.Debugf("Received message from %v %d", srcAddr, nr)
			dispatchCb(data[0:nr])
		}
	}()

	return conn, addr, nil
}
