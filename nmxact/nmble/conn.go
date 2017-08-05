package nmble

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

type Notification struct {
	Chr        *Characteristic
	Data       []byte
	Indication bool
}

type NotifyListener struct {
	NotifyChan chan Notification
	ErrChan    chan error
}

func NewNotifyListener() *NotifyListener {
	return &NotifyListener{
		NotifyChan: make(chan Notification),
		ErrChan:    make(chan error),
	}
}

type Conn struct {
	bx      *BleXport
	rxvr    *Receiver
	attMtu  uint16
	profile Profile
	desc    BleConnDesc

	connHandle     uint16
	connecting     bool
	disconnectChan chan error
	wg             sync.WaitGroup

	// Terminates all go routines.  Gets set to null after disconnect.
	stopChan chan struct{}

	notifyMap map[*Characteristic][](*NotifyListener)

	// Protects:
	// * connHandle
	// * connecting
	// * notifyMap
	// * stopChan
	mtx sync.Mutex
}

func NewConn(bx *BleXport) *Conn {
	return &Conn{
		bx:             bx,
		rxvr:           NewReceiver(nmxutil.GetNextId(), bx, 1),
		connHandle:     BLE_CONN_HANDLE_NONE,
		attMtu:         BLE_ATT_MTU_DFLT,
		disconnectChan: make(chan error, 1),
		stopChan:       make(chan struct{}),
		notifyMap:      map[*Characteristic][](*NotifyListener){},
	}
}

func (c *Conn) DisconnectChan() <-chan error {
	return c.disconnectChan
}

func (c *Conn) initiateShutdown() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.stopChan == nil {
		return false
	}

	close(c.stopChan)
	c.stopChan = nil

	return true
}

func (c *Conn) abortNotifyListeners(err error) {
	// No need to lock mutex; this should only be called after all go routines
	// have terminated.
	for _, nls := range c.notifyMap {
		for _, nl := range nls {
			nl.ErrChan <- err
			close(nl.NotifyChan)
			close(nl.ErrChan)
		}
	}
}

func (c *Conn) shutdown(err error) {
	if !c.initiateShutdown() {
		return
	}

	c.connecting = false
	c.connHandle = BLE_CONN_HANDLE_NONE

	c.bx.StopWaitingForMaster(c, err)

	c.rxvr.RemoveAll("shutdown")
	c.rxvr.WaitUntilNoListeners()

	c.wg.Wait()

	c.abortNotifyListeners(err)

	c.disconnectChan <- err
	close(c.disconnectChan)
}

func (c *Conn) newDisconnectError(reason int) error {
	str := fmt.Sprintf("BLE peer disconnected; "+
		"reason=\"%s\" (%d) connection=%s",
		ErrCodeToString(reason), reason, c.desc.String())

	return nmxutil.NewBleSesnDisconnectError(reason, str)
}

// Listens for events in the background.
func (c *Conn) eventListen(bl *Listener) error {
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()
		defer c.rxvr.RemoveListener("connect", bl)

		for {
			select {
			case <-c.stopChan:
				return

			case err := <-bl.ErrChan:
				go c.shutdown(err)
				return

			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleMtuChangeEvt:
					if msg.Status != 0 {
						err := StatusError(MSG_OP_EVT,
							MSG_TYPE_MTU_CHANGE_EVT,
							msg.Status)
						log.Debugf(err.Error())
					} else {
						log.Debugf("BLE ATT MTU updated; from=%d to=%d",
							c.attMtu, msg.Mtu)
						c.attMtu = msg.Mtu
					}

				case *BleEncChangeEvt:
					var err error
					if msg.Status != 0 {
						err = StatusError(MSG_OP_EVT,
							MSG_TYPE_ENC_CHANGE_EVT,
							msg.Status)
						log.Debugf(err.Error())
					} else {
						log.Debugf("Connection encrypted; conn_handle=%d",
							msg.ConnHandle)
					}

				case *BleDisconnectEvt:
					go c.shutdown(c.newDisconnectError(msg.Reason))
					return

				default:
				}
			}
		}
	}()
	return nil
}

