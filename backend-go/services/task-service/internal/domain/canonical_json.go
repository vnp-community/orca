package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// MaxCanonicalJSONDepth bounds nesting so a hostile spec cannot exhaust the stack.
const MaxCanonicalJSONDepth = 32

// CanonicalJSON renders raw as the single byte sequence every service hashes:
// keys sorted by code point, no whitespace, NFC strings (keys included), integral numbers
// without a fraction, no HTML escaping. Duplicate keys are rejected because encoding/json would
// silently keep the last one and two digests of "the same" spec could then differ.
// request-service keeps its own copy; testdata/artifacts/task holds the shared golden cases.
func CanonicalJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out bytes.Buffer
	if err := writeCanonicalValue(dec, 0, &out); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("canonical json: trailing data after the top-level value")
	}
	return out.Bytes(), nil
}

func writeCanonicalValue(dec *json.Decoder, depth int, out *bytes.Buffer) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("canonical json: %w", err)
	}
	switch t := tok.(type) {
	case json.Delim:
		if depth+1 > MaxCanonicalJSONDepth {
			return fmt.Errorf("canonical json: nesting deeper than %d", MaxCanonicalJSONDepth)
		}
		if t == '{' {
			return writeCanonicalObject(dec, depth, out)
		}
		return writeCanonicalArray(dec, depth, out)
	case string:
		writeCanonicalString(out, t)
	case json.Number:
		out.WriteString(canonicalNumber(t))
	case bool:
		out.WriteString(strconv.FormatBool(t))
	case nil:
		out.WriteString("null")
	}
	return nil
}

func writeCanonicalObject(dec *json.Decoder, depth int, out *bytes.Buffer) error {
	members := map[string][]byte{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("canonical json: %w", err)
		}
		key := norm.NFC.String(keyTok.(string))
		if _, dup := members[key]; dup {
			return fmt.Errorf("canonical json: duplicate key %q", key)
		}
		var v bytes.Buffer
		if err := writeCanonicalValue(dec, depth+1, &v); err != nil {
			return err
		}
		members[key] = v.Bytes()
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return fmt.Errorf("canonical json: %w", err)
	}
	keys := make([]string, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	sort.Strings(keys) // byte order of UTF-8 equals code point order
	out.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		writeCanonicalString(out, k)
		out.WriteByte(':')
		out.Write(members[k])
	}
	out.WriteByte('}')
	return nil
}

func writeCanonicalArray(dec *json.Decoder, depth int, out *bytes.Buffer) error {
	out.WriteByte('[')
	first := true
	for dec.More() {
		if !first {
			out.WriteByte(',')
		}
		first = false
		if err := writeCanonicalValue(dec, depth+1, out); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil { // closing ']'
		return fmt.Errorf("canonical json: %w", err)
	}
	out.WriteByte(']')
	return nil
}

func writeCanonicalString(out *bytes.Buffer, s string) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(norm.NFC.String(s)) // a Go string always encodes
	out.WriteString(strings.TrimSuffix(b.String(), "\n"))
}

// canonicalNumber drops a redundant fraction or exponent ("1.0", "1e2" -> "1", "100") so the
// same value never hashes two ways; other numbers use the shortest round-trip form.
func canonicalNumber(n json.Number) string {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		if s == "-0" {
			return "0"
		}
		return s
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	if a := math.Abs(f); a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
