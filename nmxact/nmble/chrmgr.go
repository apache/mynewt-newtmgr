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

package nmble

import (
	"fmt"

	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type chrMgrElem struct {
	AttHandle uint16
	SvcUuid   BleUuid
	ChrUuid   BleUuid
	Cb        BleGattAccessFn
}

type ChrMgr struct {
	chrs map[uint16]chrMgrElem
}

func (cm *ChrMgr) Clear() {
	cm.chrs = map[uint16]chrMgrElem{}
}

func (cm *ChrMgr) add(chr chrMgrElem) error {
	if _, ok := cm.chrs[chr.AttHandle]; ok {
		return fmt.Errorf("Characteristic with duplicate ATT handle: %d",
			chr.AttHandle)
	}

	cm.chrs[chr.AttHandle] = chr
	return nil
}

func (cm *ChrMgr) SetServices(x *BleXport, svcs []BleSvc) error {
	if err := ClearSvcsXact(x); err != nil {
		return err
	}
	cm.Clear()

	addSvcs := make([]BleAddSvc, len(svcs))
	for i, svc := range svcs {
		addSvcs[i] = BleSvcToAddSvc(svc)
	}

	if err := AddSvcsXact(x, addSvcs); err != nil {
		return err
	}

	regSvcs, err := CommitSvcsXact(x)
	if err != nil {
		return err
	}

	//               [uuid => svc]
	//              /             \
	// [uuid => chr]               [uuid => chr]
	svcMap := map[BleUuid]map[BleUuid]BleChr{}

	for _, svc := range svcs {
		m := map[BleUuid]BleChr{}
		svcMap[svc.Uuid] = m

		for _, chr := range svc.Chrs {
			m[chr.Uuid] = chr
		}
	}

	for _, rs := range regSvcs {
		srcSvc, ok := svcMap[rs.Uuid]
		if !ok {
			// XXX: Log
			continue
		}

		for _, rc := range rs.Chrs {
			srcChr, ok := srcSvc[rc.Uuid]
			if !ok {
				// XXX: Log
				continue
			}

			cm.add(chrMgrElem{
				AttHandle: rc.ValHandle,
				SvcUuid:   rs.Uuid,
				ChrUuid:   rc.Uuid,
				Cb:        srcChr.AccessCb,
			})
		}
	}

	return nil
}

func (cm *ChrMgr) findByAttHandle(handle uint16) *chrMgrElem {
	chr, ok := cm.chrs[handle]
	if !ok {
		return nil
	} else {
		return &chr
	}
}

func (cm *ChrMgr) Access(x *BleXport, evt *BleAccessEvt) error {
	chr := cm.findByAttHandle(evt.AttHandle)
	if chr == nil {
		return AccessStatusXact(x, uint8(ERR_CODE_ATT_INVALID_HANDLE), nil)
	}

	if chr.Cb == nil {
		return AccessStatusXact(x, 0, nil)
	}

	access := BleGattAccess{
		Op:         evt.GattOp,
		ConnHandle: evt.ConnHandle,
		SvcUuid:    chr.SvcUuid,
		ChrUuid:    chr.ChrUuid,
		Data:       evt.Data.Bytes,
	}

	status, val := chr.Cb(access)
	return AccessStatusXact(x, status, val)
}
