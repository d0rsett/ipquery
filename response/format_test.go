package response

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestAlignedOrder(t *testing.T) {
	input := []byte(`{"status":"success","countryCode":"NL","city":"Naaldwijk"}`)
	want := "{\n  \"status\"      : \"success\",\n  \"countryCode\" : \"NL\",\n  \"city\"        : \"Naaldwijk\"\n}\n"
	got, err := Format(input, "pretty", DefaultConfig())
	if err != nil || string(got) != want {
		t.Fatalf("got %s, err %v", got, err)
	}
}

func TestValuesAndColors(t *testing.T) {
	inputs := []string{
		`{}`, `[]`, `null`, `true`, `false`, `-12.375`, `1.2e-18`, `9007199254740993`,
		`{"outer":{"x":-12.5,"long":[true,false,null,"Привет 世界",{},[],[1,2]]},"empty":{}}`,
		`{"цитата":"\"\\\n\t\u001b[31m <>&","é":"café","é":"combining","世界":"東京"}`,
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			var expected any
			d := json.NewDecoder(strings.NewReader(input))
			d.UseNumber()
			if err := d.Decode(&expected); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"json", "pretty", "ansi"} {
				out, err := Format([]byte(input), mode, DefaultConfig())
				if err != nil {
					t.Fatal(err)
				}
				plain := ansi.ReplaceAll(out, nil)
				if mode != "ansi" && strings.Contains(string(out), "\x1b") {
					t.Fatal("unexpected ANSI")
				}
				if !strings.HasSuffix(string(out), "\n") || !json.Valid(plain) {
					t.Fatalf("invalid output %q", out)
				}
				var actual any
				d := json.NewDecoder(strings.NewReader(string(plain)))
				d.UseNumber()
				_ = d.Decode(&actual)
				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf("values changed: %s", out)
				}
			}
		})
	}
	input := []byte(`{"s":"text","n":-1.5,"t":true,"f":false,"z":null}`)
	colored, _ := Format(input, "ansi", DefaultConfig())
	plain, _ := Format(input, "pretty", DefaultConfig())
	if string(ansi.ReplaceAll(colored, nil)) != string(plain) {
		t.Fatal("ANSI changed alignment")
	}
	for _, token := range []string{"\x1b[36m\"s\"\x1b[0m", "\x1b[32m\"text\"\x1b[0m", "\x1b[35m-1.5\x1b[0m", "\x1b[34mtrue\x1b[0m", "\x1b[34mfalse\x1b[0m", "\x1b[90mnull\x1b[0m"} {
		if !strings.Contains(string(colored), token) {
			t.Fatalf("missing colored token %q", token)
		}
	}
}

func TestOptionsAndInvalidInput(t *testing.T) {
	c := DefaultConfig()
	c.AlignKeys = false
	c.Indent = 4
	c.ColorEnabled = false
	out, err := Format([]byte(`{"a":{},"long":[]}`), "ansi", c)
	if err != nil || string(out) != "{\n    \"a\": {},\n    \"long\": []\n}\n" {
		t.Fatalf("%q %v", out, err)
	}
	for _, bad := range []string{"", "{} {}", "{", "[1,]", "NaN"} {
		if _, err := Format([]byte(bad), "pretty", c); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, mode := range []string{"auto", "unknown"} {
		if _, err := Format([]byte(`{}`), mode, c); err == nil {
			t.Fatal("unresolved mode accepted")
		}
	}
	c.Indent = -1
	if c.Validate() == nil {
		t.Fatal("negative indent accepted")
	}
	c.Indent = 17
	if c.Validate() == nil {
		t.Fatal("unbounded indent accepted")
	}
	c = DefaultConfig()
	c.DefaultFormat = "invalid"
	if c.Validate() == nil {
		t.Fatal("invalid format accepted")
	}
}

func TestUnicodeColumns(t *testing.T) {
	out, _ := Format([]byte(`{"я":1,"世界":2,"é":3,"long":4,"a\"b":5}`), "pretty", DefaultConfig())
	want := "{\n  \"я\"    : 1,\n  \"世界\" : 2,\n  \"é\"    : 3,\n  \"long\" : 4,\n  \"a\\\"b\" : 5\n}\n"
	if string(out) != want {
		t.Fatalf("unexpected Unicode alignment: %s", out)
	}
}
