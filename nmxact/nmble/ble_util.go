package nmble

import (
	"fmt"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

const NmpPlainSvcUuid = "8D53DC1D-1DB7-4CD3-868B-8A527460AA84"
const NmpPlainChrUuid = "DA2E7828-FBCE-4E01-AE9E-261174997C48"
const NmpOicSvcUuid = "ADE3D529-C784-4F63-A987-EB69F70EE816"
const NmpOicReqChrUuid = "AD7B334F-4637-4B86-90B6-9D787F03D218"
const NmpOicRspChrUuid = "E9241982-4580-42C4-8831-95048216B256"

const WRITE_CMD_BASE_SZ = 3
const NOTIFY_CMD_BASE_SZ = 3

var nextSeq uint32

func NextSeq() BleSeq {
	return BleSeq(atomic.AddUint32(&nextSeq, 1))
}

func BhdTimeoutError(rspType MsgType) error {
	str := fmt.Sprintf("Timeout waiting for blehostd to send %s response",
		MsgTypeToString(rspType))

	log.Debug(str)
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
		Data: e.Data.Bytes,

		Flags:               e.DataFlags,
		Uuids16:             e.DataUuids16,
		Uuids16IsComplete:   e.DataUuids16IsComplete,
		Uuids32:             e.DataUuids32,
		Uuids32IsComplete:   e.DataUuids32IsComplete,
		Uuids128:            e.DataUuids128,
		Uuids128IsComplete:  e.DataUuids128IsComplete,
		Name:                e.DataName,
		NameIsComplete:      e.DataNameIsComplete,
		TxPwrLvl:            e.DataTxPwrLvl,
		TxPwrLvlIsPresent:   e.DataTxPwrLvlIsPresent,
		SlaveItvlMin:        e.DataSlaveItvlMin,
		SlaveItvlMax:        e.DataSlaveItvlMax,
		SlaveItvlIsPresent:  e.DataSlaveItvlIsPresent,
		SvcDataUuid16:       e.DataSvcDataUuid16.Bytes,
		PublicTgtAddrs:      e.DataPublicTgtAddrs,
		Appearance:          e.DataAppearance,
		AppearanceIsPresent: e.DataAppearanceIsPresent,
		AdvItvl:             e.DataAdvItvl,
		AdvItvlIsPresent:    e.DataAdvItvlIsPresent,
		SvcDataUuid32:       e.DataSvcDataUuid32.Bytes,
		SvcDataUuid128:      e.DataSvcDataUuid128.Bytes,
		Uri:                 e.DataUri.Bytes,
		MfgData:             e.DataMfgData.Bytes,
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

func ConnFindXact(x *BleXport, connHandle uint16) (BleConnDesc, error) {
	r := NewBleConnFindReq()
	r.ConnHandle = connHandle

	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewBleListener()
	if err := x.Bd.AddListener(base, bl); err != nil {
		return BleConnDesc{}, err
	}
	defer x.Bd.RemoveListener(base)

	return connFind(x, bl, r)
}

func GenRandAddrXact(x *BleXport) (BleAddr, error) {
	r := NewBleGenRandAddrReq()
	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewBleListener()
	if err := x.Bd.AddListener(base, bl); err != nil {
		return BleAddr{}, err
	}
	defer x.Bd.RemoveListener(base)

	return genRandAddr(x, bl, r)
}

func SetRandAddrXact(x *BleXport, addr BleAddr) error {
	r := NewBleSetRandAddrReq()
	r.Addr = addr

	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewBleListener()
	if err := x.Bd.AddListener(base, bl); err != nil {
		return err
	}
	defer x.Bd.RemoveListener(base)

	return setRandAddr(x, bl, r)
}

func SetPreferredMtuXact(x *BleXport, mtu uint16) error {
	r := NewBleSetPreferredMtuReq()
	r.Mtu = mtu

	base := BleMsgBase{
		Op:         -1,
		Type:       -1,
		Seq:        r.Seq,
		ConnHandle: -1,
	}

	bl := NewBleListener()
	if err := x.Bd.AddListener(base, bl); err != nil {
		return err
	}
	defer x.Bd.RemoveListener(base)

	return setPreferredMtu(x, bl, r)
}
