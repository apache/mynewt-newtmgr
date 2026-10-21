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

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/go-homedir"

	"mynewt.apache.org/newtmgr/newtmgr/nmutil"
)

func TestConnTypeStrings(t *testing.T) {
	tests := []struct {
		ct ConnType
		s  string
	}{
		{CONN_TYPE_SERIAL_PLAIN, "serial"},
		{CONN_TYPE_SERIAL_OIC, "oic_serial"},
		{CONN_TYPE_BLL_PLAIN, "ble"},
		{CONN_TYPE_BLL_OIC, "oic_ble"},
		{CONN_TYPE_BLE_PLAIN, "bhd"},
		{CONN_TYPE_BLE_OIC, "oic_bhd"},
		{CONN_TYPE_UDP_PLAIN, "udp"},
		{CONN_TYPE_UDP_OIC, "oic_udp"},
		{CONN_TYPE_MTECH_LORA_OIC, "oic_mtech"},
		{CONN_TYPE_NONE, "???"},
	}

	for _, tt := range tests {
		if got := ConnTypeToString(tt.ct); got != tt.s {
			t.Errorf("ConnTypeToString(%d) = %q, want %q", tt.ct, got, tt.s)
		}
		got, err := ConnTypeFromString(tt.s)
		if err != nil {
			t.Errorf("ConnTypeFromString(%q): %v", tt.s, err)
		} else if got != tt.ct {
			t.Errorf("ConnTypeFromString(%q) = %d, want %d", tt.s, got, tt.ct)
		}
	}

	if len(connTypeNameMap) != len(tests) {
		t.Errorf("connTypeNameMap has %d entries, test covers %d",
			len(connTypeNameMap), len(tests))
	}

	if s := ConnTypeToString(ConnType(100)); s != "" {
		t.Errorf("ConnTypeToString(100) = %q, want \"\"", s)
	}
}

func TestConnTypeFromStringInvalid(t *testing.T) {
	for _, s := range []string{"", "SERIAL", "foo"} {
		ct, err := ConnTypeFromString(s)
		if err == nil {
			t.Errorf("ConnTypeFromString(%q): expected error", s)
			continue
		}
		if ct != CONN_TYPE_NONE {
			t.Errorf("ConnTypeFromString(%q) = %d, want NONE", s, ct)
		}
		want := "Invalid connection type: " + s
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	}
}

func TestConnTypeJSON(t *testing.T) {
	ct := CONN_TYPE_SERIAL_OIC
	b, err := json.Marshal(&ct)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != `"oic_serial"` {
		t.Errorf("Marshal = %s, want \"oic_serial\"", b)
	}

	var back ConnType
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != ct {
		t.Errorf("Unmarshal = %d, want %d", back, ct)
	}

	// Non-string JSON is rejected.
	if err := json.Unmarshal([]byte("3"), &back); err == nil {
		t.Errorf("Unmarshal(3): expected error")
	}
}

// An unknown connection type name in the profile file is not an error; it
// is silently mapped to CONN_TYPE_NONE.  This documents current behaviour.
func TestConnTypeUnmarshalUnknown(t *testing.T) {
	ct := CONN_TYPE_UDP_PLAIN
	if err := json.Unmarshal([]byte(`"carrier_pigeon"`), &ct); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ct != CONN_TYPE_NONE {
		t.Errorf("Unmarshal = %d, want CONN_TYPE_NONE", ct)
	}
}

func TestConnProfileString(t *testing.T) {
	p := &ConnProfile{
		Name:       "p1",
		Type:       CONN_TYPE_SERIAL_PLAIN,
		ConnString: "dev=/dev/ttyUSB0",
	}
	want := "name=p1 type=serial connstring=dev=/dev/ttyUSB0"
	if p.String() != want {
		t.Errorf("String() = %q, want %q", p.String(), want)
	}
}

// fakeHome points the home directory at a fresh temporary directory and
// returns the path of the connection profile file within it.
func fakeHome(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	homedir.Reset()
	t.Cleanup(homedir.Reset)

	old := nmutil.ToolInfo
	nmutil.ToolInfo.CfgFilename = ".newtmgr.cp.json"
	t.Cleanup(func() { nmutil.ToolInfo = old })

	return filepath.Join(dir, ".newtmgr.cp.json")
}

func TestConnProfileMgrMissingFile(t *testing.T) {
	path := fakeHome(t)

	cpm, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("NewConnProfileMgr: %v", err)
	}
	list, err := cpm.GetConnProfileList()
	if err != nil {
		t.Fatalf("GetConnProfileList: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected no profiles, got %d", len(list))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("profile file should not be created on load: %v", err)
	}
}

