package nmble

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type AddrType int
type MsgOp int
type MsgType int

type BleBytes struct {
	Bytes []byte
}

type BleAddr struct {
	Bytes [6]byte
}

type BleUuid struct {
	Bytes [16]byte
}

const (
	BLE_HS_EAGAIN       int = 1
	BLE_HS_EALREADY         = 2
	BLE_HS_EINVAL           = 3
	BLE_HS_EMSGSIZE         = 4
	BLE_HS_ENOENT           = 5
	BLE_HS_ENOMEM           = 6
	BLE_HS_ENOTCONN         = 7
	BLE_HS_ENOTSUP          = 8
	BLE_HS_EAPP             = 9
	BLE_HS_EBADDATA         = 10
	BLE_HS_EOS              = 11
	BLE_HS_ECONTROLLER      = 12
	BLE_HS_ETIMEOUT         = 13
	BLE_HS_EDONE            = 14
	BLE_HS_EBUSY            = 15
	BLE_HS_EREJECT          = 16
	BLE_HS_EUNKNOWN         = 17
	BLE_HS_EROLE            = 18
	BLE_HS_ETIMEOUT_HCI     = 19
	BLE_HS_ENOMEM_EVT       = 20
	BLE_HS_ENOADDR          = 21
	BLE_HS_ENOTSYNCED       = 22
)

const (
	BLE_ERR_UNKNOWN_HCI_CMD     int = 1
	BLE_ERR_UNK_CONN_ID             = 2
	BLE_ERR_HW_FAIL                 = 3
	BLE_ERR_PAGE_TMO                = 4
	BLE_ERR_AUTH_FAIL               = 5
	BLE_ERR_PINKEY_MISSING          = 6
	BLE_ERR_MEM_CAPACITY            = 7
	BLE_ERR_CONN_SPVN_TMO           = 8
	BLE_ERR_CONN_LIMIT              = 9
	BLE_ERR_SYNCH_CONN_LIMIT        = 10
	BLE_ERR_ACL_CONN_EXISTS         = 11
	BLE_ERR_CMD_DISALLOWED          = 12
	BLE_ERR_CONN_REJ_RESOURCES      = 13
	BLE_ERR_CONN_REJ_SECURITY       = 14
	BLE_ERR_CONN_REJ_BD_ADDR        = 15
	BLE_ERR_CONN_ACCEPT_TMO         = 16
	BLE_ERR_UNSUPPORTED             = 17
	BLE_ERR_INV_HCI_CMD_PARMS       = 18
	BLE_ERR_REM_USER_CONN_TERM      = 19
	BLE_ERR_RD_CONN_TERM_RESRCS     = 20
	BLE_ERR_RD_CONN_TERM_PWROFF     = 21
	BLE_ERR_CONN_TERM_LOCAL         = 22
	BLE_ERR_REPEATED_ATTEMPTS       = 23
	BLE_ERR_NO_PAIRING              = 24
	BLE_ERR_UNK_LMP                 = 25
	BLE_ERR_UNSUPP_REM_FEATURE      = 26
	BLE_ERR_SCO_OFFSET              = 27
	BLE_ERR_SCO_ITVL                = 28
	BLE_ERR_SCO_AIR_MODE            = 29
	BLE_ERR_INV_LMP_LL_PARM         = 30
	BLE_ERR_UNSPECIFIED             = 31
	BLE_ERR_UNSUPP_LMP_LL_PARM      = 32
	BLE_ERR_NO_ROLE_CHANGE          = 33
	BLE_ERR_LMP_LL_RSP_TMO          = 34
	BLE_ERR_LMP_COLLISION           = 35
	BLE_ERR_LMP_PDU                 = 36
	BLE_ERR_ENCRYPTION_MODE         = 37
	BLE_ERR_LINK_KEY_CHANGE         = 38
	BLE_ERR_UNSUPP_QOS              = 39
	BLE_ERR_INSTANT_PASSED          = 40
	BLE_ERR_UNIT_KEY_PAIRING        = 41
	BLE_ERR_DIFF_TRANS_COLL         = 42
	BLE_ERR_QOS_PARM                = 44
	BLE_ERR_QOS_REJECTED            = 45
	BLE_ERR_CHAN_CLASS              = 46
	BLE_ERR_INSUFFICIENT_SEC        = 47
	BLE_ERR_PARM_OUT_OF_RANGE       = 48
	BLE_ERR_PENDING_ROLE_SW         = 50
	BLE_ERR_RESERVED_SLOT           = 52
	BLE_ERR_ROLE_SW_FAIL            = 53
	BLE_ERR_INQ_RSP_TOO_BIG         = 54
	BLE_ERR_SEC_SIMPLE_PAIR         = 55
	BLE_ERR_HOST_BUSY_PAIR          = 56
	BLE_ERR_CONN_REJ_CHANNEL        = 57
	BLE_ERR_CTLR_BUSY               = 58
	BLE_ERR_CONN_PARMS              = 59
	BLE_ERR_DIR_ADV_TMO             = 60
	BLE_ERR_CONN_TERM_MIC           = 61
	BLE_ERR_CONN_ESTABLISHMENT      = 62
	BLE_ERR_MAC_CONN_FAIL           = 63
	BLE_ERR_COARSE_CLK_ADJ          = 64
)

