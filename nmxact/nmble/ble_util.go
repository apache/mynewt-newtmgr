package nmble

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

const WRITE_CMD_BASE_SZ = 3
const NOTIFY_CMD_BASE_SZ = 3

var nextSeq BleSeq = BLE_SEQ_MIN
var seqMtx sync.Mutex

func NextSeq() BleSeq {
	seqMtx.Lock()
	defer seqMtx.Unlock()

	seq := nextSeq
	nextSeq++
	if nextSeq >= BLE_SEQ_EVT_MIN {
		nextSeq = BLE_SEQ_MIN
	}

	return seq
}

func BhdTimeoutError(rspType MsgType, seq BleSeq) error {
	str := fmt.Sprintf(
		"Timeout waiting for blehostd to send %s response (seq=%d)",
		MsgTypeToString(rspType), seq)

	log.Debug(str)

	// XXX: Print stack trace; temporary change to debug timeout.
	buf := make([]byte, 1024*1024)
	stacklen := runtime.Stack(buf, true)
	log.Debug(buf[:stacklen])

	return nmxutil.NewXportError(str)
}

func StatusError(op MsgOp, msgType MsgType, status int) error {
	str := fmt.Sprintf("%s %s indicates error: %s (%d)",
		MsgOpToString(op),
		MsgTypeToString(msgType),
		ErrCodeToString(status),
		status)

	log.Debug(str)
	return nmxutil.NewBleHostError(status, str)
}

func BleDescFromConnFindRsp(r *BleConnFindRsp) BleConnDesc {
	return BleConnDesc{
		ConnHandle:      r.ConnHandle,
		OwnIdAddrType:   r.OwnIdAddrType,
		OwnIdAddr:       r.OwnIdAddr,
		OwnOtaAddrType:  r.OwnOtaAddrType,
		OwnOtaAddr:      r.OwnOtaAddr,
		PeerIdAddrType:  r.PeerIdAddrType,
		PeerIdAddr:      r.PeerIdAddr,
		PeerOtaAddrType: r.PeerOtaAddrType,
		PeerOtaAddr:     r.PeerOtaAddr,
	}
}

func BleAdvReportFromScanEvt(e *BleScanEvt) BleAdvReport {
	return BleAdvReport{
		EventType: e.EventType,
		Sender: BleDev{
			AddrType: e.AddrType,
			Addr:     e.Addr,
		},
		Rssi: e.Rssi,

		Fields: BleAdvFields{
			Data: e.Data.Bytes,

			Flags:              e.DataFlags,
			Uuids16:            e.DataUuids16,
			Uuids16IsComplete:  e.DataUuids16IsComplete,
			Uuids32:            e.DataUuids32,
			Uuids32IsComplete:  e.DataUuids32IsComplete,
			Uuids128:           e.DataUuids128,
			Uuids128IsComplete: e.DataUuids128IsComplete,
			Name:               e.DataName,
			NameIsComplete:     e.DataNameIsComplete,
			TxPwrLvl:           e.DataTxPwrLvl,
			SlaveItvlMin:       e.DataSlaveItvlMin,
			SlaveItvlMax:       e.DataSlaveItvlMax,
			SvcDataUuid16:      e.DataSvcDataUuid16.Bytes,
			PublicTgtAddrs:     e.DataPublicTgtAddrs,
			Appearance:         e.DataAppearance,
			AdvItvl:            e.DataAdvItvl,
			SvcDataUuid32:      e.DataSvcDataUuid32.Bytes,
			SvcDataUuid128:     e.DataSvcDataUuid128.Bytes,
			Uri:                e.DataUri,
			MfgData:            e.DataMfgData.Bytes,
		},
	}
}

func NewBleConnectReq() *BleConnectReq {
	return &BleConnectReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_CONNECT,
		Seq:  NextSeq(),

		OwnAddrType:  BLE_ADDR_TYPE_PUBLIC,
		PeerAddrType: BLE_ADDR_TYPE_PUBLIC,
		PeerAddr:     BleAddr{},

		DurationMs:         30000,
		ScanItvl:           0x0010,
		ScanWindow:         0x0010,
		ItvlMin:            24,
		ItvlMax:            40,
		Latency:            0,
		SupervisionTimeout: 0x0200,
		MinCeLen:           0x0010,
		MaxCeLen:           0x0300,
	}
}

func NewBleTerminateReq() *BleTerminateReq {
	return &BleTerminateReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_TERMINATE,
		Seq:  NextSeq(),

		ConnHandle: 0,
		HciReason:  0,
	}
}

func NewBleConnCancelReq() *BleConnCancelReq {
	return &BleConnCancelReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_CONN_CANCEL,
		Seq:  NextSeq(),
	}
}

func NewBleDiscSvcUuidReq() *BleDiscSvcUuidReq {
	return &BleDiscSvcUuidReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_DISC_SVC_UUID,
		Seq:  NextSeq(),

		ConnHandle: 0,
		Uuid:       BleUuid{},
	}
}