func TestConnProfileMgrRoundTrip(t *testing.T) {
	fakeHome(t)

	cpm, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("NewConnProfileMgr: %v", err)
	}

	profs := []*ConnProfile{
		{Name: "zeta", Type: CONN_TYPE_UDP_PLAIN, ConnString: "127.0.0.1:5683"},
		{Name: "alpha", Type: CONN_TYPE_SERIAL_PLAIN,
			ConnString: "dev=/dev/ttyUSB0,baud=115200"},
		{Name: "mid", Type: CONN_TYPE_BLL_OIC, ConnString: "peer_name=a=b"},
	}
	for _, p := range profs {
		if err := cpm.AddConnProfile(p); err != nil {
			t.Fatalf("AddConnProfile(%s): %v", p.Name, err)
		}
	}

	cpm2, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	list, _ := cpm2.GetConnProfileList()
	names := []string{}
	for _, p := range list {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "alpha,mid,zeta" {
		t.Errorf("profile list = %v, want sorted alpha,mid,zeta", names)
	}

	for _, want := range profs {
		got, err := cpm2.GetConnProfile(want.Name)
		if err != nil {
			t.Fatalf("GetConnProfile(%s): %v", want.Name, err)
		}
		if *got != *want {
			t.Errorf("profile %s = %+v, want %+v", want.Name, *got, *want)
		}
	}

	// Replace an existing profile.
	repl := &ConnProfile{Name: "mid", Type: CONN_TYPE_SERIAL_OIC,
		ConnString: "COM1"}
	if err := cpm2.AddConnProfile(repl); err != nil {
		t.Fatalf("AddConnProfile(replace): %v", err)
	}

	// Delete a profile and reload.
	if err := cpm2.DeleteConnProfile("zeta"); err != nil {
		t.Fatalf("DeleteConnProfile: %v", err)
	}

	cpm3, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	list, _ = cpm3.GetConnProfileList()
	if len(list) != 2 || list[0].Name != "alpha" || list[1].Name != "mid" {
		t.Fatalf("unexpected profiles after delete: %v", list)
	}
	if *list[1] != *repl {
		t.Errorf("replaced profile = %+v, want %+v", *list[1], *repl)
	}
	if _, err := cpm3.GetConnProfile("zeta"); err == nil {
		t.Errorf("deleted profile still present")
	}
}

func TestConnProfileMgrMissingProfile(t *testing.T) {
	fakeHome(t)

	cpm, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("NewConnProfileMgr: %v", err)
	}

	_, err = cpm.GetConnProfile("nope")
	if err == nil || err.Error() != `connection profile "nope" doesn't exist` {
		t.Errorf("GetConnProfile error = %v", err)
	}

	err = cpm.DeleteConnProfile("nope")
	if err == nil || err.Error() != `connection profile "nope" doesn't exist` {
		t.Errorf("DeleteConnProfile error = %v", err)
	}
}

func TestConnProfileMgrCorruptFile(t *testing.T) {
	path := fakeHome(t)

	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	cpm, err := NewConnProfileMgr()
	if err == nil {
		t.Fatalf("expected error, got %+v", cpm)
	}
	prefix := "error reading connection profile config (" + path + "): "
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

func TestConnProfileMgrUnreadableFile(t *testing.T) {
	path := fakeHome(t)

	// A directory in place of the file cannot be read.
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := NewConnProfileMgr(); err == nil {
		t.Fatalf("expected error")
	}
}

const goldenProfiles = `[
    {
        "MyName": "a",
        "MyType": "oic_serial",
        "MyConnString": "dev=/dev/ttyACM0"
    },
    {
        "MyName": "b",
        "MyType": "oic_mtech",
        "MyConnString": ""
    }
]`

func TestConnProfileMgrFileFormat(t *testing.T) {
	path := fakeHome(t)

	cpm, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("NewConnProfileMgr: %v", err)
	}
	if err := cpm.AddConnProfile(&ConnProfile{Name: "b",
		Type: CONN_TYPE_MTECH_LORA_OIC}); err != nil {
		t.Fatalf("AddConnProfile: %v", err)
	}
	if err := cpm.AddConnProfile(&ConnProfile{Name: "a",
		Type: CONN_TYPE_SERIAL_OIC, ConnString: "dev=/dev/ttyACM0"}); err != nil {
		t.Fatalf("AddConnProfile: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != goldenProfiles {
		t.Errorf("file contents:\n%s\nwant:\n%s", b, goldenProfiles)
	}

	// The golden file must also load back.
	if err := os.WriteFile(path, []byte(goldenProfiles), 0644); err != nil {
		t.Fatal(err)
	}
	cpm2, err := NewConnProfileMgr()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	p, err := cpm2.GetConnProfile("b")
	if err != nil || p.Type != CONN_TYPE_MTECH_LORA_OIC {
		t.Errorf("GetConnProfile(b) = %+v, %v", p, err)
	}
}

func TestSortConnProfs(t *testing.T) {
	in := []*ConnProfile{{Name: "c"}, {Name: "a"}, {Name: "b"}}
	out := SortConnProfs(in)
	if out[0].Name != "a" || out[1].Name != "b" || out[2].Name != "c" {
		t.Errorf("unexpected order: %v", out)
	}
	if in[0].Name != "c" {
		t.Errorf("input slice was modified")
	}
}
