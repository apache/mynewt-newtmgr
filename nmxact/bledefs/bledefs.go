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

package bledefs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const BLE_ATT_ATTR_MAX_LEN = 512

const BLE_ATT_MTU_DFLT = 23

const CccdUuid = 0x2902

const IotivitySvcUuid = "ade3d529-c784-4f63-a987-eb69f70ee816"
const IotivityReqChrUuid = "ad7b334f-4637-4b86-90b6-9d787f03d218"
const IotivityRspChrUuid = "e9241982-4580-42c4-8831-95048216b256"

const UnauthSvcUuid = "0c08c213-98ed-4e43-a499-7e1137c39567"
const UnauthReqChrUuid = "69b8a928-2ab2-487b-923e-54ce53a18bc1"
const UnauthRspChrUuid = "bca10aea-5df1-4248-b72b-f52955ad9c88"

const SecureSvcUuid = 0xfe18
const SecureReqChrUuid = 0x1000
const SecureRspChrUuid = 0x1001

const NmpPlainSvcUuid = "8d53dc1d-1db7-4cd3-868b-8a527460aa84"
const NmpPlainChrUuid = "da2e7828-fbce-4e01-ae9e-261174997c48"

const OmpUnsecSvcUuid = "ade3d529-c784-4f63-a987-eb69f70ee816"
const OmpUnsecReqChrUuid = "ad7b334f-4637-4b86-90b6-9d787f03d218"
const OmpUnsecRspChrUuid = "e9241982-4580-42c4-8831-95048216b256"

const OmpSecSvcUuid = SecureSvcUuid
const OmpSecReqChrUuid = SecureReqChrUuid
const OmpSecRspChrUuid = SecureRspChrUuid

type BleAddrType int

const (
	BLE_ADDR_TYPE_PUBLIC  BleAddrType = 0
	BLE_ADDR_TYPE_RANDOM              = 1
	BLE_ADDR_TYPE_RPA_PUB             = 2
	BLE_ADDR_TYPE_RPA_RND             = 3
)

var BleAddrTypeStringMap = map[BleAddrType]string{
	BLE_ADDR_TYPE_PUBLIC:  "public",
	BLE_ADDR_TYPE_RANDOM:  "random",
	BLE_ADDR_TYPE_RPA_PUB: "rpa_pub",
	BLE_ADDR_TYPE_RPA_RND: "rpa_rnd",
}

func BleAddrTypeToString(addrType BleAddrType) string {
	s := BleAddrTypeStringMap[addrType]
	if s == "" {
		return "???"
	}

	return s
}

func BleAddrTypeFromString(s string) (BleAddrType, error) {
	for addrType, name := range BleAddrTypeStringMap {
		if s == name {
			return addrType, nil
		}
	}

	return BleAddrType(0), fmt.Errorf("Invalid BleAddrType string: %s", s)
}

func (a BleAddrType) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAddrTypeToString(a))
}

func (a *BleAddrType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAddrTypeFromString(s)
	return err
}

type BleAddr struct {
	Bytes [6]byte
}

func ParseBleAddr(s string) (BleAddr, error) {
	ba := BleAddr{}

	toks := strings.Split(strings.ToLower(s), ":")
	if len(toks) != 6 {
		return ba, fmt.Errorf("invalid BLE addr string: %s", s)
	}

	for i, t := range toks {
		u64, err := strconv.ParseUint(t, 16, 8)
		if err != nil {
			return ba, err
		}
		ba.Bytes[i] = byte(u64)
	}

	return ba, nil
}

func (ba *BleAddr) String() string {
	var buf bytes.Buffer
	buf.Grow(len(ba.Bytes) * 3)

	for i, b := range ba.Bytes {
		if i != 0 {
			buf.WriteString(":")
		}
		fmt.Fprintf(&buf, "%02x", b)
	}

	return buf.String()
}

func (ba *BleAddr) MarshalJSON() ([]byte, error) {
	return json.Marshal(ba.String())
}

func (ba *BleAddr) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var err error
	*ba, err = ParseBleAddr(s)
	if err != nil {
		return err
	}

	return nil
}

type BleDev struct {
	AddrType BleAddrType
	Addr     BleAddr
}

