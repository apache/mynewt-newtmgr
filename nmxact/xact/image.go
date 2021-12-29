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

package xact

import (
	"crypto/sha256"
	"fmt"

	pb "gopkg.in/cheggaaa/pb.v1"

	log "github.com/sirupsen/logrus"
	"mynewt.apache.org/newtmgr/nmxact/mgmt"
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
	"mynewt.apache.org/newtmgr/nmxact/sesn"
	"sync"
	"sync/atomic"
	"time"
)

//////////////////////////////////////////////////////////////////////////////
// $upload                                                                  //
//////////////////////////////////////////////////////////////////////////////
const IMAGE_UPLOAD_MAX_CHUNK = 512
const IMAGE_UPLOAD_MIN_1ST_CHUNK = 32
const IMAGE_UPLOAD_STATUS_MISSED = -1
const IMAGE_UPLOAD_CHUNK_MISSED_WM = -1
const IMAGE_UPLOAD_START_WS = 1
const IMAGE_UPLOAD_DEF_MAX_WS = 5
const IMAGE_UPLOAD_STATUS_EXPECTED = 0
const IMAGE_UPLOAD_STATUS_RQ = 1

type ImageUploadProgressFn func(c *ImageUploadCmd, r *nmp.ImageUploadRsp)
type ImageUploadCmd struct {
	CmdBase
	Data       []byte
	StartOff   int
	Upgrade    bool
	ProgressCb ImageUploadProgressFn
	ImageNum   int
	MaxWinSz   int
}

type ImageUploadIntTracker struct {
	Mutex    sync.Mutex
	TuneWS   bool
	RspMap   map[int]int
	WCount   int
	WCap     int
	Off      int
	MaxRxOff int32
}

type ImageUploadResult struct {
	Rsps []*nmp.ImageUploadRsp
}

