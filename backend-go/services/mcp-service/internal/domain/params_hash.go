package domain

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// ErrDuplicateKey is returned for an object with a repeated key: parsers
// disagree on which value wins, so `{"cmd":"ls","cmd":"rm"}` could show the
// approver one command and run another.
var ErrDuplicateKey = errors.New("domain: duplicate key in JSON object")

const maxJSONDepth = 32

type jsonObject struct {
	keys []string
	vals map[string]any
}

func parseJSONValue(dec *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, errors.New("domain: JSON nested too deeply")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := &jsonObject{vals: map[string]any{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, ok := kt.(string)
				if !ok {
					return nil, errors.New("domain: object key is not a string")
				}
				if _, dup := obj.vals[k]; dup {
					return nil, ErrDuplicateKey
				}
				v, err := parseJSONValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				obj.keys = append(obj.keys, k)
				obj.vals[k] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseJSONValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return arr, nil
		}
		return nil, errors.New("domain: unexpected delimiter")
	default:
		return tok, nil // string, json.Number, bool, nil
	}
}

func parseStrictJSON(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	v, err := parseJSONValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("domain: trailing data after JSON value")
	}
	return v, nil
}

func isInvisible(r rune) bool {
	switch {
	case r >= 0x200B && r <= 0x200F, r >= 0x202A && r <= 0x202E, r >= 0x2060 && r <= 0x2064,
		r >= 0x2066 && r <= 0x2069, r == 0xFEFF, r == 0x2028, r == 0x2029, r == 0x85, r == 0x7F:
		return true
	}
	return false
}

// writeCanonicalString escapes `"` `\` and every control character as
// \u00xx (lowercase hex), plus U+2028/2029; no HTML escaping. When
// escapeInvisible is set (preview) bidi/zero-width characters become visible \uXXXX.
func writeJSONString(b *strings.Builder, s string, escapeInvisible bool) {
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r < 0x20:
			fmt.Fprintf(b, `\u%04x`, r)
		case r == 0x2028 || r == 0x2029:
			fmt.Fprintf(b, `\u%04x`, r)
		case escapeInvisible && isInvisible(r):
			fmt.Fprintf(b, `\u%04X`, r)
		case r == utf8.RuneError:
			b.WriteString("\\" + "ufffd")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

func writeCanonical(b *strings.Builder, v any, indent string, level int, escapeInvisible bool) {
	nl := func(l int) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(indent, l))
		}
	}
	switch t := v.(type) {
	case *jsonObject:
		keys := append([]string(nil), t.keys...)
		sort.Strings(keys) // byte order of UTF-8
		if len(keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			writeJSONString(b, k, escapeInvisible)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			writeCanonical(b, t.vals[k], indent, level+1, escapeInvisible)
		}
		nl(level)
		b.WriteByte('}')
	case []any:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(level + 1)
			writeCanonical(b, e, indent, level+1, escapeInvisible)
		}
		nl(level)
		b.WriteByte(']')
	case string:
		writeJSONString(b, t, escapeInvisible)
	case json.Number:
		b.WriteString(t.String()) // literal preserved: 1 and 1.0 hash differently
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case nil:
		b.WriteString("null")
	}
}

// CanonicalArgs returns the canonical form of the client's arguments (`{}`
// when absent). It rejects duplicate keys and non-object arguments.
func canonicalArgsValue(args []byte) (any, error) {
	if len(bytes.TrimSpace(args)) == 0 {
		return &jsonObject{vals: map[string]any{}}, nil
	}
	v, err := parseStrictJSON(args)
	if err != nil {
		return nil, err
	}
	if _, ok := v.(*jsonObject); !ok && v != nil {
		return nil, errors.New("domain: tool arguments must be a JSON object")
	}
	if v == nil {
		return &jsonObject{vals: map[string]any{}}, nil
	}
	return v, nil
}

// ParamsHash is "sha256:" + hex of canonical({"v":1,"tenant","user","client","tool","args"}).
// tool is the channel. Hash covers the whole args, including parts the preview redacts.
func ParamsHash(tenantID, userID, clientID, channel string, args []byte) (string, error) {
	av, err := canonicalArgsValue(args)
	if err != nil {
		return "", err
	}
	env := &jsonObject{vals: map[string]any{
		"v": json.Number("1"), "tenant": tenantID, "user": userID, "client": clientID, "tool": channel, "args": av,
	}}
	for k := range env.vals {
		env.keys = append(env.keys, k)
	}
	var b strings.Builder
	writeCanonical(&b, env, "", 0, false)
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ParamsHashEqual compares hashes in constant time.
func ParamsHashEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// MaxArgsPreviewBytes: larger arguments are denied, never truncated, because
// the approver must see everything the hash covers.
const MaxArgsPreviewBytes = 8 * 1024

// ArgsPreview renders the arguments for the approver: pretty, key-sorted,
// secrets redacted, invisible/bidi characters made visible.
func ArgsPreview(args []byte, r Redactor) (text string, redacted bool, err error) {
	av, err := canonicalArgsValue(args)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	writeCanonical(&b, av, "  ", 0, true)
	text = b.String()
	if r != nil {
		text, redacted = r.Redact(text)
	}
	return text, redacted, nil
}

// Redactor masks secret material in free text.
type Redactor interface {
	Redact(s string) (string, bool)
}
