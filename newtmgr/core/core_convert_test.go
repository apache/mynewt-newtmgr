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

package core

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type testTlv struct {
	typ  uint8
	off  uint32
	data []byte
}

// buildCore builds a corefile image consisting of a header followed by the
// given TLVs.
func buildCore(magic uint32, tlvs []testTlv) []byte {
	var b bytes.Buffer

	binary.Write(&b, binary.LittleEndian, magic)
	binary.Write(&b, binary.LittleEndian, uint32(0))
	for _, t := range tlvs {
		b.WriteByte(t.typ)
		b.WriteByte(0)
		binary.Write(&b, binary.LittleEndian, uint16(len(t.data)))
		binary.Write(&b, binary.LittleEndian, t.off)
		b.Write(t.data)
	}

	return b.Bytes()
}

// convert writes the corefile to a temporary directory, converts it and
// returns the converter and the path of the resulting ELF file.
func convert(t *testing.T, core []byte) (*CoreConvert, string, error) {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "core")
	dst := filepath.Join(dir, "core.elf")
	if err := os.WriteFile(src, core, 0644); err != nil {
		t.Fatal(err)
	}

	cc, err := ConvertFilenames(src, dst)
	return cc, dst, err
}

func seq(n int, start byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = start + byte(i)
	}
	return b
}

func readProg(t *testing.T, p *elf.Prog) []byte {
	t.Helper()
	b, err := io.ReadAll(p.Open())
	if err != nil {
		t.Fatalf("reading segment: %v", err)
	}
	return b
}