func NewImageUploadCmd() *ImageUploadCmd {
	return &ImageUploadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageUploadResult() *ImageUploadResult {
	return &ImageUploadResult{}
}

func (r *ImageUploadResult) Status() int {
	if len(r.Rsps) > 0 {
		return r.Rsps[len(r.Rsps)-1].Rc
	} else {
		return nmp.NMP_ERR_EUNKNOWN
	}
}

func buildImageUploadReq(imageSz int, hash []byte, upgrade bool, chunk []byte,
	off int, imageNum int, seq uint8) *nmp.ImageUploadReq {

	r := nmp.NewImageUploadReqWithSeq(seq)

	if off == 0 {
		r.Len = uint32(imageSz)
		r.DataSha = hash
		r.Upgrade = upgrade
	}
	r.Off = uint32(off)
	r.Data = chunk
	r.ImageNum = uint8(imageNum)

	return r
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func encodeUploadReq(s sesn.Sesn, hash []byte, upgrade bool, data []byte,
	off int, chunklen int, imageNum int, seq uint8) ([]byte, error) {

	r := buildImageUploadReq(len(data), hash, upgrade, data[off:off+chunklen],
		off, imageNum, seq)
	enc, err := mgmt.EncodeMgmt(s, r.Msg())
	if err != nil {
		return nil, err
	}

	return enc, nil
}

func findChunkLen(s sesn.Sesn, hash []byte, upgrade bool, data []byte,
	off int, imageNum int, seq uint8) (int, error) {

	// Let's start by encoding max allowed chunk len and we will see how many
	// bytes we need to cut
	chunklen := min(len(data)-off, IMAGE_UPLOAD_MAX_CHUNK)

	// Keep reducing the chunk size until the request fits the MTU.
	for {
		enc, err := encodeUploadReq(s, hash, upgrade, data, off, chunklen, imageNum, seq)
		if err != nil {
			return 0, err
		}

		if len(enc) <= (s.MtuOut()) {
			break
		}

		// Encoded length is larger than MTU, we need to make chunk shorter
		overflow := len(enc) - s.MtuOut()
		chunklen -= overflow
	}

	return chunklen, nil
}

func nextImageUploadReq(s sesn.Sesn, upgrade bool, data []byte, off int, imageNum int) (
	*nmp.ImageUploadReq, error) {
	var hash []byte = nil

	// Ensure we produce consistent requests while we calculate the chunk
	// length.
	txFilter, _ := s.Filters()
	if txFilter != nil {
		txFilter.Freeze()
		defer txFilter.Unfreeze()
	}

	// For 1st chunk we'll need valid data hash
	if off == 0 {
		sha := sha256.Sum256(data)
		hash = sha[:]
	}

	seq := nmxutil.NextNmpSeq()

	// Find chunk length
	chunklen, err := findChunkLen(s, hash, upgrade, data, off, imageNum, seq)
	if err != nil {
		return nil, err
	}

	// For 1st chunk we need to send at least full header so if it does not
	// fit we'll recalculate without hash
	if off == 0 && chunklen < IMAGE_UPLOAD_MIN_1ST_CHUNK {
		hash = nil
		chunklen, err = findChunkLen(s, hash, upgrade, data, off, imageNum, seq)
		if err != nil {
			return nil, err
		}
	}

	// If calculated chunk length is not enough to send at least single byte
	// we can't do much more...
	if chunklen <= 0 {
		return nil, fmt.Errorf("Cannot create image upload request; "+
			"MTU too low to fit any image data; max-payload-size=%d chunklen %d",
			s.MtuOut(), chunklen)
	}

	r := buildImageUploadReq(len(data), hash, upgrade,
		data[off:off+chunklen], off, imageNum, seq)

	// Request above should encode just fine since we calculate proper chunk
	// length but (at least for now) let's double check it
	enc, err := mgmt.EncodeMgmt(s, r.Msg())
	if err != nil {
		return nil, err
	}
	if len(enc) > s.MtuOut() {
		return nil, fmt.Errorf("Invalid chunk length; payload-size=%d "+
			"max-payload-size=%d", len(enc), s.MtuOut())
	}

	return r, nil
}

func (t *ImageUploadIntTracker) UpdateTracker(off int, status int) {
	if status == IMAGE_UPLOAD_STATUS_MISSED {
		/* Upon error, set the value to missed for retransmission */
		t.RspMap[off] = IMAGE_UPLOAD_CHUNK_MISSED_WM
	} else if status == IMAGE_UPLOAD_STATUS_EXPECTED {
		/* When the chunk at a certain offset is transmitted,
		   a response requesting the next offset is expected. This
		   indicates that the chunk is successfully trasmitted. Wait
		   on the chunk in response e.g when offset 0, len 100 is sent,
		   expected offset in the ack is 100 etc. */
		t.RspMap[off] = 1
	} else if status == IMAGE_UPLOAD_STATUS_RQ {
		/* If the chunk at this offset was already transmitted, value
		   goes to zero and that KV pair gets cleaned up subsequently.
		   If there is a repeated request for a certain offset,
		   that offset is not received by the remote side. Decrement
		   the value. Missed chunk processing routine retransmits it */
		t.RspMap[off] -= 1
	}
}

func (t *ImageUploadIntTracker) CheckWindow() bool {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()

	return t.WCount < t.WCap
}

func (t *ImageUploadIntTracker) ProcessMissedChunks() {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	for o, c := range t.RspMap {
		if c < IMAGE_UPLOAD_CHUNK_MISSED_WM {
			delete(t.RspMap, o)
			t.Off = o
			log.Debugf("missed? off %d count %d", o, c)
		}
		// clean up done chunks
		if c == 0 {
			delete(t.RspMap, o)
		}
	}
}

func (t *ImageUploadIntTracker) HandleResponse(c *ImageUploadCmd, rsp nmp.NmpRsp, res *ImageUploadResult) bool {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	wFull := false

	if rsp != nil {
		irsp := rsp.(*nmp.ImageUploadRsp)
		res.Rsps = append(res.Rsps, irsp)
		t.UpdateTracker(int(irsp.Off), IMAGE_UPLOAD_STATUS_RQ)

		if t.MaxRxOff < int32(irsp.Off) {
			t.MaxRxOff = int32(irsp.Off)
		}
		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}
	}

	if t.WCap == t.WCount {
		wFull = true
	}

	if t.TuneWS && t.WCap < c.MaxWinSz {
		t.WCap += 1
	}
	t.WCount -= 1

	// Indicate transition from window being full to with open slot(s)
	if wFull && t.WCap > t.WCount {
		return true
	} else {
		return false
	}
}

func (t *ImageUploadIntTracker) HandleError(off int, err error) bool {
	/*XXX: there could be an Unauthorize or EOF error  when the rate is too high
	  due to a large window, we retry. example:
	  "failed to decrypt message: coap_sec_tunnel: decode GCM fail EOF"
	  Since the error is sent with fmt.Errorf() API, with no code,
	  the string may have to be parsed to know the particular error */
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	log.Debugf("HandleError off %v error %v", off, err)
	var wFull = false
	if t.WCap == t.WCount {
		wFull = true
	}

	if t.WCount > IMAGE_UPLOAD_START_WS+1 {
		t.WCap -= 1
	}
	t.TuneWS = false
	t.WCount -= 1
	t.UpdateTracker(off, IMAGE_UPLOAD_STATUS_MISSED)

	// Indicate transition from window being full to with open slot(s)
	if wFull && t.WCap > t.WCount {
		return true
	} else {
		return false
	}
}

func (c *ImageUploadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newImageUploadResult()
	ch := make(chan int)
	rspc := make(chan nmp.NmpRsp, c.MaxWinSz)
	errc := make(chan error, c.MaxWinSz)

	t := ImageUploadIntTracker{
		TuneWS:   true,
		WCount:   0,
		WCap:     IMAGE_UPLOAD_START_WS,
		Off:      c.StartOff,
		RspMap:   make(map[int]int),
		MaxRxOff: 0,
	}

	for int(atomic.LoadInt32(&t.MaxRxOff)) < len(c.Data) {
		// Block if window is full
		if !t.CheckWindow() {
			ch <- 1
		}

		t.ProcessMissedChunks()

		if t.Off == len(c.Data) {
			continue
		}

		t.Mutex.Lock()
		r, err := nextImageUploadReq(s, c.Upgrade, c.Data, t.Off, c.ImageNum)
		if err != nil {
			t.Mutex.Unlock()
			return nil, err
		}

		t.Off = (int(r.Off) + len(r.Data))

		// Use up a chunk in window
		t.WCount += 1
		err = txReqAsync(s, r.Msg(), &c.CmdBase, rspc, errc)
		if err != nil {
			log.Debugf("err txReqAsync %v", err)
			t.Mutex.Unlock()
			break
		}
		// Mark the expected offset in successful tx of this chunk. i.e off + len
		t.UpdateTracker(int(r.Off)+len(r.Data), IMAGE_UPLOAD_STATUS_EXPECTED)
		t.Mutex.Unlock()

		go func(off int) {
			select {
			case err := <-errc:
				sig := t.HandleError(off, err)
				if sig {
					<-ch
				}
				return
			case rsp := <-rspc:
				sig := t.HandleResponse(c, rsp, res)
				if sig {
					<-ch
				}
				return
			}
		}(int(r.Off))
	}

	if int(t.MaxRxOff) == len(c.Data) {
		return res, nil
	} else {
		return nil, fmt.Errorf("ImageUpload unexpected error after %d/%d bytes",
			t.MaxRxOff, len(c.Data))
	}
}

//////////////////////////////////////////////////////////////////////////////
// $upgrade                                                                 //
//////////////////////////////////////////////////////////////////////////////

// Image upgrade combines the image erase and image upload commands into a
// single command.  Some hardware and / or BLE connection settings cause the
// connection to drop while flash is being erased or written to.  The image
// upgrade command addresses this issue with the following sequence:
// 1. Send image erase command.
// 2. If the image erase command succeeded, proceed to step 5.
// 3. Else if the peer is disconnected, attempt to reconnect to the peer.  If
//    the reconnect attempt fails, abort the command and report the error.  If
//    the reconnect attempt succeeded, proceed to step 5.
// 4. Else (the erase command failed and the peer is still connected), proceed
//    to step 5.
// 5. Execute the upload command.  If the connection drops before the final
//    part is uploaded, reconnect and retry the previous part.

type ImageUpgradeCmd struct {
	CmdBase
	Data        []byte
	NoErase     bool
	ResetDevice bool
	Hash        []byte
	ProgressCb  ImageUploadProgressFn
	LastOff     uint32
	Upgrade     bool
	ProgressBar *pb.ProgressBar
	ImageNum    int
	MaxWinSz    int
}

type ImageUpgradeResult struct {
	EraseRes  *ImageEraseResult
	UploadRes *ImageUploadResult
}

func NewImageUpgradeCmd() *ImageUpgradeCmd {
	return &ImageUpgradeCmd{
		CmdBase:  NewCmdBase(),
		NoErase:  false,
		ResetDevice: false,
		ImageNum: 0,
	}
}

func newImageUpgradeResult() *ImageUpgradeResult {
	return &ImageUpgradeResult{}
}

func (r *ImageUpgradeResult) Status() int {
	if r.UploadRes != nil {
		return r.UploadRes.Status()
	} else if r.EraseRes != nil {
		return r.EraseRes.Status()
	} else {
		return nmp.NMP_ERR_EUNKNOWN
	}
}

// Attempts to recover from a disconnect.
func (c *ImageUpgradeCmd) rescue(s sesn.Sesn, err error) error {
	if err != nil {
		if !s.IsOpen() {
			if err := s.Open(); err == nil {
				return nil
			}
		}
	}

	return err
}

func (c *ImageUpgradeCmd) runErase(s sesn.Sesn) (*ImageEraseResult, error) {
	cmd := NewImageEraseCmd()
	cmd.SetTxOptions(c.TxOptions())
	res, err := cmd.Run(s)

	if err := c.rescue(s, err); err != nil {
		return nil, err
	}

	if res == nil {
		// We didn't get a response back but we rescued ourselves from the
		// disconnect.
		res = newImageEraseResult()
	}

	return res.(*ImageEraseResult), nil
}

func (c *ImageUpgradeCmd) runUpload(s sesn.Sesn) (*ImageUploadResult, error) {
	startOff := 0
	progressCb := func(uc *ImageUploadCmd, r *nmp.ImageUploadRsp) {
		if r.Rc == 0 {
			startOff = int(r.Off)
		}
		c.ProgressCb(uc, r)
	}

	for {
		var opt = sesn.TxOptions{
			Timeout: 3 * time.Second,
			Tries:   1,
		}
		cmd := NewImageUploadCmd()
		cmd.Data = c.Data
		cmd.StartOff = startOff
		cmd.Upgrade = c.Upgrade
		cmd.ProgressCb = progressCb
		cmd.ImageNum = c.ImageNum
		cmd.SetTxOptions(opt)
		cmd.MaxWinSz = c.MaxWinSz

		res, err := cmd.Run(s)
		if err == nil {
			return res.(*ImageUploadResult), nil
		}

		if err := c.rescue(s, err); err != nil {
			// Disconnected and couldn't recover.
			return nil, err
		}

		// Disconnected but recovered; retry last part.
	}
}

func (c *ImageUpgradeCmd) Run(s sesn.Sesn) (Result, error) {
	var eres *ImageEraseResult = nil
	var err error

	if c.NoErase == false {
		eres, err = c.runErase(s)
		if err != nil {
			return nil, err
		}
	} else {
		eres = nil
	}
	ures, err := c.runUpload(s)
	if err != nil {
		fmt.Printf("failed\n")
		return nil, err
	}

	if len(c.Hash) > 0 {
		fmt.Printf("set image %x to test\n", c.Hash)
		t := nmp.NewImageStateWriteReq()
		t.Hash = c.Hash
		t.Confirm = false
	
		_, err = txReq(s, t.Msg(), &c.CmdBase)
		if err != nil {
			fmt.Printf("failed\n")
			return nil, err
		}
	}

	if c.ResetDevice == true {
		fmt.Printf("reset device\n")
		r := nmp.NewResetReq()
		_, err = txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			fmt.Printf("failed\n")
			return nil, err
		}
	}

	upgradeRes := newImageUpgradeResult()
	upgradeRes.EraseRes = eres
	upgradeRes.UploadRes = ures
	return upgradeRes, nil
}

