package tools

// Exports for mcpserver/resources, which shares the tool pipeline's
// normalisation and redaction so a resource can never reveal more than the
// equivalent read tool.

// RedactText applies the same secret redaction rules as tool results.
func RedactText(s string) string { return redactString(s) }

// NormalizeChannelResult turns a channel result into a redacted JSON object,
// bounded by maxBytes (truncated results carry truncated=true).
func NormalizeChannelResult(res any, untrusted bool, maxBytes int) (map[string]any, error) {
	return normalizeResult(res, &ToolSpec{Untrusted: untrusted, MaxResultBytes: maxBytes})
}

// NormalizeFilePreview is NormalizeChannelResult for files.readPreview: the
// base64 content becomes text (or binary metadata).
func NormalizeFilePreview(res any, maxBytes int) (map[string]any, error) {
	return normalizeResult(res, &ToolSpec{Untrusted: true, MaxResultBytes: maxBytes, Post: decodeFilePreview})
}

// MapExecError exposes the executor's error shaping (never leaks internals):
// the result starts with a stable CODE.
func MapExecError(err error) error { return mapExecError(err) }
