package nmxutil

import (
	"fmt"
)

// Represents an NMP timeout; request sent, but no response received.
type NmpTimeoutError struct {
	Text string
}

func NewNmpTimeoutError(text string) *NmpTimeoutError {
	return &NmpTimeoutError{
		Text: text,
	}
}

func FmtNmpTimeoutError(format string, args ...interface{}) *NmpTimeoutError {
	return NewNmpTimeoutError(fmt.Sprintf(format, args...))
}

func (e *NmpTimeoutError) Error() string {
	return e.Text
}

func IsNmpTimeout(err error) bool {
	_, ok := err.(*NmpTimeoutError)
	return ok
}

type SesnDisconnectError struct {
	Text string
}

func NewSesnDisconnectError(text string) *SesnDisconnectError {
	return &SesnDisconnectError{
		Text: text,
	}
}

func FmtSesnDisconnectError(format string,
	args ...interface{}) *SesnDisconnectError {

	return NewSesnDisconnectError(fmt.Sprintf(format, args...))
}

func (e *SesnDisconnectError) Error() string {
	return e.Text
}

func IsSesnDisconnect(err error) bool {
	_, ok := err.(*SesnDisconnectError)
	return ok
}

// Represents a low-level transport error.
type XportError struct {
	Text string
}

func NewXportError(text string) *XportError {
	return &XportError{text}
}

func (e *XportError) Error() string {
	return e.Text
}

func IsXport(err error) bool {
	_, ok := err.(*XportError)
	return ok
}

type XportTimeoutError struct {
	Text string
}

func NewXportTimeoutError(text string) *XportTimeoutError {
	return &XportTimeoutError{text}
}

func (e *XportTimeoutError) Error() string {
	return e.Text
}

func IsXportTimeout(err error) bool {
	_, ok := err.(*XportTimeoutError)
	return ok
}

type BleHostError struct {
	Text   string
	Status int
}

func NewBleHostError(status int, text string) *BleHostError {
	return &BleHostError{
		Status: status,
		Text:   text,
	}
}

func FmtBleHostError(status int, format string,
	args ...interface{}) *BleHostError {

	return NewBleHostError(status, fmt.Sprintf(format, args...))
}

func (e *BleHostError) Error() string {
	return e.Text
}

func IsBleHost(err error) bool {
	_, ok := err.(*BleHostError)
	return ok
}

func ToBleHost(err error) *BleHostError {
	if berr, ok := err.(*BleHostError); ok {
		return berr
	} else {
		return nil
	}
}
