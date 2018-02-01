package darwin

import "github.com/go-ble/ble"

func uuidSlice(uu []ble.UUID) [][]byte {
	us := [][]byte{}
	for _, u := range uu {
		us = append(us, ble.Reverse(u))
	}
	return us
}