func (bd *BleDev) String() string {
	return fmt.Sprintf("%s,%s",
		BleAddrTypeToString(bd.AddrType),
		bd.Addr.String())
}

type BleUuid16 uint16

func (bu16 *BleUuid16) String() string {
	return fmt.Sprintf("0x%04x", *bu16)
}

func ParseUuid16(s string) (BleUuid16, error) {
	val, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return BleUuid16(0), fmt.Errorf("Invalid UUID: %s", s)
	}

	return BleUuid16(val), nil
}

type BleUuid128 [16]byte

func (bu128 *BleUuid128) String() string {
	var buf bytes.Buffer
	buf.Grow(len(bu128)*2 + 3)

	for i, b := range bu128 {
		switch i {
		case 4, 6, 8, 10:
			buf.WriteString("-")
		}

		fmt.Fprintf(&buf, "%02x", b)
	}

	return buf.String()
}

func ParseUuid128(s string) (BleUuid128, error) {
	var bu128 BleUuid128

	if len(s) != 36 {
		return bu128, fmt.Errorf("Invalid UUID: %s", s)
	}

	boff := 0
	for i := 0; i < 36; {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return bu128, fmt.Errorf("Invalid UUID: %s", s)
			}
			i++

		default:
			u64, err := strconv.ParseUint(s[i:i+2], 16, 8)
			if err != nil {
				return bu128, fmt.Errorf("Invalid UUID: %s", s)
			}
			bu128[boff] = byte(u64)
			i += 2
			boff++
		}
	}

	return bu128, nil
}

func (bu128 *BleUuid128) MarshalJSON() ([]byte, error) {
	return json.Marshal(bu128.String())
}

func (bu128 *BleUuid128) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var err error
	*bu128, err = ParseUuid128(s)
	if err != nil {
		return err
	}

	return nil
}

type BleUuid struct {
	// Set to 0 if the 128-bit UUID should be used.
	U16 BleUuid16

	U128 BleUuid128
}

func (bu *BleUuid) String() string {
	if bu.U16 != 0 {
		return bu.U16.String()
	} else {
		return bu.U128.String()
	}
}

func ParseUuid(uuidStr string) (BleUuid, error) {
	bu := BleUuid{}
	var err error

	// First, try to parse as a 16-bit UUID.
	bu.U16, err = ParseUuid16(uuidStr)
	if err == nil {
		return bu, nil
	}

	// Try to parse as a 128-bit UUID.
	bu.U128, err = ParseUuid128(uuidStr)
	if err == nil {
		return bu, nil
	}

	return bu, err
}

func NewBleUuid16(uuid16 uint16) BleUuid {
	return BleUuid{BleUuid16(uuid16), BleUuid128{}}
}

func (bu *BleUuid) MarshalJSON() ([]byte, error) {
	if bu.U16 != 0 {
		return json.Marshal(bu.U16)
	} else {
		return json.Marshal(bu.U128.String())
	}
}

func (bu *BleUuid) UnmarshalJSON(data []byte) error {
	var err error

	// If the value is a string, try to parse a UUID from it. */
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*bu, err = ParseUuid(s)
		return err
	}

	// Not a string; maybe it's a raw 16-bit number.
	if err = json.Unmarshal(data, &bu.U16); err != nil {
		return err
	}

	return nil
}

func CompareUuids(a BleUuid, b BleUuid) int {
	if a.U16 != 0 || b.U16 != 0 {
		return int(a.U16) - int(b.U16)
	} else {
		return bytes.Compare(a.U128[:], b.U128[:])
	}
}

type BleScanFilterPolicy int

const (
	BLE_SCAN_FILT_NO_WL        BleScanFilterPolicy = 0
	BLE_SCAN_FILT_USE_WL                           = 1
	BLE_SCAN_FILT_NO_WL_INITA                      = 2
	BLE_SCAN_FILT_USE_WL_INITA                     = 3
)

var BleScanFilterPolicyStringMap = map[BleScanFilterPolicy]string{
	BLE_SCAN_FILT_NO_WL:        "no_wl",
	BLE_SCAN_FILT_USE_WL:       "use_wl",
	BLE_SCAN_FILT_NO_WL_INITA:  "no_wl_inita",
	BLE_SCAN_FILT_USE_WL_INITA: "use_wl_inita",
}

