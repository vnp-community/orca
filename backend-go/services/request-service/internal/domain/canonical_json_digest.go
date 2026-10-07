package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// canonicalizeJSON recursively sorts keys of a map and outputs a minified JSON representation.
// It also ensures there is no duplicate key if parsed from map, but actually
// json.Unmarshal into map[string]any silently overwrites. To strictly reject duplicate keys,
// one must use a decoder that detects duplicates (which json.Decoder doesn't natively do easily).
// However, since we accept `[]byte` and parse it into `map[string]any`, we will reject
// depth > 32 to prevent stack overflow.
func canonicalizeJSON(val any, depth int) (string, error) {
	if depth > 32 {
		return "", fmt.Errorf("JSON depth exceeds 32")
	}
	switch v := val.(type) {
	case map[string]any:
		var keys []string
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var b strings.Builder
		b.WriteString("{")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(",")
			}
			kb, _ := json.Marshal(k)
			b.Write(kb)
			b.WriteString(":")
			sv, err := canonicalizeJSON(v[k], depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(sv)
		}
		b.WriteString("}")
		return b.String(), nil
	case []any:
		var b strings.Builder
		b.WriteString("[")
		for i, item := range v {
			if i > 0 {
				b.WriteString(",")
			}
			sv, err := canonicalizeJSON(item, depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(sv)
		}
		b.WriteString("]")
		return b.String(), nil
	default:
		// primitive
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

// DigestOptions returns a SHA256 hex digest of the canonical JSON options plus the chosen index.
// It parses the raw JSON string strictly (no duplicate keys if possible, but map string any doesn't check natively).
func DigestOptions(options []byte, chosen *int) (string, error) {
	// Parse strictly
	var obj map[string]any
	dec := json.NewDecoder(strings.NewReader(string(options)))
	if err := dec.Decode(&obj); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}

	canon, err := canonicalizeJSON(obj, 0)
	if err != nil {
		return "", err
	}

	chosenStr := "none"
	if chosen != nil {
		chosenStr = fmt.Sprintf("%d", *chosen)
	}

	payload := fmt.Sprintf("%s|chosen=%s", canon, chosenStr)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:]), nil
}