func (c *Conn) rxNotify(msg *BleNotifyRxEvt) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	chr := c.profile.FindChrByHandle(uint16(msg.AttrHandle))
	if chr == nil {
		return
	}

	nls := c.notifyMap[chr]
	if nls == nil {
		return
	}

	n := Notification{
		Chr:        chr,
		Data:       msg.Data.Bytes,
		Indication: msg.Indication,
	}
	for _, nl := range nls {
		nl.NotifyChan <- n
	}
}

// Listens for incoming notifications and indications.
func (c *Conn) notifyListen() error {
	key := TchKey(MSG_TYPE_NOTIFY_RX_EVT, int(c.connHandle))
	bl, err := c.rxvr.AddListener("notifications", key)
	if err != nil {
		return err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.rxvr.RemoveListener("notifications", bl)

		for {
			select {
			case <-c.stopChan:
				return

			case <-bl.ErrChan:
				return

			case bm := <-bl.MsgChan:
				switch msg := bm.(type) {
				case *BleNotifyRxEvt:
					c.rxNotify(msg)
				}
			}
		}
	}()

	return nil
}

func (c *Conn) startConnecting() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.stopChan == nil {
		return fmt.Errorf("Attempt to re-use conn object")
	}

	if c.connHandle != BLE_CONN_HANDLE_NONE {
		return nmxutil.NewSesnAlreadyOpenError(
			"BLE connection already established")
	}
	if c.connecting {
		return nmxutil.NewSesnAlreadyOpenError(
			"BLE connection already being established")
	}

	c.connecting = true
	return nil
}

func (c *Conn) stopConnecting() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.connecting = false
}

func (c *Conn) finalizeConnection(connHandle uint16,
	eventListener *Listener) error {

	c.mtx.Lock()
	c.connecting = false
	c.connHandle = connHandle
	c.mtx.Unlock()

	// Listen for events in the background.
	if err := c.eventListen(eventListener); err != nil {
		return err
	}

	// Listen for notifications in the background.
	if err := c.notifyListen(); err != nil {
		return err
	}

	d, err := ConnFindXact(c.bx, c.connHandle)
	if err != nil {
		return err
	}
	c.desc = d

	return nil
}

func (c *Conn) IsConnected() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	return c.connHandle != BLE_CONN_HANDLE_NONE
}

func (c *Conn) Connect(bx *BleXport, ownAddrType BleAddrType, peer BleDev,
	timeout time.Duration) error {

	if err := c.startConnecting(); err != nil {
		return err
	}
	defer c.stopConnecting()

	r := NewBleConnectReq()
	r.OwnAddrType = ownAddrType
	r.PeerAddrType = peer.AddrType
	r.PeerAddr = peer.Addr

	// Initiating a connection requires dedicated master privileges.
	if err := c.bx.AcquireMaster(c); err != nil {
		return err
	}
	defer c.bx.ReleaseMaster()

	bl, err := c.rxvr.AddListener("connect", SeqKey(r.Seq))
	if err != nil {
		return err
	}

	// Tell blehostd to initiate connection.
	connHandle, err := connect(c.bx, bl, r, timeout)
	if err != nil {
		bhe := nmxutil.ToBleHost(err)
		if bhe != nil && bhe.Status == ERR_CODE_EDONE {
			// Already connected.
			c.rxvr.RemoveListener("connect", bl)
			return fmt.Errorf("Already connected to peer %s", peer)
		} else if !nmxutil.IsXport(err) {
			// The transport did not restart; always attempt to cancel the
			// connect operation.  In most cases, the host has already stopped
			// connecting and will respond with an "ealready" error that can be
			// ignored.
			if err := c.connCancel(); err != nil {
				log.Errorf("Failed to cancel connect in progress: %s",
					err.Error())
			}
		}

		c.rxvr.RemoveListener("connect", bl)
		return err
	}

	if err := c.finalizeConnection(connHandle, bl); err != nil {
		return err
	}

	return nil
}

func (c *Conn) Inherit(connHandle uint16, bl *Listener) error {
	if err := c.startConnecting(); err != nil {
		return err
	}

	if err := c.finalizeConnection(connHandle, bl); err != nil {
		return err
	}

	return nil
}

func (c *Conn) ConnInfo() BleConnDesc {
	return c.desc
}

