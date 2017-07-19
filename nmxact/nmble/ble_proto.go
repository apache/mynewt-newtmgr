package nmble

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type MsgOp int
type MsgType int
type BleSeq uint32

type BleBytes struct {
	Bytes []byte
}

const BLE_SEQ_MIN BleSeq = 0
const BLE_SEQ_EVT_MIN BleSeq = 0xffffff00
const BLE_SEQ_NONE BleSeq = 0xffffffff

const ERR_CODE_ATT_BASE = 0x100
const ERR_CODE_HCI_BASE = 0x200
const ERR_CODE_L2C_BASE = 0x300
const ERR_CODE_SM_US_BASE = 0x400
const ERR_CODE_SM_PEER_BASE = 0x500

const (
	ERR_CODE_EAGAIN       int = 1
	ERR_CODE_EALREADY         = 2
	ERR_CODE_EINVAL           = 3
	ERR_CODE_EMSGSIZE         = 4
	ERR_CODE_ENOENT           = 5
	ERR_CODE_ENOMEM           = 6
	ERR_CODE_ENOTCONN         = 7
	ERR_CODE_ENOTSUP          = 8
	ERR_CODE_EAPP             = 9
	ERR_CODE_EBADDATA         = 10
	ERR_CODE_EOS              = 11
	ERR_CODE_ECONTROLLER      = 12
	ERR_CODE_ETIMEOUT         = 13
	ERR_CODE_EDONE            = 14
	ERR_CODE_EBUSY            = 15
	ERR_CODE_EREJECT          = 16
	ERR_CODE_EUNKNOWN         = 17
	ERR_CODE_EROLE            = 18
	ERR_CODE_ETIMEOUT_HCI     = 19
	ERR_CODE_ENOMEM_EVT       = 20
	ERR_CODE_ENOADDR          = 21
	ERR_CODE_ENOTSYNCED       = 22
)

var ErrCodeStringMap = map[int]string{
	ERR_CODE_EAGAIN:       "eagain",
	ERR_CODE_EALREADY:     "ealready",
	ERR_CODE_EINVAL:       "einval",
	ERR_CODE_EMSGSIZE:     "emsgsize",
	ERR_CODE_ENOENT:       "enoent",
	ERR_CODE_ENOMEM:       "enomem",
	ERR_CODE_ENOTCONN:     "enotconn",
	ERR_CODE_ENOTSUP:      "enotsup",
	ERR_CODE_EAPP:         "eapp",
	ERR_CODE_EBADDATA:     "ebaddata",
	ERR_CODE_EOS:          "eos",
	ERR_CODE_ECONTROLLER:  "econtroller",
	ERR_CODE_ETIMEOUT:     "etimeout",
	ERR_CODE_EDONE:        "edone",
	ERR_CODE_EBUSY:        "ebusy",
	ERR_CODE_EREJECT:      "ereject",
	ERR_CODE_EUNKNOWN:     "eunknown",
	ERR_CODE_EROLE:        "erole",
	ERR_CODE_ETIMEOUT_HCI: "etimeout_hci",
	ERR_CODE_ENOMEM_EVT:   "enomem_evt",
	ERR_CODE_ENOADDR:      "enoaddr",
	ERR_CODE_ENOTSYNCED:   "enotsynced",
}