func BleScanFilterPolicyToString(filtPolicy BleScanFilterPolicy) string {
	s := BleScanFilterPolicyStringMap[filtPolicy]
	if s == "" {
		return "???"
	}

	return s
}

func BleScanFilterPolicyFromString(s string) (BleScanFilterPolicy, error) {
	for filtPolicy, name := range BleScanFilterPolicyStringMap {
		if s == name {
			return filtPolicy, nil
		}
	}

	return BleScanFilterPolicy(0),
		fmt.Errorf("Invalid BleScanFilterPolicy string: %s", s)
}

func (a BleScanFilterPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleScanFilterPolicyToString(a))
}

func (a *BleScanFilterPolicy) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleScanFilterPolicyFromString(s)
	return err
}

type BleAdvEventType int

const (
	BLE_ADV_EVENT_IND           BleAdvEventType = 0
	BLE_ADV_EVENT_DIRECT_IND_HD                 = 1
	BLE_ADV_EVENT_SCAN_IND                      = 2
	BLE_ADV_EVENT_NONCONN_IND                   = 3
	BLE_ADV_EVENT_DIRECT_IND_LD                 = 4
)

var BleAdvEventTypeStringMap = map[BleAdvEventType]string{
	BLE_ADV_EVENT_IND:           "ind",
	BLE_ADV_EVENT_DIRECT_IND_HD: "direct_ind_hd",
	BLE_ADV_EVENT_SCAN_IND:      "scan_ind",
	BLE_ADV_EVENT_NONCONN_IND:   "nonconn_ind",
	BLE_ADV_EVENT_DIRECT_IND_LD: "direct_ind_ld",
}

func BleAdvEventTypeToString(advEventType BleAdvEventType) string {
	s := BleAdvEventTypeStringMap[advEventType]
	if s == "" {
		return "???"
	}

	return s
}

func BleAdvEventTypeFromString(s string) (BleAdvEventType, error) {
	for advEventType, name := range BleAdvEventTypeStringMap {
		if s == name {
			return advEventType, nil
		}
	}

	return BleAdvEventType(0),
		fmt.Errorf("Invalid BleAdvEventType string: %s", s)
}

func (a BleAdvEventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAdvEventTypeToString(a))
}

func (a *BleAdvEventType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAdvEventTypeFromString(s)
	return err
}

type BleAdvConnMode int

const (
	BLE_ADV_CONN_MODE_NON BleAdvConnMode = iota
	BLE_ADV_CONN_MODE_DIR
	BLE_ADV_CONN_MODE_UND
)

var BleAdvConnModeStringMap = map[BleAdvConnMode]string{
	BLE_ADV_CONN_MODE_NON: "non",
	BLE_ADV_CONN_MODE_DIR: "dir",
	BLE_ADV_CONN_MODE_UND: "und",
}

func BleAdvConnModeToString(connMode BleAdvConnMode) string {
	s := BleAdvConnModeStringMap[connMode]
	if s == "" {
		return "???"
	}

	return s
}

func BleAdvConnModeFromString(s string) (BleAdvConnMode, error) {
	for advConnMode, name := range BleAdvConnModeStringMap {
		if s == name {
			return advConnMode, nil
		}
	}

	return BleAdvConnMode(0),
		fmt.Errorf("Invalid BleAdvConnMode string: %s", s)
}

func (a BleAdvConnMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAdvConnModeToString(a))
}

func (a *BleAdvConnMode) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAdvConnModeFromString(s)
	return err
}

type BleAdvDiscMode int

const (
	BLE_ADV_DISC_MODE_NON BleAdvDiscMode = iota
	BLE_ADV_DISC_MODE_LTD
	BLE_ADV_DISC_MODE_GEN
)

var BleAdvDiscModeStringMap = map[BleAdvDiscMode]string{
	BLE_ADV_DISC_MODE_NON: "non",
	BLE_ADV_DISC_MODE_LTD: "ltd",
	BLE_ADV_DISC_MODE_GEN: "gen",
}

func BleAdvDiscModeToString(discMode BleAdvDiscMode) string {
	s := BleAdvDiscModeStringMap[discMode]
	if s == "" {
		return "???"
	}

	return s
}

