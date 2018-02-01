package ble

import "testing"

func TestNewAddr(t *testing.T) {
	a := NewAddr("TeSt")

	if a.String() != "test" {
		t.Error("address should be \"test\" but is ", a.String())
	}
}
