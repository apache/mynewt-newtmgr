package oic

import (
	"fmt"

	"github.com/runtimeco/go-coap"
)

func EncodeGet(resUri string, token []byte) ([]byte, error) {
	if len(token) > 8 {
		return nil,
			fmt.Errorf("Invalid token; len=%d, must be < 8", len(token))
	}

	req := &coap.TcpMessage{
		Message: coap.Message{
			Type:  coap.Confirmable,
			Code:  coap.GET,
			Token: token,
		},
	}
	req.SetPathString(resUri)

	b, err := req.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("Failed to encode CoAP: %s\n", err.Error())
	}

	return b, nil
}
