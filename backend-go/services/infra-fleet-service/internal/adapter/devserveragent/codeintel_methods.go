package devserveragent

import "strings"

// IsCodeIntelMethod returns true if the JSON-RPC method belongs to the codeintel or quality group.
func IsCodeIntelMethod(method string) bool {
	return strings.HasPrefix(method, "codeintel.") || strings.HasPrefix(method, "quality.")
}