const (
	ERR_CODE_HCI_UNKNOWN_HCI_CMD     int = 1
	ERR_CODE_HCI_UNK_CONN_ID             = 2
	ERR_CODE_HCI_HW_FAIL                 = 3
	ERR_CODE_HCI_PAGE_TMO                = 4
	ERR_CODE_HCI_AUTH_FAIL               = 5
	ERR_CODE_HCI_PINKEY_MISSING          = 6
	ERR_CODE_HCI_MEM_CAPACITY            = 7
	ERR_CODE_HCI_CONN_SPVN_TMO           = 8
	ERR_CODE_HCI_CONN_LIMIT              = 9
	ERR_CODE_HCI_SYNCH_CONN_LIMIT        = 10
	ERR_CODE_HCI_ACL_CONN_EXISTS         = 11
	ERR_CODE_HCI_CMD_DISALLOWED          = 12
	ERR_CODE_HCI_CONN_REJ_RESOURCES      = 13
	ERR_CODE_HCI_CONN_REJ_ENC            = 14
	ERR_CODE_HCI_CONN_REJ_BD_ADDR        = 15
	ERR_CODE_HCI_CONN_ACCEPT_TMO         = 16
	ERR_CODE_HCI_UNSUPPORTED             = 17
	ERR_CODE_HCI_INV_HCI_CMD_PARMS       = 18
	ERR_CODE_HCI_REM_USER_CONN_TERM      = 19
	ERR_CODE_HCI_RD_CONN_TERM_RESRCS     = 20
	ERR_CODE_HCI_RD_CONN_TERM_PWROFF     = 21
	ERR_CODE_HCI_CONN_TERM_LOCAL         = 22
	ERR_CODE_HCI_REPEATED_ATTEMPTS       = 23
	ERR_CODE_HCI_NO_PAIRING              = 24
	ERR_CODE_HCI_UNK_LMP                 = 25
	ERR_CODE_HCI_UNSUPP_REM_FEATURE      = 26
	ERR_CODE_HCI_SCO_OFFSET              = 27
	ERR_CODE_HCI_SCO_ITVL                = 28
	ERR_CODE_HCI_SCO_AIR_MODE            = 29
	ERR_CODE_HCI_INV_LMP_LL_PARM         = 30
	ERR_CODE_HCI_UNSPECIFIED             = 31
	ERR_CODE_HCI_UNSUPP_LMP_LL_PARM      = 32
	ERR_CODE_HCI_NO_ROLE_CHANGE          = 33
	ERR_CODE_HCI_LMP_LL_RSP_TMO          = 34
	ERR_CODE_HCI_LMP_COLLISION           = 35
	ERR_CODE_HCI_LMP_PDU                 = 36
	ERR_CODE_HCI_ENCRYPTION_MODE         = 37
	ERR_CODE_HCI_LINK_KEY_CHANGE         = 38
	ERR_CODE_HCI_UNSUPP_QOS              = 39
	ERR_CODE_HCI_INSTANT_PASSED          = 40
	ERR_CODE_HCI_UNIT_KEY_PAIRING        = 41
	ERR_CODE_HCI_DIFF_TRANS_COLL         = 42
	ERR_CODE_HCI_QOS_PARM                = 44
	ERR_CODE_HCI_QOS_REJECTED            = 45
	ERR_CODE_HCI_CHAN_CLASS              = 46
	ERR_CODE_HCI_INSUFFICIENT_SEC        = 47
	ERR_CODE_HCI_PARM_OUT_OF_RANGE       = 48
	ERR_CODE_HCI_PENDING_ROLE_SW         = 50
	ERR_CODE_HCI_RESERVED_SLOT           = 52
	ERR_CODE_HCI_ROLE_SW_FAIL            = 53
	ERR_CODE_HCI_INQ_RSP_TOO_BIG         = 54
	ERR_CODE_HCI_SEC_SIMPLE_PAIR         = 55
	ERR_CODE_HCI_HOST_BUSY_PAIR          = 56
	ERR_CODE_HCI_CONN_REJ_CHANNEL        = 57
	ERR_CODE_HCI_CTLR_BUSY               = 58
	ERR_CODE_HCI_CONN_PARMS              = 59
	ERR_CODE_HCI_DIR_ADV_TMO             = 60
	ERR_CODE_HCI_CONN_TERM_MIC           = 61
	ERR_CODE_HCI_CONN_ESTABLISHMENT      = 62
	ERR_CODE_HCI_MAC_CONN_FAIL           = 63
	ERR_CODE_HCI_COARSE_CLK_ADJ          = 64
)

