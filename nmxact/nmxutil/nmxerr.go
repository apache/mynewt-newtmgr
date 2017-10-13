/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package nmxutil

import (
	"fmt"
)

// Represents a application-layer timeout (e.g., NMP or CoAP); request sent,
// but no response received.
type RspTimeoutError struct {
	Text string
}

func NewRspTimeoutError(text string) *RspTimeoutError {
	return &RspTimeoutError{
		Text: text,
	}
}

func FmtRspTimeoutError(format string, args ...interface{}) *RspTimeoutError {
	return NewRspTimeoutError(fmt.Sprintf(format, args...))
}

func (e *RspTimeoutError) Error() string {
	return e.Text
}

func IsRspTimeout(err error) bool {
	_, ok := err.(*RspTimeoutError)
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

// Represents a BLE pairing failure due to missing or mismatched key material.
type BleSecurityError struct {
	Text string
}

func NewBleSecurityError(text string) *BleSecurityError {
	return &BleSecurityError{text}
}

func (err *BleSecurityError) Error() string {
	return err.Text
}

func IsBleSecurity(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*BleSecurityError)
	return ok
}

func ToBleSecurity(err error) *BleSecurityError {
	if berr, ok := err.(*BleSecurityError); ok {
		return berr
	} else {
		return nil
	}
}
