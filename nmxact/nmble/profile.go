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
	. "mynewt.apache.org/newtmgr/nmxact/bledefs"
)

type Descriptor struct {
	Uuid     BleUuid
	Handle   uint16
	AttFlags BleAttFlags
}

type Characteristic struct {
	Uuid       BleUuid
	DefHandle  uint16
	ValHandle  uint16
	Properties BleDiscChrProperties
	Dscs       []*Descriptor
}

type Service struct {
	Uuid        BleUuid
	StartHandle uint16
	EndHandle   uint16
	Chrs        []*Characteristic
}

type Profile struct {
	svcs  []Service
	chrs  map[BleChrId]*Characteristic
	attrs map[uint16]*Characteristic
}

func (c *Characteristic) String() string {
	return c.Uuid.String()
}

func (c *Characteristic) SubscribeType() BleDiscChrProperties {
	if c.Properties&BLE_GATT_F_NOTIFY != 0 {
		return BLE_GATT_F_NOTIFY
	} else {
		return c.Properties & BLE_DISC_CHR_PROP_INDICATE
	}
}

func NewProfile() Profile {
	return Profile{
		chrs:  map[BleChrId]*Characteristic{},
		attrs: map[uint16]*Characteristic{},
	}
}

func (p *Profile) Services() []Service {
	return p.svcs
}

func (p *Profile) SetServices(svcs []Service) {
	p.svcs = svcs
	p.chrs = map[BleChrId]*Characteristic{}
	p.attrs = map[uint16]*Characteristic{}

	for _, s := range svcs {
		for _, c := range s.Chrs {
			p.chrs[BleChrId{s.Uuid, c.Uuid}] = c
			p.attrs[uint16(c.ValHandle)] = c
		}
	}
}

func (p *Profile) FindChrByUuid(id BleChrId) *Characteristic {
	return p.chrs[id]
}

func (p *Profile) FindChrByHandle(handle uint16) *Characteristic {
	return p.attrs[handle]
}

func FindDscByUuid(chr *Characteristic, uuid BleUuid) *Descriptor {
	for _, d := range chr.Dscs {
		if CompareUuids(uuid, d.Uuid) == 0 {
			return d
		}
	}

	return nil
}