var HciErrCodeStringMap = map[int]string{
	ERR_CODE_HCI_UNKNOWN_HCI_CMD:     "unknown hci cmd",
	ERR_CODE_HCI_UNK_CONN_ID:         "unknown connection id",
	ERR_CODE_HCI_HW_FAIL:             "hw fail",
	ERR_CODE_HCI_PAGE_TMO:            "page tmo",
	ERR_CODE_HCI_AUTH_FAIL:           "auth fail",
	ERR_CODE_HCI_PINKEY_MISSING:      "pinkey missing",
	ERR_CODE_HCI_MEM_CAPACITY:        "mem capacity",
	ERR_CODE_HCI_CONN_SPVN_TMO:       "connection supervision timeout",
	ERR_CODE_HCI_CONN_LIMIT:          "conn limit",
	ERR_CODE_HCI_SYNCH_CONN_LIMIT:    "synch conn limit",
	ERR_CODE_HCI_ACL_CONN_EXISTS:     "acl conn exists",
	ERR_CODE_HCI_CMD_DISALLOWED:      "cmd disallowed",
	ERR_CODE_HCI_CONN_REJ_RESOURCES:  "conn rej resources",
	ERR_CODE_HCI_CONN_REJ_ENC:        "conn rej security",
	ERR_CODE_HCI_CONN_REJ_BD_ADDR:    "conn rej bd addr",
	ERR_CODE_HCI_CONN_ACCEPT_TMO:     "conn accept tmo",
	ERR_CODE_HCI_UNSUPPORTED:         "unsupported",
	ERR_CODE_HCI_INV_HCI_CMD_PARMS:   "inv hci cmd parms",
	ERR_CODE_HCI_REM_USER_CONN_TERM:  "rem user conn term",
	ERR_CODE_HCI_RD_CONN_TERM_RESRCS: "rd conn term resrcs",
	ERR_CODE_HCI_RD_CONN_TERM_PWROFF: "rd conn term pwroff",
	ERR_CODE_HCI_CONN_TERM_LOCAL:     "conn term local",
	ERR_CODE_HCI_REPEATED_ATTEMPTS:   "repeated attempts",
	ERR_CODE_HCI_NO_PAIRING:          "no pairing",
	ERR_CODE_HCI_UNK_LMP:             "unk lmp",
	ERR_CODE_HCI_UNSUPP_REM_FEATURE:  "unsupp rem feature",
	ERR_CODE_HCI_SCO_OFFSET:          "sco offset",
	ERR_CODE_HCI_SCO_ITVL:            "sco itvl",
	ERR_CODE_HCI_SCO_AIR_MODE:        "sco air mode",
	ERR_CODE_HCI_INV_LMP_LL_PARM:     "inv lmp ll parm",
	ERR_CODE_HCI_UNSPECIFIED:         "unspecified",
	ERR_CODE_HCI_UNSUPP_LMP_LL_PARM:  "unsupp lmp ll parm",
	ERR_CODE_HCI_NO_ROLE_CHANGE:      "no role change",
	ERR_CODE_HCI_LMP_LL_RSP_TMO:      "lmp ll rsp tmo",
	ERR_CODE_HCI_LMP_COLLISION:       "lmp collision",
	ERR_CODE_HCI_LMP_PDU:             "lmp pdu",
	ERR_CODE_HCI_ENCRYPTION_MODE:     "encryption mode",
	ERR_CODE_HCI_LINK_KEY_CHANGE:     "link key change",
	ERR_CODE_HCI_UNSUPP_QOS:          "unsupp qos",
	ERR_CODE_HCI_INSTANT_PASSED:      "instant passed",
	ERR_CODE_HCI_UNIT_KEY_PAIRING:    "unit key pairing",
	ERR_CODE_HCI_DIFF_TRANS_COLL:     "diff trans coll",
	ERR_CODE_HCI_QOS_PARM:            "qos parm",
	ERR_CODE_HCI_QOS_REJECTED:        "qos rejected",
	ERR_CODE_HCI_CHAN_CLASS:          "chan class",
	ERR_CODE_HCI_INSUFFICIENT_SEC:    "insufficient sec",
	ERR_CODE_HCI_PARM_OUT_OF_RANGE:   "parm out of range",
	ERR_CODE_HCI_PENDING_ROLE_SW:     "pending role sw",
	ERR_CODE_HCI_RESERVED_SLOT:       "reserved slot",
	ERR_CODE_HCI_ROLE_SW_FAIL:        "role sw fail",
	ERR_CODE_HCI_INQ_RSP_TOO_BIG:     "inq rsp too big",
	ERR_CODE_HCI_SEC_SIMPLE_PAIR:     "sec simple pair",
	ERR_CODE_HCI_HOST_BUSY_PAIR:      "host busy pair",
	ERR_CODE_HCI_CONN_REJ_CHANNEL:    "conn rej channel",
	ERR_CODE_HCI_CTLR_BUSY:           "ctlr busy",
	ERR_CODE_HCI_CONN_PARMS:          "conn parms",
	ERR_CODE_HCI_DIR_ADV_TMO:         "dir adv tmo",
	ERR_CODE_HCI_CONN_TERM_MIC:       "conn term mic",
	ERR_CODE_HCI_CONN_ESTABLISHMENT:  "conn establishment",
	ERR_CODE_HCI_MAC_CONN_FAIL:       "mac conn fail",
	ERR_CODE_HCI_COARSE_CLK_ADJ:      "coarse clk adj",
}

