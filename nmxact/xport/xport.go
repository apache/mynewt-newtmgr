package xport

import ()

type RxFn func(data []byte)

type Xport interface {
	Start() error
	Stop() error

	Tx(data []byte) error
}