func BleAdvDiscModeFromString(s string) (BleAdvDiscMode, error) {
	for advDiscMode, name := range BleAdvDiscModeStringMap {
		if s == name {
			return advDiscMode, nil
		}
	}

	return BleAdvDiscMode(0),
		fmt.Errorf("Invalid BleAdvDiscMode string: %s", s)
}

func (a BleAdvDiscMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAdvDiscModeToString(a))
}

func (a *BleAdvDiscMode) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAdvDiscModeFromString(s)
	return err
}

type BleAdvFilterPolicy int

const (
	BLE_ADV_FILTER_POLICY_NONE BleAdvFilterPolicy = iota
	BLE_ADV_FILTER_POLICY_SCAN
	BLE_ADV_FILTER_POLICY_CONN
	BLE_ADV_FILTER_POLICY_BOTH
)

var BleAdvFilterPolicyStringMap = map[BleAdvFilterPolicy]string{
	BLE_ADV_FILTER_POLICY_NONE: "none",
	BLE_ADV_FILTER_POLICY_SCAN: "scan",
	BLE_ADV_FILTER_POLICY_CONN: "conn",
	BLE_ADV_FILTER_POLICY_BOTH: "both",
}

func BleAdvFilterPolicyToString(discMode BleAdvFilterPolicy) string {
	s := BleAdvFilterPolicyStringMap[discMode]
	if s == "" {
		return "???"
	}

	return s
}

func BleAdvFilterPolicyFromString(s string) (BleAdvFilterPolicy, error) {
	for advFilterPolicy, name := range BleAdvFilterPolicyStringMap {
		if s == name {
			return advFilterPolicy, nil
		}
	}

	return BleAdvFilterPolicy(0),
		fmt.Errorf("Invalid BleAdvFilterPolicy string: %s", s)
}

func (a BleAdvFilterPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleAdvFilterPolicyToString(a))
}

func (a *BleAdvFilterPolicy) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleAdvFilterPolicyFromString(s)
	return err
}

type BleAdvFields struct {
	Data []byte

	// Each field is only present if the sender included it in its
	// advertisement.
	Flags              *uint8
	Uuids16            []BleUuid16
	Uuids16IsComplete  bool
	Uuids32            []uint32
	Uuids32IsComplete  bool
	Uuids128           []BleUuid128
	Uuids128IsComplete bool
	Name               *string
	NameIsComplete     bool
	TxPwrLvl           *int8
	SlaveItvlMin       *uint16
	SlaveItvlMax       *uint16
	SvcDataUuid16      []byte
	PublicTgtAddrs     []BleAddr
	Appearance         *uint16
	AdvItvl            *uint16
	SvcDataUuid32      []byte
	SvcDataUuid128     []byte
	Uri                *string
	MfgData            []byte
}

type BleAdvReport struct {
	// These fields are always present.
	EventType BleAdvEventType
	Sender    BleDev
	Rssi      int8

	Fields BleAdvFields
}

type BleAdvRptFn func(r BleAdvReport)
type BleAdvPredicate func(adv BleAdvReport) bool

type BleRole int

const (
	BLE_ROLE_MASTER BleRole = iota
	BLE_ROLE_SLAVE
)

type BleConnDesc struct {
	ConnHandle      uint16
	OwnIdAddrType   BleAddrType
	OwnIdAddr       BleAddr
	OwnOtaAddrType  BleAddrType
	OwnOtaAddr      BleAddr
	PeerIdAddrType  BleAddrType
	PeerIdAddr      BleAddr
	PeerOtaAddrType BleAddrType
	PeerOtaAddr     BleAddr
	Role            BleRole
	Encrypted       bool
	Authenticated   bool
	Bonded          bool
	KeySize         int
}

func (d *BleConnDesc) String() string {
	return fmt.Sprintf("conn_handle=%d "+
		"own_id_addr=%s,%s own_ota_addr=%s,%s "+
		"peer_id_addr=%s,%s peer_ota_addr=%s,%s",
		d.ConnHandle,
		BleAddrTypeToString(d.OwnIdAddrType),
		d.OwnIdAddr.String(),
		BleAddrTypeToString(d.OwnOtaAddrType),
		d.OwnOtaAddr.String(),
		BleAddrTypeToString(d.PeerIdAddrType),
		d.PeerIdAddr.String(),
		BleAddrTypeToString(d.PeerOtaAddrType),
		d.PeerOtaAddr.String())
}

