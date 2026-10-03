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
	if ContainsPrivateKey(string(raw)) {
		return withholdKeyMaterial(m)
	}
	m["text"] = string(raw)
	return m
}

// withholdKeyMaterial drops content that holds private key material even when
// the file name looked harmless.
func withholdKeyMaterial(m map[string]any) map[string]any {
	delete(m, "content")
	m["withheld"] = "private key material"
	return m
}

// guardChunkContent applies the same private-key check to readChunk's base64.
func guardChunkContent(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	enc, _ := m["content"].(string)
	if raw, err := base64.StdEncoding.DecodeString(enc); err == nil && ContainsPrivateKey(string(raw)) {
		return withholdKeyMaterial(m)
	}
	return m
}