func TestConvert(t *testing.T) {
	hash := seq(32, 0xa0)
	mem1 := seq(16, 0x10)
	mem2 := seq(5, 0x50) // odd length; padded in the file
	regs := make([]byte, 4*20)
	for i := 0; i < 20; i++ {
		binary.LittleEndian.PutUint32(regs[i*4:], 0x1000+uint32(i))
	}

	core := buildCore(COREDUMP_MAGIC, []testTlv{
		{COREDUMP_TLV_IMAGE, 0, hash},
		{COREDUMP_TLV_REGS, 0, regs},
		{COREDUMP_TLV_MEM, 0x20000000, mem1},
		{COREDUMP_TLV_MEM, 0x20001000, mem2},
	})

	cc, dst, err := convert(t, core)
	if err != nil {
		t.Fatalf("ConvertFilenames: %v", err)
	}
	if !bytes.Equal(cc.ImageHash, hash) {
		t.Errorf("ImageHash = %x, want %x", cc.ImageHash, hash)
	}

	f, err := elf.Open(dst)
	if err != nil {
		t.Fatalf("elf.Open: %v", err)
	}
	defer f.Close()

	if f.Class != elf.ELFCLASS32 || f.Data != elf.ELFDATA2LSB {
		t.Errorf("class/data = %v/%v", f.Class, f.Data)
	}
	if f.Type != elf.ET_CORE {
		t.Errorf("Type = %v, want ET_CORE", f.Type)
	}
	if f.Machine != elf.EM_ARM {
		t.Errorf("Machine = %v, want EM_ARM", f.Machine)
	}
	if len(f.Sections) != 0 {
		t.Errorf("expected no sections, got %d", len(f.Sections))
	}
	if len(f.Progs) != 3 {
		t.Fatalf("expected 3 program headers, got %d", len(f.Progs))
	}

	// Program headers appear in TLV order.
	note := f.Progs[0]
	if note.Type != elf.PT_NOTE {
		t.Errorf("phdr 0 type = %v, want PT_NOTE", note.Type)
	}
	nb := readProg(t, note)
	if len(nb) != 12+8+148 {
		t.Fatalf("note size = %d, want %d", len(nb), 12+8+148)
	}
	if n := binary.LittleEndian.Uint32(nb[0:]); n != 5 {
		t.Errorf("note namesz = %d, want 5", n)
	}
	if n := binary.LittleEndian.Uint32(nb[4:]); n != 148 {
		t.Errorf("note descsz = %d, want 148", n)
	}
	if n := binary.LittleEndian.Uint32(nb[8:]); n != uint32(elf.NT_PRSTATUS) {
		t.Errorf("note type = %d, want NT_PRSTATUS", n)
	}
	if !bytes.Equal(nb[12:20], []byte(".reg\x00\x00\x00\x00")) {
		t.Errorf("note name = %q", nb[12:20])
	}
	// prstatus: 18 words of padding, then 18 registers.  Registers beyond
	// the 18th are dropped.
	desc := nb[20:]
	if !bytes.Equal(desc[:72], make([]byte, 72)) {
		t.Errorf("prstatus padding not zero: %x", desc[:72])
	}
	for i := 0; i < 18; i++ {
		r := binary.LittleEndian.Uint32(desc[72+i*4:])
		if r != 0x1000+uint32(i) {
			t.Errorf("reg %d = %#x, want %#x", i, r, 0x1000+i)
		}
	}
	if r := binary.LittleEndian.Uint32(desc[144:]); r != 0 {
		t.Errorf("trailing word = %#x, want 0", r)
	}

	wantLoads := []struct {
		vaddr uint32
		data  []byte
	}{
		{0x20000000, mem1},
		{0x20001000, mem2},
	}
	for i, w := range wantLoads {
		p := f.Progs[i+1]
		if p.Type != elf.PT_LOAD {
			t.Errorf("phdr %d type = %v, want PT_LOAD", i+1, p.Type)
		}
		if p.Vaddr != uint64(w.vaddr) {
			t.Errorf("phdr %d vaddr = %#x, want %#x", i+1, p.Vaddr, w.vaddr)
		}
		if p.Filesz != uint64(len(w.data)) || p.Memsz != p.Filesz {
			t.Errorf("phdr %d filesz/memsz = %d/%d, want %d", i+1,
				p.Filesz, p.Memsz, len(w.data))
		}
		if p.Flags != elf.PF_R || p.Align != 4 {
			t.Errorf("phdr %d flags/align = %v/%d", i+1, p.Flags, p.Align)
		}
		if p.Off%4 != 0 {
			t.Errorf("phdr %d offset %#x not 4-byte aligned", i+1, p.Off)
		}
		if got := readProg(t, p); !bytes.Equal(got, w.data) {
			t.Errorf("phdr %d data = %x, want %x", i+1, got, w.data)
		}
	}

	// Segment data is laid out back to back after the program headers,
	// with the odd-length segment padded to a multiple of 4.
	const ehdrSz, phdrSz = 52, 32
	off := uint64(ehdrSz + 3*phdrSz)
	for i, p := range f.Progs {
		if p.Off != off {
			t.Errorf("phdr %d offset = %#x, want %#x", i, p.Off, off)
		}
		off += (p.Filesz + 3) &^ 3
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if uint64(st.Size()) != off {
		t.Errorf("file size = %d, want %d", st.Size(), off)
	}
}

func TestConvertShortRegs(t *testing.T) {
	regs := make([]byte, 8)
	binary.LittleEndian.PutUint32(regs[0:], 0xdeadbeef)
	binary.LittleEndian.PutUint32(regs[4:], 0xcafef00d)

	_, dst, err := convert(t, buildCore(COREDUMP_MAGIC, []testTlv{
		{COREDUMP_TLV_REGS, 0, regs},
	}))
	if err != nil {
		t.Fatalf("ConvertFilenames: %v", err)
	}

	f, err := elf.Open(dst)
	if err != nil {
		t.Fatalf("elf.Open: %v", err)
	}
	defer f.Close()

	if len(f.Progs) != 1 || f.Progs[0].Type != elf.PT_NOTE {
		t.Fatalf("unexpected program headers: %v", f.Progs)
	}
	desc := readProg(t, f.Progs[0])[20:]
	want := []uint32{0xdeadbeef, 0xcafef00d, 0}
	for i, w := range want {
		if r := binary.LittleEndian.Uint32(desc[72+i*4:]); r != w {
			t.Errorf("reg %d = %#x, want %#x", i, r, w)
		}
	}
}

func TestConvertErrors(t *testing.T) {
	valid := buildCore(COREDUMP_MAGIC, []testTlv{
		{COREDUMP_TLV_MEM, 0x100, seq(8, 0)},
	})

	tests := []struct {
		name string
		core []byte
		err  string
	}{
		{"empty file", nil, "Error reading: EOF"},
		{"short header", []byte{0xc3, 0x47, 0x0c, 0x69}, "Short read"},
		{"bad magic", buildCore(0x12345678, nil),
			"Source file is not corefile"},
		{"short tlv header", append(buildCore(COREDUMP_MAGIC, nil),
			COREDUMP_TLV_MEM, 0, 4), "Short read"},
		{"unknown tlv", buildCore(COREDUMP_MAGIC, []testTlv{
			{9, 0, seq(4, 0)},
		}), "Unknown TLV type"},
		{"tlv data truncated", valid[:len(valid)-3], "Short file"},
		{"tlv data missing", valid[:len(valid)-8], "Error reading: EOF"},
		{"regs not multiple of 4", buildCore(COREDUMP_MAGIC, []testTlv{
			{COREDUMP_TLV_REGS, 0, seq(6, 0)},
		}), "Invalid register area size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := convert(t, tt.core)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err.Error() != tt.err {
				t.Errorf("error = %q, want %q", err.Error(), tt.err)
			}
		})
	}
}

func TestConvertMissingFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := ConvertFilenames(filepath.Join(dir, "nope"),
		filepath.Join(dir, "out"))
	if err == nil {
		t.Errorf("expected error for missing source")
	}

	src := filepath.Join(dir, "core")
	if err := os.WriteFile(src, buildCore(COREDUMP_MAGIC, nil), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = ConvertFilenames(src, filepath.Join(dir, "nodir", "out"))
	if err == nil {
		t.Errorf("expected error for unwritable target")
	}

	if err := NewCoreConvert().Convert(); err == nil ||
		err.Error() != "Missing file parameters" {
		t.Errorf("Convert() without files: err = %v", err)
	}
}