func NewBleDiscAllChrsReq() *BleDiscAllChrsReq {
	return &BleDiscAllChrsReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_DISC_ALL_CHRS,
		Seq:  NextSeq(),
	}
}

func NewBleExchangeMtuReq() *BleExchangeMtuReq {
	return &BleExchangeMtuReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_EXCHANGE_MTU,
		Seq:  NextSeq(),

		ConnHandle: 0,
	}
}

func NewBleGenRandAddrReq() *BleGenRandAddrReq {
	return &BleGenRandAddrReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_GEN_RAND_ADDR,
		Seq:  NextSeq(),
	}
}

func NewBleSetRandAddrReq() *BleSetRandAddrReq {
	return &BleSetRandAddrReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SET_RAND_ADDR,
		Seq:  NextSeq(),
	}
}

func NewBleWriteCmdReq() *BleWriteCmdReq {
	return &BleWriteCmdReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_WRITE_CMD,
		Seq:  NextSeq(),

		ConnHandle: 0,
		AttrHandle: 0,
		Data:       BleBytes{},
	}
}

func NewBleScanReq() *BleScanReq {
	return &BleScanReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SCAN,
		Seq:  NextSeq(),
	}
}

func NewBleScanCancelReq() *BleScanCancelReq {
	return &BleScanCancelReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SCAN_CANCEL,
		Seq:  NextSeq(),
	}
}

func NewBleSetPreferredMtuReq() *BleSetPreferredMtuReq {
	return &BleSetPreferredMtuReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SET_PREFERRED_MTU,
		Seq:  NextSeq(),
	}
}

func NewBleConnFindReq() *BleConnFindReq {
	return &BleConnFindReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_CONN_FIND,
		Seq:  NextSeq(),
	}
}

func NewResetReq() *BleResetReq {
	return &BleResetReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_RESET,
		Seq:  NextSeq(),
	}
}

func NewBleSecurityInitiateReq() *BleSecurityInitiateReq {
	return &BleSecurityInitiateReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_SECURITY_INITIATE,
		Seq:  NextSeq(),
	}
}

func NewBleAdvFieldsReq() *BleAdvFieldsReq {
	return &BleAdvFieldsReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_ADV_FIELDS,
		Seq:  NextSeq(),
	}
}

func NewBleAdvSetDataReq() *BleAdvSetDataReq {
	return &BleAdvSetDataReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_ADV_SET_DATA,
		Seq:  NextSeq(),
	}
}

func NewBleAdvRspSetDataReq() *BleAdvRspSetDataReq {
	return &BleAdvRspSetDataReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_ADV_RSP_SET_DATA,
		Seq:  NextSeq(),
	}
}

func NewBleAdvStartReq() *BleAdvStartReq {
	return &BleAdvStartReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_ADV_START,
		Seq:  NextSeq(),
	}
}

func NewBleAdvStopReq() *BleAdvStopReq {
	return &BleAdvStopReq{
		Op:   MSG_OP_REQ,
		Type: MSG_TYPE_ADV_STOP,
		Seq:  NextSeq(),
	}
}

func ConnFindXact(x *BleXport, connHandle uint16) (BleConnDesc, error) {
	r := NewBleConnFindReq()
	r.ConnHandle = connHandle

	key := SeqKey(r.Seq)
	bl, err := x.AddListener(key)
	if err != nil {
		return BleConnDesc{}, err
	}
	defer x.RemoveListener(bl)

	return connFind(x, bl, r)
}

func GenRandAddrXact(x *BleXport) (BleAddr, error) {
	r := NewBleGenRandAddrReq()

	key := SeqKey(r.Seq)
	bl, err := x.AddListener(key)
	if err != nil {
		return BleAddr{}, err
	}
	defer x.RemoveListener(bl)

	return genRandAddr(x, bl, r)
}

func SetRandAddrXact(x *BleXport, addr BleAddr) error {
	r := NewBleSetRandAddrReq()
	r.Addr = addr

	key := SeqKey(r.Seq)
	bl, err := x.AddListener(key)
	if err != nil {
		return err
	}
	defer x.RemoveListener(bl)

	return setRandAddr(x, bl, r)
}

func SetPreferredMtuXact(x *BleXport, mtu uint16) error {
	r := NewBleSetPreferredMtuReq()
	r.Mtu = mtu

	key := SeqKey(r.Seq)
	bl, err := x.AddListener(key)
	if err != nil {
		return err
	}
	defer x.RemoveListener(bl)

	return setPreferredMtu(x, bl, r)
}

func ResetXact(x *BleXport) error {
	r := NewResetReq()

	key := SeqKey(r.Seq)
	bl, err := x.AddListener(key)
	if err != nil {
		return err
	}
	defer x.RemoveListener(bl)

	return reset(x, bl, r)
}

func DiscoverDeviceWithName(
	bx *BleXport,
	ownAddrType BleAddrType,
	timeout time.Duration,
	name string) (*BleDev, error) {

	advPred := func(r BleAdvReport) bool {
		return r.Fields.Name != nil && *r.Fields.Name == name
	}

	return DiscoverDevice(bx, ownAddrType, timeout, advPred)
}
