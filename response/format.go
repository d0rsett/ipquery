// Package response presents JSON without changing its field order or values.
package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Config struct {
	DefaultFormat string `env:"IPQUERY_DEFAULT_FORMAT" envDefault:"auto"`
	ColorEnabled  bool   `env:"IPQUERY_COLOR_ENABLED" envDefault:"true"`
	AlignKeys     bool   `env:"IPQUERY_ALIGN_KEYS" envDefault:"true"`
	Indent        int    `env:"IPQUERY_JSON_INDENT" envDefault:"2"`
}

func DefaultConfig() Config { return Config{"auto", true, true, 2} }

func validFormat(s string) bool {
	switch s {
	case "json", "pretty", "ansi", "auto":
		return true
	}
	return false
}

func (c Config) Validate() error {
	if !validFormat(c.DefaultFormat) {
		return fmt.Errorf("invalid IPQUERY_DEFAULT_FORMAT %q", c.DefaultFormat)
	}
	if c.Indent < 0 || c.Indent > 16 {
		return fmt.Errorf("IPQUERY_JSON_INDENT must be between 0 and 16")
	}
	return nil
}

type member struct {
	key   string
	value node
}
type node struct {
	kind    byte
	scalar  string
	members []member
	items   []node
}

// read keeps object members in source order and numbers in their original precision.
func read(d *json.Decoder) (node, error) {
	t, err := d.Token()
	if err != nil {
		return node{}, err
	}
	switch v := t.(type) {
	case json.Delim:
		n := node{kind: byte(v)}
		for d.More() {
			if v == '{' {
				key, err := d.Token()
				if err != nil {
					return node{}, err
				}
				child, err := read(d)
				if err != nil {
					return node{}, err
				}
				b, _ := json.Marshal(key.(string))
				n.members = append(n.members, member{string(b), child})
			} else {
				child, err := read(d)
				if err != nil {
					return node{}, err
				}
				n.items = append(n.items, child)
			}
		}
		_, err := d.Token()
		return n, err
	case string:
		b, _ := json.Marshal(v)
		return node{kind: 's', scalar: string(b)}, nil
	case json.Number:
		return node{kind: 'n', scalar: string(v)}, nil
	case bool:
		return node{kind: 'b', scalar: fmt.Sprint(v)}, nil
	default:
		return node{kind: '0', scalar: "null"}, nil
	}
}

// width counts rendered columns, ignoring combining marks and allowing common
// East Asian wide characters. Escape sequences in JSON strings are literal text.
func width(s string) int {
	w := 0
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		w++
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe19 || r >= 0xfe30 && r <= 0xfe6f || r >= 0xff00 && r <= 0xff60 || r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f300 && r <= 0x1faff || r >= 0x20000 && r <= 0x3fffd) {
			w++
		}
	}
	return w
}

func paint(b *strings.Builder, s, code string, color bool) {
	if color {
		b.WriteString("\x1b[" + code + "m")
	}
	b.WriteString(s)
	if color {
		b.WriteString("\x1b[0m")
	}
}

func (n node) write(b *strings.Builder, c Config, color bool, depth int) {
	if n.kind != '{' && n.kind != '[' {
		code := map[byte]string{'s': "32", 'n': "35", 'b': "34", '0': "90"}[n.kind]
		paint(b, n.scalar, code, color)
		return
	}
	close := byte('}')
	count := len(n.members)
	if n.kind == '[' {
		close = ']'
		count = len(n.items)
	}
	b.WriteByte(n.kind)
	if count == 0 {
		b.WriteByte(close)
		return
	}
	b.WriteByte('\n')
	max := 0
	if c.AlignKeys {
		for _, m := range n.members {
			if w := width(m.key); w > max {
				max = w
			}
		}
	}
	for i := 0; i < count; i++ {
		b.WriteString(strings.Repeat(" ", (depth+1)*c.Indent))
		if n.kind == '{' {
			m := n.members[i]
			paint(b, m.key, "36", color)
			if c.AlignKeys {
				b.WriteString(strings.Repeat(" ", max-width(m.key)+1))
			}
			b.WriteString(": ")
			m.value.write(b, c, color, depth+1)
		} else {
			n.items[i].write(b, c, color, depth+1)
		}
		if i+1 < count {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(" ", depth*c.Indent))
	b.WriteByte(close)
}

// Format accepts exactly one JSON value and always appends a newline.
// Marshaling the API value first preserves encoding/json behavior and struct order.
func Format(data []byte, mode string, c Config) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if !utf8.Valid(data) || !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON")
	}
	if mode == "json" {
		var b bytes.Buffer
		if err := json.Compact(&b, data); err != nil {
			return nil, err
		}
		b.WriteByte('\n')
		return b.Bytes(), nil
	}
	if mode != "pretty" && mode != "ansi" {
		return nil, fmt.Errorf("unresolved format %q", mode)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	n, err := read(d)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	var b strings.Builder
	n.write(&b, c, mode == "ansi" && c.ColorEnabled, 0)
	b.WriteByte('\n')
	return []byte(b.String()), nil
}
