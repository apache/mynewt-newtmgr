package oic

import (
	"fmt"

	"github.com/runtimeco/go-coap"
)

type Server struct {
	rxer  Receiver
	rm    ResMgr
	isTcp bool
}

func NewServer(isTcp bool) Server {
	s := Server{
		rxer:  NewReceiver(isTcp),
		rm:    NewResMgr(),
		isTcp: isTcp,
	}

	return s
}

func (s *Server) AddResource(r Resource) error {
	return s.rm.Add(r)
}

// @return                      Response to send back, if any.
func (s *Server) Rx(data []byte) (coap.Message, error) {
	m := s.rxer.Rx(data)
	if m == nil {
		return nil, nil
	}

	var typ coap.COAPType
	switch m.Type() {
	case coap.Confirmable:
		typ = coap.Acknowledgement

	case coap.NonConfirmable:
		typ = coap.NonConfirmable

	default:
		return nil, fmt.Errorf("Don't know how to handle CoAP message with "+
			"type=%d (%s)", m.Type(), m.Type().String())
	}

	code, payload := s.rm.Access(m)

	p := coap.MessageParams{
		Type:      typ,
		Code:      code,
		MessageID: NextMessageId(),
		Token:     m.Token(),
		Payload:   payload,
	}

	if !s.isTcp {
		return coap.NewDgramMessage(p), nil
	} else {
		return coap.NewTcpMessage(p), nil
	}
}
