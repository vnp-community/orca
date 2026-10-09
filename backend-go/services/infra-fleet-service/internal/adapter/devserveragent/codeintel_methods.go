package devserveragent

import "strings"

// IsCodeIntelMethod returns true if the JSON-RPC method belongs to the codeintel or quality group.
func IsCodeIntelMethod(method string) bool {
	return strings.HasPrefix(method, "codeintel.") || strings.HasPrefix(method, "quality.")
}

// IsStructuredErrorMethod is true for methods whose JSON-RPC error.data the backend maps to typed errors
// (code intelligence, and ai.complete whose reasons tell the caller what to fix). Other methods keep returning raw errors.
func IsStructuredErrorMethod(method string) bool {
	return IsCodeIntelMethod(method) || method == "ai.complete"
}