type BleEncryptWhen int

const (
	BLE_ENCRYPT_NEVER BleEncryptWhen = iota
	BLE_ENCRYPT_AS_REQD
	BLE_ENCRYPT_ALWAYS
)

type BleGattOp int

const (
	BLE_GATT_ACCESS_OP_READ_CHR  BleGattOp = 0
	BLE_GATT_ACCESS_OP_WRITE_CHR           = 1
	BLE_GATT_ACCESS_OP_READ_DSC            = 2
	BLE_GATT_ACCESS_OP_WRITE_DSC           = 3
)

var BleGattOpStringMap = map[BleGattOp]string{
	BLE_GATT_ACCESS_OP_READ_CHR:  "read_chr",
	BLE_GATT_ACCESS_OP_WRITE_CHR: "write_chr",
	BLE_GATT_ACCESS_OP_READ_DSC:  "read_dsc",
	BLE_GATT_ACCESS_OP_WRITE_DSC: "write_dsc",
}

func BleGattOpToString(op BleGattOp) string {
	s := BleGattOpStringMap[op]
	if s == "" {
		return "???"
	}

	return s
}

func BleGattOpFromString(s string) (BleGattOp, error) {
	for op, name := range BleGattOpStringMap {
		if s == name {
			return op, nil
		}
	}

	return BleGattOp(0),
		fmt.Errorf("Invalid BleGattOp string: %s", s)
}

type BleSvcType int

const (
	BLE_SVC_TYPE_PRIMARY BleSvcType = iota
	BLE_SVC_TYPE_SECONDARY
)

var BleSvcTypeStringMap = map[BleSvcType]string{
	BLE_SVC_TYPE_PRIMARY:   "primary",
	BLE_SVC_TYPE_SECONDARY: "secondary",
}

func BleSvcTypeToString(svcType BleSvcType) string {
	s := BleSvcTypeStringMap[svcType]
	if s == "" {
		return "???"
	}

	return s
}

func BleSvcTypeFromString(s string) (BleSvcType, error) {
	for svcType, name := range BleSvcTypeStringMap {
		if s == name {
			return svcType, nil
		}
	}

	return BleSvcType(0),
		fmt.Errorf("Invalid BleSvcType string: %s", s)
}

func (a BleSvcType) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleSvcTypeToString(a))
}

func (a *BleSvcType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleSvcTypeFromString(s)
	return err
}

type BleChrFlags int

const (
	BLE_GATT_F_BROADCAST       BleChrFlags = 0x0001
	BLE_GATT_F_READ                        = 0x0002
	BLE_GATT_F_WRITE_NO_RSP                = 0x0004
	BLE_GATT_F_WRITE                       = 0x0008
	BLE_GATT_F_NOTIFY                      = 0x0010
	BLE_GATT_F_INDICATE                    = 0x0020
	BLE_GATT_F_AUTH_SIGN_WRITE             = 0x0040
	BLE_GATT_F_RELIABLE_WRITE              = 0x0080
	BLE_GATT_F_AUX_WRITE                   = 0x0100
	BLE_GATT_F_READ_ENC                    = 0x0200
	BLE_GATT_F_READ_AUTHEN                 = 0x0400
	BLE_GATT_F_READ_AUTHOR                 = 0x0800
	BLE_GATT_F_WRITE_ENC                   = 0x1000
	BLE_GATT_F_WRITE_AUTHEN                = 0x2000
	BLE_GATT_F_WRITE_AUTHOR                = 0x4000
)

type BleAttFlags int

const (
	BLE_ATT_F_READ         BleAttFlags = 0x01
	BLE_ATT_F_WRITE                    = 0x02
	BLE_ATT_F_READ_ENC                 = 0x04
	BLE_ATT_F_READ_AUTHEN              = 0x08
	BLE_ATT_F_READ_AUTHOR              = 0x10
	BLE_ATT_F_WRITE_ENC                = 0x20
	BLE_ATT_F_WRITE_AUTHEN             = 0x40
	BLE_ATT_F_WRITE_AUTHOR             = 0x80
)

