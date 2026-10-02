package tools

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const defaultMaxResultBytes = 64 * 1024

// normalizeResult turns a channel result into a JSON object: protojson for
// proto messages (camelCase), encoding/json otherwise with snake_case keys
// camelized; arrays are wrapped as {"items":[...]} because structuredContent
// must be an object. Secrets are redacted before truncation.
func normalizeResult(res any, spec *ToolSpec) (map[string]any, error) {
	var raw []byte
	var err error
	if m, ok := res.(proto.Message); ok && m != nil {
		raw, err = protojson.MarshalOptions{UseProtoNames: false}.Marshal(m)
	} else {
		raw, err = json.Marshal(res)
	}
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if spec == nil || !spec.KeepKeys {
		v = camelizeKeys(v)
	}
	if spec != nil && spec.Post != nil {
		v = spec.Post(v)
	}
	v = redactValue(v)
	var obj map[string]any
	switch t := v.(type) {
	case map[string]any:
		obj = t
	case []any:
		obj = map[string]any{"items": t}
	case nil:
		obj = map[string]any{}
	default:
		obj = map[string]any{"value": t}
	}
	if spec != nil && spec.Untrusted {
		obj["untrusted"] = true
	}
	limit := defaultMaxResultBytes
	if spec != nil && spec.MaxResultBytes > 0 {
		limit = spec.MaxResultBytes
	}
	return truncateObject(obj, limit), nil
}

func jsonSize(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 1 << 30
	}
	return len(b)
}

const truncationHint = "result truncated; narrow the request (limit, path, filters)"

// truncateObject shrinks obj until its JSON fits limit, always yielding valid
// JSON: arrays lose trailing elements, long strings are cut on a rune
// boundary, and as a last resort the payload is replaced by a marker.
func truncateObject(obj map[string]any, limit int) map[string]any {
	if jsonSize(obj) <= limit {
		return obj
	}
	obj["truncated"] = true
	obj["hint"] = truncationHint
	for _, cap := range []int{8192, 2048, 512, 128} {
		shrink(obj, limit, cap)
		if jsonSize(obj) <= limit {
			return obj
		}
	}
	return map[string]any{"truncated": true, "hint": truncationHint}
}

// shrink caps strings at cap bytes and halves arrays until the size fits.
func shrink(v any, limit, cap int) {
	capStrings(v, cap)
	for i := 0; i < 40 && jsonSize(v) > limit; i++ {
		if !halveLargestArray(v) {
			return
		}
	}
}

func capStrings(v any, cap int) {
	switch t := v.(type) {
	case []any:
		for i, x := range t {
			if s, ok := x.(string); ok {
				t[i] = cutRunes(s, cap)
			} else {
				capStrings(x, cap)
			}
		}
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && k != "hint" {
				t[k] = cutRunes(s, cap)
			} else {
				capStrings(x, cap)
			}
		}
	}
}

func cutRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "...[truncated]"
}

// halveLargestArray halves the biggest array found; false when none can shrink.
func halveLargestArray(v any) bool {
	var best *[]any
	var bestParent map[string]any
	var bestKey string
	bestSize := 0
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, c := range t {
				if arr, ok := c.([]any); ok && len(arr) > 1 {
					if sz := jsonSize(arr); sz > bestSize {
						cp := arr
						best, bestParent, bestKey, bestSize = &cp, t, k, sz
					}
				}
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	if best == nil {
		return false
	}
	bestParent[bestKey] = (*best)[:len(*best)/2]
	return true
}