const (
	ADDR_TYPE_PUBLIC  AddrType = 0
	ADDR_TYPE_RANDOM           = 1
	ADDR_TYPE_RPA_PUB          = 2
	ADDR_TYPE_RPA_RND          = 3
)

const (
	MSG_OP_REQ MsgOp = 0
	MSG_OP_RSP       = 1
	MSG_OP_EVT       = 2
)

const (
	MSG_TYPE_ERR           MsgType = 1
	MSG_TYPE_SYNC                  = 2
	MSG_TYPE_CONNECT               = 3
	MSG_TYPE_TERMINATE             = 4
	MSG_TYPE_DISC_ALL_SVCS         = 5
	MSG_TYPE_DISC_SVC_UUID         = 6
	MSG_TYPE_DISC_ALL_CHRS         = 7
	MSG_TYPE_DISC_CHR_UUID         = 8
	MSG_TYPE_WRITE                 = 9
	MSG_TYPE_WRITE_CMD             = 10
	MSG_TYPE_EXCHANGE_MTU          = 11
	MSG_TYPE_GEN_RAND_ADDR         = 12
	MSG_TYPE_SET_RAND_ADDR         = 13
	MSG_TYPE_CONN_CANCEL           = 14

	MSG_TYPE_SYNC_EVT       = 2049
	MSG_TYPE_CONNECT_EVT    = 2050
	MSG_TYPE_DISCONNECT_EVT = 2051
	MSG_TYPE_DISC_SVC_EVT   = 2052
	MSG_TYPE_DISC_CHR_EVT   = 2053
	MSG_TYPE_WRITE_ACK_EVT  = 2054
	MSG_TYPE_NOTIFY_RX_EVT  = 2055
	MSG_TYPE_MTU_CHANGE_EVT = 2056
)

var AddrTypeStringMap = map[AddrType]string{
	ADDR_TYPE_PUBLIC:  "public",
	ADDR_TYPE_RANDOM:  "random",
	ADDR_TYPE_RPA_PUB: "rpa_pub",
	ADDR_TYPE_RPA_RND: "rpa_rnd",
}

var MsgOpStringMap = map[MsgOp]string{
	MSG_OP_REQ: "request",
	MSG_OP_RSP: "response",
	MSG_OP_EVT: "event",
}

var MsgTypeStringMap = map[MsgType]string{
	MSG_TYPE_ERR:           "error",
	MSG_TYPE_SYNC:          "sync",
	MSG_TYPE_CONNECT:       "connect",
	MSG_TYPE_TERMINATE:     "terminate",
	MSG_TYPE_DISC_SVC_UUID: "disc_svc_uuid",
	MSG_TYPE_DISC_CHR_UUID: "disc_chr_uuid",
	MSG_TYPE_DISC_ALL_CHRS: "disc_all_chrs",
	MSG_TYPE_WRITE_CMD:     "write_cmd",
	MSG_TYPE_EXCHANGE_MTU:  "exchange_mtu",
	MSG_TYPE_CONN_CANCEL:   "conn_cancel",

	MSG_TYPE_SYNC_EVT:       "sync_evt",
	MSG_TYPE_CONNECT_EVT:    "connect_evt",
	MSG_TYPE_DISCONNECT_EVT: "disconnect_evt",
	MSG_TYPE_DISC_SVC_EVT:   "disc_svc_evt",
	MSG_TYPE_DISC_CHR_EVT:   "disc_chr_evt",
	MSG_TYPE_NOTIFY_RX_EVT:  "notify_rx_evt",
	MSG_TYPE_MTU_CHANGE_EVT: "mtu_change_evt",
}