type BleDiscChrProperties int

const (
	BLE_DISC_CHR_PROP_BROADCAST       BleDiscChrProperties = 0x01
	BLE_DISC_CHR_PROP_READ                                 = 0x02
	BLE_DISC_CHR_PROP_WRITE_NO_RSP                         = 0x04
	BLE_DISC_CHR_PROP_WRITE                                = 0x08
	BLE_DISC_CHR_PROP_NOTIFY                               = 0x10
	BLE_DISC_CHR_PROP_INDICATE                             = 0x20
	BLE_DISC_CHR_PROP_AUTH_SIGN_WRITE                      = 0x40
	BLE_DISC_CHR_PROP_EXTENDED                             = 0x80
)

type BleGattAccess struct {
	Op         BleGattOp
	ConnHandle uint16
	SvcUuid    BleUuid
	ChrUuid    BleUuid
	Data       []byte
}

type BleGattAccessFn func(access BleGattAccess) (uint8, []byte)

type BleDsc struct {
	Uuid       BleUuid
	AttFlags   BleAttFlags
	MinKeySize int
}

type BleChr struct {
	Uuid       BleUuid
	Flags      BleChrFlags
	MinKeySize int
	AccessCb   BleGattAccessFn
	Dscs       []BleDsc
}

type BleSvc struct {
	Uuid    BleUuid
	SvcType BleSvcType
	Chrs    []BleChr
}

type BleChrId struct {
	SvcUuid BleUuid
	ChrUuid BleUuid
}

func (b *BleChrId) String() string {
	return fmt.Sprintf("s=%s c=%s", b.SvcUuid.String(), b.ChrUuid.String())
}

func CompareChrIds(a BleChrId, b BleChrId) int {
	if rc := CompareUuids(a.SvcUuid, b.SvcUuid); rc != 0 {
		return rc
	}
	if rc := CompareUuids(a.ChrUuid, b.ChrUuid); rc != 0 {
		return rc
	}
	return 0
}

type BleMgmtChrs struct {
	NmpReqChr       *BleChrId
	NmpRspChr       *BleChrId
	ResPublicReqChr *BleChrId
	ResPublicRspChr *BleChrId
	ResUnauthReqChr *BleChrId
	ResUnauthRspChr *BleChrId
	ResSecureReqChr *BleChrId
	ResSecureRspChr *BleChrId
}

type BleSmAction int

const (
	BLE_SM_ACTION_OOB BleSmAction = iota
	BLE_SM_ACTION_INPUT
	BLE_SM_ACTION_DISP
	BLE_SM_ACTION_NUMCMP
)

var BleSmActionStringMap = map[BleSmAction]string{
	BLE_SM_ACTION_OOB:    "oob",
	BLE_SM_ACTION_INPUT:  "input",
	BLE_SM_ACTION_DISP:   "disp",
	BLE_SM_ACTION_NUMCMP: "numcmp",
}

func BleSmActionToString(smAction BleSmAction) string {
	s := BleSmActionStringMap[smAction]
	if s == "" {
		return "???"
	}

	return s
}

func BleSmActionFromString(s string) (BleSmAction, error) {
	for smAction, name := range BleSmActionStringMap {
		if s == name {
			return smAction, nil
		}
	}

	return BleSmAction(0),
		fmt.Errorf("Invalid BleSmAction string: %s", s)
}

func (io BleSmAction) String() string {
	return BleSmActionToString(io)
}

func (a BleSmAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleSmActionToString(a))
}

func (a *BleSmAction) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleSmActionFromString(s)
	return err
}

type BleSmIoCap int

const (
	BLE_SM_IO_CAP_DISP_ONLY BleSmIoCap = iota
	BLE_SM_IO_CAP_DISP_YES_NO
	BLE_SM_IO_CAP_KEYBOARD_ONLY
	BLE_SM_IO_CAP_NO_IO
	BLE_SM_IO_CAP_KEYBOARD_DISP
)

