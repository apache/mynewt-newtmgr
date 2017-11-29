package dev

import "github.com/runtimeco/ble"

// NewDevice ...
func NewDevice(impl string) (d ble.Device, err error) {
	return DefaultDevice()
}