// These values never get transmitted or received, so their precise values
// don't matter.  We specify them explicitly here to match the blehostd source.
const (
	MSG_OP_REQ MsgOp = 0
	MSG_OP_RSP       = 1
	MSG_OP_EVT       = 2
)

// These values never get transmitted or received, so their precise values
// don't matter.  We specify them explicitly here to match the blehostd source.
const (
	MSG_TYPE_ERR               MsgType = 1
	MSG_TYPE_SYNC                      = 2
	MSG_TYPE_CONNECT                   = 3
	MSG_TYPE_TERMINATE                 = 4
	MSG_TYPE_DISC_ALL_SVCS             = 5
	MSG_TYPE_DISC_SVC_UUID             = 6
	MSG_TYPE_DISC_ALL_CHRS             = 7
	MSG_TYPE_DISC_CHR_UUID             = 8
	MSG_TYPE_WRITE                     = 9
	MSG_TYPE_WRITE_CMD                 = 10
	MSG_TYPE_EXCHANGE_MTU              = 11
	MSG_TYPE_GEN_RAND_ADDR             = 12
	MSG_TYPE_SET_RAND_ADDR             = 13
	MSG_TYPE_CONN_CANCEL               = 14
	MSG_TYPE_SCAN                      = 15
	MSG_TYPE_SCAN_CANCEL               = 16
	MSG_TYPE_SET_PREFERRED_MTU         = 17
	MSG_TYPE_SECURITY_INITIATE         = 18
	MSG_TYPE_CONN_FIND                 = 19
	MSG_TYPE_RESET                     = 20
	MSG_TYPE_ADV_START                 = 21
	MSG_TYPE_ADV_STOP                  = 22
	MSG_TYPE_ADV_SET_DATA              = 23
	MSG_TYPE_ADV_RSP_SET_DATA          = 24
	MSG_TYPE_ADV_FIELDS                = 25

	MSG_TYPE_SYNC_EVT       = 2049
	MSG_TYPE_CONNECT_EVT    = 2050
	MSG_TYPE_DISCONNECT_EVT = 2051
	MSG_TYPE_DISC_SVC_EVT   = 2052
	MSG_TYPE_DISC_CHR_EVT   = 2053
	MSG_TYPE_WRITE_ACK_EVT  = 2054
	MSG_TYPE_NOTIFY_RX_EVT  = 2055
	MSG_TYPE_MTU_CHANGE_EVT = 2056
	MSG_TYPE_SCAN_EVT       = 2057
	MSG_TYPE_SCAN_TMO_EVT   = 2058
	MSG_TYPE_ENC_CHANGE_EVT = 2059
	MSG_TYPE_RESET_EVT      = 2060
)

var MsgOpStringMap = map[MsgOp]string{
	MSG_OP_REQ: "request",
	MSG_OP_RSP: "response",
	MSG_OP_EVT: "event",
}