type BleHdr struct {
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`
}

type BleMsg interface{}

type BleSvc struct {
	StartHandle int     `json:"start_handle"`
	EndHandle   int     `json:"end_handle"`
	Uuid        BleUuid `json:"uuid"`
}

type BleChr struct {
	DefHandle  int     `json:"def_handle"`
	ValHandle  int     `json:"val_handle"`
	Properties int     `json:"properties"`
	Uuid       BleUuid `json:"uuid"`
}

type BleSyncReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`
}

type BleConnectReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	OwnAddrType  AddrType `json:"own_addr_type"`
	PeerAddrType AddrType `json:"peer_addr_type"`
	PeerAddr     BleAddr  `json:"peer_addr"`

	// Optional
	DurationMs         int `json:"duration_ms"`
	ScanItvl           int `json:"scan_itvl"`
	ScanWindow         int `json:"scan_window"`
	ItvlMin            int `json:"itvl_min"`
	ItvlMax            int `json:"itvl_max"`
	Latency            int `json:"latency"`
	SupervisionTimeout int `json:"supervision_timeout"`
	MinCeLen           int `json:"min_ce_len"`
	MaxCeLen           int `json:"max_ce_len"`
}

type BleConnectRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleConnectEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status          int      `json:"status"`
	ConnHandle      int      `json:"conn_handle"`
	OwnIdAddrType   AddrType `json:"own_id_addr_type"`
	OwnIdAddr       BleAddr  `json:"own_id_addr"`
	OwnOtaAddrType  AddrType `json:"own_ota_addr_type"`
	OwnOtaAddr      BleAddr  `json:"own_ota_addr"`
	PeerIdAddrType  AddrType `json:"peer_id_addr_type"`
	PeerIdAddr      BleAddr  `json:"peer_id_addr"`
	PeerOtaAddrType AddrType `json:"peer_ota_addr_type"`
	PeerOtaAddr     BleAddr  `json:"peer_ota_addr"`
}

type BleTerminateReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	ConnHandle int `json:"conn_handle"`
	HciReason  int `json:"hci_reason"`
}

type BleTerminateRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleConnCancelReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`
}

type BleConnCancelRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDisconnectEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Reason     int `json:"reason"`
	ConnHandle int `json:"conn_handle"`
}

type BleDiscSvcUuidReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle int     `json:"conn_handle"`
	Uuid       BleUuid `json:"svc_uuid"`
}

type BleDiscSvcUuidRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDiscSvcEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Svc    BleSvc `json:"service"`
}

type BleDiscChrUuidReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle  int     `json:"conn_handle"`
	StartHandle int     `json:"start_handle"`
	EndHandle   int     `json:"end_handle"`
	Uuid        BleUuid `json:"chr_uuid"`
}

type BleDiscAllChrsReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle  int `json:"conn_handle"`
	StartHandle int `json:"start_handle"`
	EndHandle   int `json:"end_handle"`
}

type BleDiscAllChrsRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleErrRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type BleSyncRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Synced bool `json:"synced"`
}

type BleDiscChrUuidRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDiscChrEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Chr    BleChr `json:"characteristic"`
}

type BleWriteCmdReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle int      `json:"conn_handle"`
	AttrHandle int      `json:"attr_handle"`
	Data       BleBytes `json:"data"`
}

type BleWriteCmdRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleSyncEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Synced bool `json:"synced"`
}

type BleNotifyRxEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle int      `json:"conn_handle"`
	AttrHandle int      `json:"attr_handle"`
	Indication bool     `json:"indication"`
	Data       BleBytes `json:"data"`
}

type BleExchangeMtuReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	ConnHandle int `json:"conn_handle"`
}

type BleExchangeMtuRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleMtuChangeEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  int     `json:"seq"`

	// Mandatory
	Status     int `json:"status"`
	ConnHandle int `json:"conn_handle"`
	Mtu        int `json:"mtu"`
}

func AddrTypeToString(addrType AddrType) string {
	s := AddrTypeStringMap[addrType]
	if s == "" {
		panic(fmt.Sprintf("Invalid AddrType: %d", int(addrType)))
	}

	return s
}

func AddrTypeFromString(s string) (AddrType, error) {
	for addrType, name := range AddrTypeStringMap {
		if s == name {
			return addrType, nil
		}
	}

	return AddrType(0), errors.New("Invalid AddrType string: " + s)
}

func MsgOpToString(addrType MsgOp) string {
	s := MsgOpStringMap[addrType]
	if s == "" {
		panic(fmt.Sprintf("Invalid MsgOp: %d", int(addrType)))
	}

	return s
}

func MsgOpFromString(s string) (MsgOp, error) {
	for op, name := range MsgOpStringMap {
		if s == name {
			return op, nil
		}
	}

	return MsgOp(0), errors.New("Invalid MsgOp string: " + s)
}

func MsgTypeToString(msgType MsgType) string {
	s := MsgTypeStringMap[msgType]
	if s == "" {
		panic(fmt.Sprintf("Invalid MsgType: %d", int(msgType)))
	}

	return s
}

func MsgTypeFromString(s string) (MsgType, error) {
	for addrType, name := range MsgTypeStringMap {
		if s == name {
			return addrType, nil
		}
	}

	return MsgType(0), errors.New("Invalid MsgType string: " + s)
}

func (a AddrType) MarshalJSON() ([]byte, error) {
	return json.Marshal(AddrTypeToString(a))
}

func (a *AddrType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*a, err = AddrTypeFromString(s)
	return err
}

func (o MsgOp) MarshalJSON() ([]byte, error) {
	return json.Marshal(MsgOpToString(o))
}

func (o *MsgOp) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*o, err = MsgOpFromString(s)
	return err
}

func (t MsgType) MarshalJSON() ([]byte, error) {
	return json.Marshal(MsgTypeToString(t))
}

func (t *MsgType) UnmarshalJSON(data []byte) error {
	var err error

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*t, err = MsgTypeFromString(s)
	return err
}

func (bb *BleBytes) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(len(bb.Bytes) * 5)

	for i, b := range bb.Bytes {
		if i != 0 {
			buf.WriteString(":")
		}
		fmt.Fprintf(&buf, "0x%02x", b)
	}

	s := buf.String()
	return json.Marshal(s)

	return buf.Bytes(), nil
}

func (bb *BleBytes) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	toks := strings.Split(strings.ToLower(s), ":")
	bb.Bytes = make([]byte, len(toks))

	for i, t := range toks {
		if !strings.HasPrefix(t, "0x") {
			return errors.New("Byte stream contains invalid token: " + t)
		}

		u64, err := strconv.ParseUint(t, 0, 8)
		if err != nil {
			return err
		}
		bb.Bytes[i] = byte(u64)
	}

	return nil
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

func (bu *BleUuid) String() string {
	var buf bytes.Buffer
	buf.Grow(len(bu.Bytes)*2 + 3)

	// XXX: For now, only support 128-bit UUIDs.

	for i, b := range bu.Bytes {
		switch i {
		case 4, 6, 8, 10:
			buf.WriteString("-")
		}

		fmt.Fprintf(&buf, "%02x", b)
	}

	return buf.String()
}

func (bu *BleUuid) MarshalJSON() ([]byte, error) {
	return json.Marshal(bu.String())
}

func (bu *BleUuid) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	var err error
	*bu, err = ParseUuid(s)
	if err != nil {
		return err
	}

	return nil
}

func CompareUuids(a BleUuid, b BleUuid) int {
	return bytes.Compare(a.Bytes[:], b.Bytes[:])
}
