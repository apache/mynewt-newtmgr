package xport

import ()

type RxFn func(data []byte)

type Xport interface {
	Start() error
	Stop() error

	Tx(data []byte) error
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

type TimeoutError struct {
	Text string
}

func NewTimeoutError(text string) *TimeoutError {
	return &TimeoutError{text}
}

func (e *TimeoutError) Error() string {
	return e.Text
}

func IsTimeout(err error) bool {
	_, ok := err.(*TimeoutError)
	return ok
}
