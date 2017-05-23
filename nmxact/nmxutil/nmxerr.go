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

type BleSesnDisconnectError struct {
	Text   string
	Reason int
}

func NewBleSesnDisconnectError(reason int,
	text string) *BleSesnDisconnectError {

	return &BleSesnDisconnectError{
		Reason: reason,
		Text:   text,
	}
}

func (e *BleSesnDisconnectError) Error() string {
	return e.Text
}

func IsBleSesnDisconnect(err error) bool {
	_, ok := err.(*BleSesnDisconnectError)
	return ok
}

type SesnAlreadyOpenError struct {
	Text string
}

func NewSesnAlreadyOpenError(text string) *SesnAlreadyOpenError {
	return &SesnAlreadyOpenError{
		Text: text,
	}
}

func (e *SesnAlreadyOpenError) Error() string {
	return e.Text
}

func IsSesnAlreadyOpen(err error) bool {
	_, ok := err.(*SesnAlreadyOpenError)
	return ok
}

type SesnClosedError struct {
	Text string
}

func NewSesnClosedError(text string) *SesnClosedError {
	return &SesnClosedError{
		Text: text,
	}
}

func (e *SesnClosedError) Error() string {
	return e.Text
}

func IsSesnClosed(err error) bool {
	_, ok := err.(*SesnClosedError)
	return ok
}

type ScanTmoError struct {
	Text string
}

func NewScanTmoError(text string) *ScanTmoError {
	return &ScanTmoError{
		Text: text,
	}
}

func (e *ScanTmoError) Error() string {
	return e.Text
}

func IsScanTmo(err error) bool {
	_, ok := err.(*ScanTmoError)
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
	if err == nil {
		return false
	}

	_, ok := err.(*XportError)
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

// Indicates an attempt to transition to the already-current state.
type AlreadyError struct {
	Text string
}

func NewAlreadyError(text string) *AlreadyError {
	return &AlreadyError{text}
}

func (err *AlreadyError) Error() string {
	return err.Text
}

func IsAlready(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*AlreadyError)
	return ok
}