var MsgTypeStringMap = map[MsgType]string{
	MSG_TYPE_ERR:               "error",
	MSG_TYPE_SYNC:              "sync",
	MSG_TYPE_CONNECT:           "connect",
	MSG_TYPE_TERMINATE:         "terminate",
	MSG_TYPE_DISC_SVC_UUID:     "disc_svc_uuid",
	MSG_TYPE_DISC_CHR_UUID:     "disc_chr_uuid",
	MSG_TYPE_DISC_ALL_CHRS:     "disc_all_chrs",
	MSG_TYPE_WRITE_CMD:         "write_cmd",
	MSG_TYPE_EXCHANGE_MTU:      "exchange_mtu",
	MSG_TYPE_GEN_RAND_ADDR:     "gen_rand_addr",
	MSG_TYPE_SET_RAND_ADDR:     "set_rand_addr",
	MSG_TYPE_CONN_CANCEL:       "conn_cancel",
	MSG_TYPE_SCAN:              "scan",
	MSG_TYPE_SCAN_CANCEL:       "scan_cancel",
	MSG_TYPE_SET_PREFERRED_MTU: "set_preferred_mtu",
	MSG_TYPE_SECURITY_INITIATE: "security_initiate",
	MSG_TYPE_CONN_FIND:         "conn_find",
	MSG_TYPE_RESET:             "reset",
	MSG_TYPE_ADV_START:         "adv_start",
	MSG_TYPE_ADV_STOP:          "adv_stop",
	MSG_TYPE_ADV_SET_DATA:      "adv_set_data",
	MSG_TYPE_ADV_RSP_SET_DATA:  "adv_rsp_set_data",
	MSG_TYPE_ADV_FIELDS:        "adv_fields",

	MSG_TYPE_SYNC_EVT:       "sync_evt",
	MSG_TYPE_CONNECT_EVT:    "connect_evt",
	MSG_TYPE_DISCONNECT_EVT: "disconnect_evt",
	MSG_TYPE_DISC_SVC_EVT:   "disc_svc_evt",
	MSG_TYPE_DISC_CHR_EVT:   "disc_chr_evt",
	MSG_TYPE_NOTIFY_RX_EVT:  "notify_rx_evt",
	MSG_TYPE_MTU_CHANGE_EVT: "mtu_change_evt",
	MSG_TYPE_SCAN_EVT:       "scan_evt",
	MSG_TYPE_SCAN_TMO_EVT:   "scan_tmo_evt",
	MSG_TYPE_ENC_CHANGE_EVT: "enc_change_evt",
	MSG_TYPE_RESET_EVT:      "reset_evt",
}

type BleHdr struct {
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type Msg interface{}

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
	Seq  BleSeq  `json:"seq"`
}

type BleConnectReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	OwnAddrType  BleAddrType `json:"own_addr_type"`
	PeerAddrType BleAddrType `json:"peer_addr_type"`
	PeerAddr     BleAddr     `json:"peer_addr"`

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
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleConnectEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status     int    `json:"status"`
	ConnHandle uint16 `json:"conn_handle"`
}

type BleTerminateReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	ConnHandle uint16 `json:"conn_handle"`
	HciReason  int    `json:"hci_reason"`
}

type BleTerminateRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleConnCancelReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type BleConnCancelRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDisconnectEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Reason     int    `json:"reason"`
	ConnHandle uint16 `json:"conn_handle"`
}

type BleDiscSvcUuidReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16  `json:"conn_handle"`
	Uuid       BleUuid `json:"svc_uuid"`
}

type BleDiscSvcUuidRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDiscSvcEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Svc    BleSvc `json:"service"`
}

type BleDiscChrUuidReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle  uint16  `json:"conn_handle"`
	StartHandle int     `json:"start_handle"`
	EndHandle   int     `json:"end_handle"`
	Uuid        BleUuid `json:"chr_uuid"`
}

type BleDiscAllChrsReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle  uint16 `json:"conn_handle"`
	StartHandle int    `json:"start_handle"`
	EndHandle   int    `json:"end_handle"`
}

type BleDiscAllChrsRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleErrRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

type BleSyncRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Synced bool `json:"synced"`
}

type BleDiscChrUuidRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleDiscChrEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Chr    BleChr `json:"characteristic"`
}

type BleWriteCmdReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16   `json:"conn_handle"`
	AttrHandle int      `json:"attr_handle"`
	Data       BleBytes `json:"data"`
}

type BleWriteCmdRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleSyncEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Synced bool `json:"synced"`
}

type BleNotifyRxEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16   `json:"conn_handle"`
	AttrHandle int      `json:"attr_handle"`
	Indication bool     `json:"indication"`
	Data       BleBytes `json:"data"`
}

type BleExchangeMtuReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16 `json:"conn_handle"`
}

type BleExchangeMtuRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleMtuChangeEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status     int    `json:"status"`
	ConnHandle uint16 `json:"conn_handle"`
	Mtu        uint16 `json:"mtu"`
}

type BleGenRandAddrReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Nrpa bool `json:"nrpa"`
}

type BleGenRandAddrRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int     `json:"status"`
	Addr   BleAddr `json:"addr"`
}

type BleSetRandAddrReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Addr BleAddr `json:"addr"`
}

type BleSetRandAddrRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleScanReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	OwnAddrType      BleAddrType         `json:"own_addr_type"`
	DurationMs       int                 `json:"duration_ms"`
	Itvl             int                 `json:"itvl"`
	Window           int                 `json:"window"`
	FilterPolicy     BleScanFilterPolicy `json:"filter_policy"`
	Limited          bool                `json:"limited"`
	Passive          bool                `json:"passive"`
	FilterDuplicates bool                `json:"filter_duplicates"`
}

type BleScanRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleScanEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	EventType BleAdvEventType `json:"event_type"`
	AddrType  BleAddrType     `json:"addr_type"`
	Addr      BleAddr         `json:"addr"`
	Rssi      int8            `json:"rssi"`
	Data      BleBytes        `json:"data"`

	// Optional
	DataFlags              *uint8       `json:"data_flags"`
	DataUuids16            []BleUuid16  `json:"data_uuids16"`
	DataUuids16IsComplete  bool         `json:"data_uuids16_is_complete"`
	DataUuids32            []uint32     `json:"data_uuids32"`
	DataUuids32IsComplete  bool         `json:"data_uuids32_is_complete"`
	DataUuids128           []BleUuid128 `json:"data_uuids128"`
	DataUuids128IsComplete bool         `json:"data_uuids128_is_complete"`
	DataName               *string      `json:"data_name"`
	DataNameIsComplete     bool         `json:"data_name_is_complete"`
	DataTxPwrLvl           *int8        `json:"data_tx_pwr_lvl"`
	DataSlaveItvlMin       *uint16      `json:"data_slave_itvl_min"`
	DataSlaveItvlMax       *uint16      `json:"data_slave_itvl_max"`
	DataSvcDataUuid16      BleBytes     `json:"data_svc_data_uuid16"`
	DataPublicTgtAddrs     []BleAddr    `json:"data_public_tgt_addrs"`
	DataAppearance         *uint16      `json:"data_appearance"`
	DataAdvItvl            *uint16      `json:"data_adv_itvl"`
	DataSvcDataUuid32      BleBytes     `json:"data_svc_data_uuid32"`
	DataSvcDataUuid128     BleBytes     `json:"data_svc_data_uuid128"`
	DataUri                *string      `json:"data_uri"`
	DataMfgData            BleBytes     `json:"data_mfg_data"`
}

type BleScanTmoEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type BleScanCancelReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type BleScanCancelRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleSetPreferredMtuReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Mtu uint16 `json:"mtu"`
}

type BleSetPreferredMtuRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleConnFindReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16 `json:"conn_handle"`
}

type BleConnFindRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status          int         `json:"status"`
	ConnHandle      uint16      `json:"conn_handle"`
	OwnIdAddrType   BleAddrType `json:"own_id_addr_type"`
	OwnIdAddr       BleAddr     `json:"own_id_addr"`
	OwnOtaAddrType  BleAddrType `json:"own_ota_addr_type"`
	OwnOtaAddr      BleAddr     `json:"own_ota_addr"`
	PeerIdAddrType  BleAddrType `json:"peer_id_addr_type"`
	PeerIdAddr      BleAddr     `json:"peer_id_addr"`
	PeerOtaAddrType BleAddrType `json:"peer_ota_addr_type"`
	PeerOtaAddr     BleAddr     `json:"peer_ota_addr"`
}

type BleResetReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type BleResetRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleSecurityInitiateReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	ConnHandle uint16 `json:"conn_handle"`
}

type BleSecurityInitiateRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleEncChangeEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status     int    `json:"status"`
	ConnHandle uint16 `json:"conn_handle"`
}

type BleAdvStartReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	OwnAddrType   BleAddrType        `json:"own_addr_type"`
	DurationMs    int                `json:"duration_ms"`
	ConnMode      BleAdvConnMode     `json:"conn_mode"`
	DiscMode      BleAdvDiscMode     `json:"disc_mode"`
	ItvlMin       uint16             `json:"itvl_min"`
	ItvlMax       uint16             `json:"itvl_max"`
	ChannelMap    uint8              `json:"channel_map"`
	FilterPolicy  BleAdvFilterPolicy `json:"filter_policy"`
	HighDutyCycle bool               `json:"high_duty_cycle"`

	// Only required for direct advertisements
	PeerAddr *BleAddr `json:"peer_addr"`
}

type BleAdvStartRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleAdvStopReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`
}

type BleAdvStopRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleAdvSetDataReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Data []byte `json:"data"`
}

type BleAdvSetDataRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleAdvRspSetDataReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Data []byte `json:"data"`
}

type BleAdvRspSetDataRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int `json:"status"`
}

type BleAdvFieldsReq struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Optional
	Flags *uint8 `json:"flags,omitempty"`

	/*** 0x02,0x03 - 16-bit service class UUIDs. */
	Uuids16           []BleUuid16 `json:"uuids16"`
	Uuids16IsComplete bool        `json:"uuids16_is_complete"`

	/*** 0x04,0x05 - 32-bit service class UUIDs. */
	Uuids32           []uint32 `json:"uuids32"`
	Uuids32IsComplete bool     `json:"uuids32_is_complete"`

	/*** 0x06,0x07 - 128-bit service class UUIDs. */
	Uuids128           []BleUuid128 `json:"uuids128"`
	Uuids128IsComplete bool         `json:"uuids128_is_complete"`

	/*** 0x08,0x09 - Local name. */
	Name           *string `json:"name,omitempty"`
	NameIsComplete bool    `json:"name_is_complete"`

	/*** 0x0a - Tx power level. */
	TxPwrLvl *int8 `json:"tx_pwr_lvl"`

	/*** 0x0d - Slave connection interval range. */
	SlaveItvlMin *uint16 `json:"slave_itvl_min"`
	SlaveItvlMax *uint16 `json:"slave_itvl_max"`

	/*** 0x16 - Service data - 16-bit UUID. */
	SvcDataUuid16 []byte `json:"svc_data_uuid16"`

	/*** 0x17 - Public target address. */
	PublicTgtAddrs []BleAddr `json:"public_tgt_addrs"`

	/*** 0x19 - Appearance. */
	Appearance *uint16 `json:"appearance"`

	/*** 0x1a - Advertising interval. */
	AdvItvl *uint16 `json:"adv_itvl"`

	/*** 0x20 - Service data - 32-bit UUID. */
	SvcDataUuid32 []byte `json:"svc_data_uuid32"`

	/*** 0x21 - Service data - 128-bit UUID. */
	SvcDataUuid128 []byte `json:"svc_data_uuid128"`

	/*** 0x24 - URI. */
	Uri *string `json:"uri,omitempty"`

	/*** 0xff - Manufacturer specific data. */
	MfgData []byte `json:"mfg_data"`
}

type BleAdvFieldsRsp struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Status int    `json:"status"`
	Data   []byte `json:"data"`
}

type BleResetEvt struct {
	// Header
	Op   MsgOp   `json:"op"`
	Type MsgType `json:"type"`
	Seq  BleSeq  `json:"seq"`

	// Mandatory
	Reason int `json:"reason"`
}

func ErrCodeToString(e int) string {
	var s string

	switch {
	case e >= ERR_CODE_SM_PEER_BASE:
	case e >= ERR_CODE_SM_US_BASE:
	case e >= ERR_CODE_L2C_BASE:
	case e >= ERR_CODE_HCI_BASE:
		s = HciErrCodeStringMap[e-ERR_CODE_HCI_BASE]

	case e >= ERR_CODE_ATT_BASE:
	default:
		s = ErrCodeStringMap[e]
	}

	if s == "" {
		s = "unknown"
	}

	return s
}

func MsgOpToString(op MsgOp) string {
	s := MsgOpStringMap[op]
	if s == "" {
		return "???"
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
		return "???"
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

	// strings.Split() appears to return { nil } when passed an empty string.
	if len(s) == 0 {
		return nil
	}

	toks := strings.Split(strings.ToLower(s), ":")
	bb.Bytes = make([]byte, len(toks))

	for i, t := range toks {
		if !strings.HasPrefix(t, "0x") {
			return fmt.Errorf(
				"Byte stream contains invalid token; token=%s stream=%s", t, s)
		}

		u64, err := strconv.ParseUint(t, 0, 8)
		if err != nil {
			return err
		}
		bb.Bytes[i] = byte(u64)
	}

	return nil
}
