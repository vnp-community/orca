package tools

import (
	"encoding/base64"
	"unicode/utf8"
)

// decodeFilePreview turns ReadFilePreview's base64 bytes into text; binary
// content is reported as metadata so the agent never receives raw base64.
func decodeFilePreview(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	enc, _ := m["content"].(string)
	raw, err := base64.StdEncoding.DecodeString(enc)
	delete(m, "content")
	if err != nil || !utf8.Valid(raw) {
		m["binary"] = true
		m["size"] = len(raw)
		return m
	}
	m["text"] = string(raw)
	return m
}
