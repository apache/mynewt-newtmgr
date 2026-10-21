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

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runtimeco/go-coap"

	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

func TestExtractResKv(t *testing.T) {
	tests := []struct {
		name   string
		params []string
		want   map[string]interface{}
	}{
		{"none", nil, map[string]interface{}{}},
		{"int", []string{"a=1"}, map[string]interface{}{"a": 1}},
		{"negative int", []string{"a=-5"}, map[string]interface{}{"a": -5}},
		{"string", []string{"a=hello"},
			map[string]interface{}{"a": "hello"}},
		{"quoted int stays string", []string{`a="12"`},
			map[string]interface{}{"a": "12"}},
		{"quoted bool stays string", []string{`a="true"`},
			map[string]interface{}{"a": "true"}},
		{"empty quoted string", []string{`a=""`},
			map[string]interface{}{"a": ""}},
		{"only leading quote", []string{`a="x`},
			map[string]interface{}{"a": `"x`}},
		// A lone quote is not a quoted string; it used to panic.
		{"single quote", []string{`a="`},
			map[string]interface{}{"a": `"`}},
		{"bool true", []string{"a=true"}, map[string]interface{}{"a": true}},
		{"bool mixed case", []string{"a=FaLsE"},
			map[string]interface{}{"a": false}},
		{"float stays string", []string{"a=1.5"},
			map[string]interface{}{"a": "1.5"}},
		{"empty value", []string{"a="}, map[string]interface{}{"a": ""}},
		{"value with equals", []string{"a=b=c"},
			map[string]interface{}{"a": "b=c"}},
		{"empty key", []string{"=1"}, map[string]interface{}{"": 1}},
		{"multiple", []string{"x=1", "y=two", "z=false"},
			map[string]interface{}{"x": 1, "y": "two", "z": false}},
		{"duplicate key last wins", []string{"x=1", "x=2"},
			map[string]interface{}{"x": 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := extractResKv(tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(m, tt.want) {
				t.Errorf("got %#v, want %#v", m, tt.want)
			}
		})
	}
}

func TestExtractResKvErrors(t *testing.T) {
	for _, p := range []string{"", "abc"} {
		m, err := extractResKv([]string{"ok=1", p})
		if err == nil {
			t.Errorf("%q: expected error, got %v", p, m)
			continue
		}
		want := "invalid resource specifier: " + p
		if err.Error() != want {
			t.Errorf("error = %q, want %q", err.Error(), want)
		}
	}
}

// normCbor converts a decoded CBOR value into a form that is easy to compare:
// maps get string keys and integers become int64.
func normCbor(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, e := range x {
			m[k.(string)] = normCbor(e)
		}
		return m
	case []interface{}:
		for i := range x {
			x[i] = normCbor(x[i])
		}
		return x
	case uint64:
		return int64(x)
	default:
		return x
	}
}

func decodeCbor(t *testing.T, b []byte) interface{} {
	t.Helper()
	v, err := nmxutil.DecodeCbor(b)
	if err != nil {
		t.Fatalf("DecodeCbor(%x): %v", b, err)
	}
	return normCbor(v)
}

// setResGlobals sets the res command flag variables for the duration of a
// test.
func setResGlobals(t *testing.T, json, toInt bool, jsonFile, rawFile,
	binFile string) {

	t.Helper()

	oJson, oInt := resJson, resInt
	oJf, oRf, oBf := resJsonFilename, resRawFilename, resBinFilename
	t.Cleanup(func() {
		resJson, resInt = oJson, oInt
		resJsonFilename, resRawFilename, resBinFilename = oJf, oRf, oBf
	})

	resJson, resInt = json, toInt
	resJsonFilename, resRawFilename, resBinFilename = jsonFile, rawFile,
		binFile
}

