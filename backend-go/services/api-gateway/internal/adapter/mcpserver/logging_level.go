package mcpserver

// RFC 5424 severities accepted by logging/setLevel (MCP logging utility).
var logLevels = map[string]struct{}{
	"debug": {}, "info": {}, "notice": {}, "warning": {},
	"error": {}, "critical": {}, "alert": {}, "emergency": {},
}

// ValidLogLevel reports whether level is a legal logging/setLevel value.
// The SDK stores any string, so the adapter validates (-32602 otherwise).
func ValidLogLevel(level string) bool {
	_, ok := logLevels[level]
	return ok
}