//////////////////////////////////////////////////////////////////////////////
// $state read                                                              //
//////////////////////////////////////////////////////////////////////////////

type ImageStateReadCmd struct {
	CmdBase
}

type ImageStateReadResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateReadCmd() *ImageStateReadCmd {
	return &ImageStateReadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageStateReadResult() *ImageStateReadResult {
	return &ImageStateReadResult{}
}

func (r *ImageStateReadResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateReadCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateReadReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateReadResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $state write                                                             //
//////////////////////////////////////////////////////////////////////////////

type ImageStateWriteCmd struct {
	CmdBase
	Hash    []byte
	Confirm bool
}

type ImageStateWriteResult struct {
	Rsp *nmp.ImageStateRsp
}

func NewImageStateWriteCmd() *ImageStateWriteCmd {
	return &ImageStateWriteCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageStateWriteResult() *ImageStateWriteResult {
	return &ImageStateWriteResult{}
}

func (r *ImageStateWriteResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageStateWriteCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageStateWriteReq()
	r.Hash = c.Hash
	r.Confirm = c.Confirm

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageStateRsp)

	res := newImageStateWriteResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $corelist                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreListCmd struct {
	CmdBase
}

type CoreListResult struct {
	Rsp *nmp.CoreListRsp
}

func NewCoreListCmd() *CoreListCmd {
	return &CoreListCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreListResult() *CoreListResult {
	return &CoreListResult{}
}

func (r *CoreListResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreListCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreListReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreListRsp)

	res := newCoreListResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $erase                                                                   //
//////////////////////////////////////////////////////////////////////////////

type ImageEraseCmd struct {
	CmdBase
}

type ImageEraseResult struct {
	Rsp *nmp.ImageEraseRsp
}

func NewImageEraseCmd() *ImageEraseCmd {
	return &ImageEraseCmd{
		CmdBase: NewCmdBase(),
	}
}

func newImageEraseResult() *ImageEraseResult {
	return &ImageEraseResult{}
}

func (r *ImageEraseResult) Status() int {
	return r.Rsp.Rc
}

func (c *ImageEraseCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewImageEraseReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.ImageEraseRsp)

	res := newImageEraseResult()
	res.Rsp = srsp
	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreload                                                                //
//////////////////////////////////////////////////////////////////////////////

type CoreLoadProgressFn func(c *CoreLoadCmd, r *nmp.CoreLoadRsp)
type CoreLoadCmd struct {
	CmdBase
	ProgressCb CoreLoadProgressFn
}

type CoreLoadResult struct {
	Rsps []*nmp.CoreLoadRsp
}

func NewCoreLoadCmd() *CoreLoadCmd {
	return &CoreLoadCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreLoadResult() *CoreLoadResult {
	return &CoreLoadResult{}
}

func (r *CoreLoadResult) Status() int {
	rsp := r.Rsps[len(r.Rsps)-1]
	return rsp.Rc
}

func (c *CoreLoadCmd) Run(s sesn.Sesn) (Result, error) {
	res := newCoreLoadResult()
	off := 0

	for {
		r := nmp.NewCoreLoadReq()
		r.Off = uint32(off)

		rsp, err := txReq(s, r.Msg(), &c.CmdBase)
		if err != nil {
			return nil, err
		}
		irsp := rsp.(*nmp.CoreLoadRsp)

		if c.ProgressCb != nil {
			c.ProgressCb(c, irsp)
		}

		res.Rsps = append(res.Rsps, irsp)
		if irsp.Rc != 0 {
			break
		}

		if len(irsp.Data) == 0 {
			// Download complete.
			break
		}

		off = int(irsp.Off) + len(irsp.Data)
	}

	return res, nil
}

//////////////////////////////////////////////////////////////////////////////
// $coreerase                                                               //
//////////////////////////////////////////////////////////////////////////////

type CoreEraseCmd struct {
	CmdBase
}

type CoreEraseResult struct {
	Rsp *nmp.CoreEraseRsp
}

func NewCoreEraseCmd() *CoreEraseCmd {
	return &CoreEraseCmd{
		CmdBase: NewCmdBase(),
	}
}

func newCoreEraseResult() *CoreEraseResult {
	return &CoreEraseResult{}
}

func (r *CoreEraseResult) Status() int {
	return r.Rsp.Rc
}

func (c *CoreEraseCmd) Run(s sesn.Sesn) (Result, error) {
	r := nmp.NewCoreEraseReq()

	rsp, err := txReq(s, r.Msg(), &c.CmdBase)
	if err != nil {
		return nil, err
	}
	srsp := rsp.(*nmp.CoreEraseRsp)

	res := newCoreEraseResult()
	res.Rsp = srsp
	return res, nil
}
