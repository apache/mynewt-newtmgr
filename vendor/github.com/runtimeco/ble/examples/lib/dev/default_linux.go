package dev

import (
	"github.com/runtimeco/ble"
	"github.com/runtimeco/ble/linux"
)

// DefaultDevice ...
func DefaultDevice() (d ble.Device, err error) {
	return linux.NewDevice()
}