var BleSmIoCapStringMap = map[BleSmIoCap]string{
	BLE_SM_IO_CAP_DISP_ONLY:     "disp_only",
	BLE_SM_IO_CAP_DISP_YES_NO:   "disp_yes_no",
	BLE_SM_IO_CAP_KEYBOARD_ONLY: "keyboard_only",
	BLE_SM_IO_CAP_NO_IO:         "no_io",
	BLE_SM_IO_CAP_KEYBOARD_DISP: "keyboard_disp",
}

func BleSmIoCapToString(smAction BleSmIoCap) string {
	s := BleSmIoCapStringMap[smAction]
	if s == "" {
		return "???"
	}

	return s
}

func BleSmIoCapFromString(s string) (BleSmIoCap, error) {
	for smAction, name := range BleSmIoCapStringMap {
		if s == name {
			return smAction, nil
		}
	}

	return BleSmIoCap(0),
		fmt.Errorf("Invalid BleSmIoCap string: %s", s)
}

func (io BleSmIoCap) String() string {
	return BleSmIoCapToString(io)
}

func (a BleSmIoCap) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleSmIoCapToString(a))
}

func (a *BleSmIoCap) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleSmIoCapFromString(s)
	return err
}

type BleSmKeyDist int

const (
	BLE_SM_KEY_DIST_ENC BleSmKeyDist = iota
	BLE_SM_KEY_DIST_ID
	BLE_SM_KEY_DIST_SIGN
	BLE_SM_KEY_DIST_LINK
)

var BleSmKeyDistStringMap = map[BleSmKeyDist]string{
	BLE_SM_KEY_DIST_ENC:  "enc",
	BLE_SM_KEY_DIST_ID:   "id",
	BLE_SM_KEY_DIST_SIGN: "sign",
	BLE_SM_KEY_DIST_LINK: "link",
}

func BleSmKeyDistToString(smAction BleSmKeyDist) string {
	s := BleSmKeyDistStringMap[smAction]
	if s == "" {
		return "???"
	}

	return s
}

func BleSmKeyDistFromString(s string) (BleSmKeyDist, error) {
	for smAction, name := range BleSmKeyDistStringMap {
		if s == name {
			return smAction, nil
		}
	}

	return BleSmKeyDist(0),
		fmt.Errorf("Invalid BleSmKeyDist string: %s", s)
}

func (io BleSmKeyDist) String() string {
	return BleSmKeyDistToString(io)
}

func (a BleSmKeyDist) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleSmKeyDistToString(a))
}

func (a *BleSmKeyDist) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleSmKeyDistFromString(s)
	return err
}

type BleSmAuthReq int

const (
	BLE_SM_AUTHREQ_BOND BleSmAuthReq = iota
	BLE_SM_AUTHREQ_MITM
	BLE_SM_AUTHREQ_SC
	BLE_SM_AUTHREQ_KEYPRESS
)

var BleSmAuthReqStringMap = map[BleSmAuthReq]string{
	BLE_SM_AUTHREQ_BOND:     "bond",
	BLE_SM_AUTHREQ_MITM:     "mitm",
	BLE_SM_AUTHREQ_SC:       "sc",
	BLE_SM_AUTHREQ_KEYPRESS: "keypress",
}

func BleSmAuthReqToString(smAction BleSmAuthReq) string {
	s := BleSmAuthReqStringMap[smAction]
	if s == "" {
		return "???"
	}

	return s
}

func BleSmAuthReqFromString(s string) (BleSmAuthReq, error) {
	for smAction, name := range BleSmAuthReqStringMap {
		if s == name {
			return smAction, nil
		}
	}

	return BleSmAuthReq(0),
		fmt.Errorf("Invalid BleSmAuthReq string: %s", s)
}

func (io BleSmAuthReq) String() string {
	return BleSmAuthReqToString(io)
}

func (a BleSmAuthReq) MarshalJSON() ([]byte, error) {
	return json.Marshal(BleSmAuthReqToString(a))
}

func (a *BleSmAuthReq) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = BleSmAuthReqFromString(s)
	return err
}

type BlePairCfg struct {
	IoCap        BleSmIoCap
	Oob          bool
	Bonding      bool
	Mitm         bool
	Sc           bool
	Keypress     bool
	OurKeyDist   BleSmKeyDist
	TheirKeyDist BleSmKeyDist
}