func (c *Conn) AttMtu() uint16 {
	return c.attMtu
}

func (c *Conn) discAllDscsOnce(startHandle uint16, endHandle uint16) (
	[]*Descriptor, error) {

	r := NewBleDiscAllDscsReq()
	r.ConnHandle = c.connHandle
	r.StartHandle = int(startHandle)
	r.EndHandle = int(endHandle)

	bl, err := c.rxvr.AddListener("disc-all-dscs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-dscs", bl)

	rawDscs, err := discAllDscs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	dscs := make([]*Descriptor, len(rawDscs))
	for i, rd := range rawDscs {
		dscs[i] = &Descriptor{
			Uuid:   rd.Uuid,
			Handle: uint16(rd.Handle),
		}
	}

	return dscs, nil
}

func (c *Conn) discAllDscs(chrs []*Characteristic, svcEndHandle uint16) error {
	for i, _ := range chrs {
		chr := chrs[i]

		// Only discover descriptors if this characteristic has a CCCD.
		if chr.SubscribeType() != 0 {
			var endHandle uint16
			if i < len(chrs)-1 {
				endHandle = chrs[i+1].DefHandle - 1
			} else {
				endHandle = svcEndHandle
			}

			dscs, err := c.discAllDscsOnce(chrs[i].ValHandle, endHandle)
			if err != nil {
				return err
			}

			chr.Dscs = dscs
		}
	}

	return nil
}

func (c *Conn) discAllChrsOnce(svc Service) ([]*Characteristic, error) {
	r := NewBleDiscAllChrsReq()
	r.ConnHandle = c.connHandle
	r.StartHandle = int(svc.StartHandle)
	r.EndHandle = int(svc.EndHandle)

	bl, err := c.rxvr.AddListener("disc-all-chrs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-chrs", bl)

	rawChrs, err := discAllChrs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	chrs := make([]*Characteristic, len(rawChrs))
	for i, rc := range rawChrs {
		chrs[i] = &Characteristic{
			Uuid:       rc.Uuid,
			DefHandle:  uint16(rc.DefHandle),
			ValHandle:  uint16(rc.ValHandle),
			Properties: BleChrFlags(rc.Properties),
		}
	}

	if err := c.discAllDscs(chrs, svc.EndHandle); err != nil {
		return nil, err
	}

	return chrs, nil
}

func (c *Conn) discAllChrs(svcs []Service) error {
	for i, _ := range svcs {
		chrs, err := c.discAllChrsOnce(svcs[i])
		if err != nil {
			return err
		}

		svcs[i].Chrs = chrs
	}

	return nil
}

func (c *Conn) discAllSvcs() ([]Service, error) {
	r := NewBleDiscAllSvcsReq()
	r.ConnHandle = c.connHandle

	bl, err := c.rxvr.AddListener("disc-all-svcs", SeqKey(r.Seq))
	if err != nil {
		return nil, err
	}
	defer c.rxvr.RemoveListener("disc-all-svcs", bl)

	rawSvcs, err := discAllSvcs(c.bx, bl, r)
	if err != nil {
		return nil, err
	}

	svcs := make([]Service, len(rawSvcs))
	for i, rs := range rawSvcs {
		svcs[i] = Service{
			Uuid:        rs.Uuid,
			StartHandle: uint16(rs.StartHandle),
			EndHandle:   uint16(rs.EndHandle),
		}
	}

	return svcs, nil
}

func (c *Conn) DiscoverSvcs() error {
	svcs, err := c.discAllSvcs()
	if err != nil {
		return err
	}

	if err := c.discAllChrs(svcs); err != nil {
		return err
	}

	c.profile.SetServices(svcs)

	return nil
}

func (c *Conn) Profile() *Profile {
	return &c.profile
}

func isExchangeMtuError(err error) bool {
	if err == nil {
		return false
	}

	// If the operation failed because the peer already initiated the
	// exchange, just pretend it was successful.
	bhe := nmxutil.ToBleHost(err)
	if bhe == nil {
		return true
	}

	switch bhe.Status {
	case ERR_CODE_EALREADY:
		return false
	case ERR_CODE_ATT_BASE + ERR_CODE_ATT_REQ_NOT_SUPPORTED:
		return false
	default:
		return true
	}
}

