package oic

import (
	log "github.com/Sirupsen/logrus"
	"github.com/runtimeco/go-coap"
)

type Receiver struct {
	reassembler *Reassembler
}

func NewReceiver(isTcp bool) Receiver {
	r := Receiver{}

	if isTcp {
		r.reassembler = NewReassembler()
	}

	return r
}

func (r *Receiver) Rx(data []byte) coap.Message {
	if r.reassembler != nil {
		// TCP.
		return r.reassembler.RxFrag(data)
	} else {
		// UDP.
		m, err := coap.ParseDgramMessage(data)
		if err != nil {
			log.Printf("CoAP parse failure: %s", err.Error())
			return nil
		}

		return m
	}
}