func TestParsePayloadNoArgs(t *testing.T) {
	// Regression (d988f6d, 0d6802e): no payload arguments must produce no
	// payload at all, not an empty CBOR map.
	for _, json := range []bool{false, true} {
		setResGlobals(t, json, false, "", "", "")

		for _, args := range [][]string{nil, {}} {
			b, err := parsePayload(args)
			if err != nil {
				t.Fatalf("json=%v: unexpected error: %v", json, err)
			}
			if b != nil {
				t.Errorf("json=%v: payload = %x, want nil", json, b)
			}

			b, err = calcCborPayload(args)
			if err != nil {
				t.Fatalf("json=%v: unexpected error: %v", json, err)
			}
			if b != nil {
				t.Errorf("json=%v: payload = %x, want nil", json, b)
			}
		}
	}
}

func TestParsePayloadKv(t *testing.T) {
	setResGlobals(t, false, false, "", "", "")

	// Regression (80e5c28): k=v pairs must not be discarded.
	b, err := parsePayload([]string{"a=1", "b=str", "c=true", `d="7"`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]interface{}{
		"a": int64(1), "b": "str", "c": true, "d": "7",
	}
	if got := decodeCbor(t, b); !reflect.DeepEqual(got, want) {
		t.Errorf("payload = %#v, want %#v", got, want)
	}

	if _, err := parsePayload([]string{"novalue"}); err == nil {
		t.Errorf("expected error for invalid specifier")
	}
}

func TestParsePayloadJsonArg(t *testing.T) {
	tests := []struct {
		name  string
		toInt bool
		arg   string
		want  interface{}
	}{
		{"object float", false, `{"a": 2, "b": [1.5, "x"]}`,
			map[string]interface{}{
				"a": float64(2),
				"b": []interface{}{1.5, "x"},
			}},
		{"object int", true, `{"a": 2, "b": [1.5, 3.0, {"c": -4}]}`,
			map[string]interface{}{
				"a": int64(2),
				"b": []interface{}{1.5, int64(3),
					map[string]interface{}{"c": int64(-4)}},
			}},
		{"scalar", true, `5`, int64(5)},
		{"string", false, `"hi"`, "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setResGlobals(t, true, tt.toInt, "", "", "")

			b, err := parsePayload([]string{tt.arg, "ignored"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := decodeCbor(t, b); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("payload = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParsePayloadJsonNull(t *testing.T) {
	setResGlobals(t, true, false, "", "", "")

	// A JSON null is treated as "no payload".
	b, err := parsePayload([]string{"null"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != nil {
		t.Errorf("payload = %x, want nil", b)
	}
}

func TestParsePayloadJsonInvalid(t *testing.T) {
	setResGlobals(t, true, false, "", "", "")

	if _, err := parsePayload([]string{"{bad"}); err == nil {
		t.Errorf("expected error for invalid JSON")
	}
	if _, err := parsePayloadJson(""); err == nil {
		t.Errorf("expected error for empty JSON")
	}
}

func TestRemoveFloats(t *testing.T) {
	tests := []struct {
		in   interface{}
		want interface{}
	}{
		{float64(2), int64(2)},
		{float64(-3), int64(-3)},
		{float64(0), int64(0)},
		{1.5, 1.5},
		{float32(4), int64(4)},
		{float32(0.5), float64(0.5)},
		{"s", "s"},
		{true, true},
		{nil, nil},
		{[]interface{}{1.0, 2.5, "x"}, []interface{}{int64(1), 2.5, "x"}},
		{map[string]interface{}{"a": 1.0, "b": []interface{}{7.0}},
			map[string]interface{}{"a": int64(1),
				"b": []interface{}{int64(7)}}},
	}

	for _, tt := range tests {
		got := removeFloats(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("removeFloats(%#v) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCalcCborPayloadRawFile(t *testing.T) {
	raw := []byte{0xa1, 0x61, 0x61, 0x01, 0xff, 0x00}
	setResGlobals(t, false, false, "", writeTemp(t, "raw", raw), "")

	// The raw file is sent verbatim and arguments are ignored.
	b, err := calcCborPayload([]string{"a=2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(b, raw) {
		t.Errorf("payload = %x, want %x", b, raw)
	}
}

func TestCalcCborPayloadJsonFile(t *testing.T) {
	p := writeTemp(t, "j.json", []byte(`{"a": 3.0, "b": "x"}`))
	setResGlobals(t, false, true, p, "", "")

	b, err := calcCborPayload(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]interface{}{"a": int64(3), "b": "x"}
	if got := decodeCbor(t, b); !reflect.DeepEqual(got, want) {
		t.Errorf("payload = %#v, want %#v", got, want)
	}

	bad := writeTemp(t, "bad.json", []byte(`{`))
	setResGlobals(t, false, false, bad, "", "")
	if _, err := calcCborPayload(nil); err == nil {
		t.Errorf("expected error for invalid JSON file")
	}
}

func TestCalcCborPayloadBinFile(t *testing.T) {
	data := []byte{0, 1, 2, 0xfe}
	setResGlobals(t, false, false, "", "", writeTemp(t, "bin", data))

	b, err := calcCborPayload(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Encoded as a CBOR byte string.
	want := append([]byte{0x44}, data...)
	if !reflect.DeepEqual(b, want) {
		t.Errorf("payload = %x, want %x", b, want)
	}
}

func TestCalcCborPayloadMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	cases := []struct {
		name              string
		jsonF, rawF, binF string
	}{
		{"json", missing, "", ""},
		{"raw", "", missing, ""},
		{"bin", "", "", missing},
	}
	for _, c := range cases {
		setResGlobals(t, false, false, c.jsonF, c.rawF, c.binF)
		if _, err := calcCborPayload(nil); err == nil {
			t.Errorf("%s: expected error for missing file", c.name)
		}
	}
}

func TestCborValStr(t *testing.T) {
	tests := []struct {
		in   interface{}
		want string
	}{
		{"abc", "abc"},
		{uint64(5), "0x5"},
		{int64(-5), "-5"},
		{true, "true"},
		{[]byte{0x01, 0x02},
			"00000000  01 02                                             |..|"},
	}

	for _, tt := range tests {
		if got := cborValStr(tt.in); got != tt.want {
			t.Errorf("cborValStr(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrintCode(t *testing.T) {
	tests := []struct {
		code coap.COAPCode
		want string
	}{
		{coap.Content, "CoAP Response Code: 2.05 Content\n"},
		{coap.NotFound, "CoAP Response Code: 4.04 NotFound\n"},
		{coap.Created, "CoAP Response Code: 2.01 Created\n"},
	}

	for _, tt := range tests {
		if got := printCode(tt.code); got != tt.want {
			t.Errorf("printCode(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCleanUpMapValue(t *testing.T) {
	in := map[interface{}]interface{}{
		"s":       "str",
		uint64(1): []interface{}{int64(-1), true, nil},
		"m":       map[interface{}]interface{}{"x": 1.5},
	}
	want := map[string]interface{}{
		"s": "str",
		"1": []interface{}{"-1", "true", "<nil>"},
		"m": map[string]interface{}{"x": "1.5"},
	}

	if got := cleanUpMapValue(in); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestResResponseStr(t *testing.T) {
	if got := resResponseStr("/a/b", nil); got != "/a/b\n    <empty>" {
		t.Errorf("empty payload: got %q", got)
	}

	b, err := nmxutil.EncodeCbor(map[string]interface{}{"k": 1})
	if err != nil {
		t.Fatal(err)
	}
	want := "/a/b\n{\n    \"k\": \"1\"\n}"
	if got := resResponseStr("/a/b", b); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got := resResponseStr("/a/b", []byte{0xff, 0xff})
	if !strings.HasPrefix(got, "/a/b\n    invalid incoming cbor:") {
		t.Errorf("invalid cbor: got %q", got)
	}
}
