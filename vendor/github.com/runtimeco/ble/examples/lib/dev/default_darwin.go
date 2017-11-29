package dev

import (
	"github.com/runtimeco/ble"
	"github.com/runtimeco/ble/darwin"
)

// DefaultDevice ...
func DefaultDevice() (d ble.Device, err error) {
	return darwin.NewDevice()
}