func (c *Conn) ExchangeMtu() error {
	r := NewBleExchangeMtuReq()
	r.ConnHandle = c.connHandle

	bl, err := c.rxvr.AddListener("exchange-mtu", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("exchange-mtu", bl)

	mtu, err := exchangeMtu(c.bx, bl, r)
	if isExchangeMtuError(err) {
		return err
	}

	c.attMtu = uint16(mtu)
	return nil
}

func (c *Conn) WriteHandle(handle uint16, payload []byte,
	name string) error {

	r := NewBleWriteReq()
	r.ConnHandle = c.connHandle
	r.AttrHandle = int(handle)
	r.Data.Bytes = payload

	bl, err := c.rxvr.AddListener(name, SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener(name, bl)

	if err := write(c.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (c *Conn) WriteHandleNoRsp(handle uint16, payload []byte,
	name string) error {

	r := NewBleWriteCmdReq()
	r.ConnHandle = c.connHandle
	r.AttrHandle = int(handle)
	r.Data.Bytes = payload

	bl, err := c.rxvr.AddListener(name, SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener(name, bl)

	if err := writeCmd(c.bx, bl, r); err != nil {
		return err
	}

	return nil
}

func (c *Conn) WriteChr(chr *Characteristic, payload []byte,
	name string) error {

	return c.WriteHandle(chr.ValHandle, payload, name)
}

func (c *Conn) WriteChrNoRsp(chr *Characteristic, payload []byte,
	name string) error {

	return c.WriteHandleNoRsp(chr.ValHandle, payload, name)
}

func (c *Conn) Subscribe(chr *Characteristic) error {
	uuid := BleUuid{CccdUuid, [16]byte{}}
	dsc := FindDscByUuid(chr, uuid)
	if dsc == nil {
		return fmt.Errorf("Cannot subscribe to characteristic %s; no CCCD",
			chr.Uuid.String())
	}

	var payload []byte
	switch chr.SubscribeType() {
	case BLE_GATT_F_NOTIFY:
		payload = []byte{1, 0}
	case BLE_GATT_F_INDICATE:
		payload = []byte{2, 0}
	default:
		return fmt.Errorf("Cannot subscribe to characteristic %s; "+
			"properties indicate unsubscribable", chr.Uuid.String())
	}

	return c.WriteHandle(dsc.Handle, payload, "subscribe")
}

func (c *Conn) ListenForNotifications(chr *Characteristic) *NotifyListener {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	nl := NewNotifyListener()
	slice := c.notifyMap[chr]

	slice = append(slice, nl)
	c.notifyMap[chr] = slice

	return nl
}

func (c *Conn) terminate() error {
	r := NewBleTerminateReq()
	r.ConnHandle = c.connHandle
	r.HciReason = ERR_CODE_HCI_REM_USER_CONN_TERM

	bl, err := c.rxvr.AddListener("terminate", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("terminate", bl)

	if err := terminate(c.bx, bl, r); err != nil {
		// Ignore ealready errors.
		bhdErr := nmxutil.ToBleHost(err)
		if bhdErr == nil || bhdErr.Status != ERR_CODE_EALREADY {
			return err
		}
	}

	return nil
}

func (c *Conn) connCancel() error {
	r := NewBleConnCancelReq()

	bl, err := c.rxvr.AddListener("conn-cancel", SeqKey(r.Seq))
	if err != nil {
		return err
	}
	defer c.rxvr.RemoveListener("conn-cancel", bl)

	if err := connCancel(c.bx, bl, r); err != nil {
		// Ignore ealready errors.
		bhdErr := nmxutil.ToBleHost(err)
		if bhdErr == nil || bhdErr.Status != ERR_CODE_EALREADY {
			return err
		}
	}

	return nil
}

func (c *Conn) Stop() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	var err error
	if c.connHandle != BLE_CONN_HANDLE_NONE {
		err = c.terminate()
	} else if c.connecting {
		err = c.connCancel()
	}

	if err != nil {
		// Something went unexpectedly wrong.  Just force a shutdown.
		go c.shutdown(err)
	}

	return nil
}
