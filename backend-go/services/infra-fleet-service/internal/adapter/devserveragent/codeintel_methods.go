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

// agentPathNotFoundCode is AgentErrorCode.PathNotFound of the agent's wire protocol (agent/src/shared/agent-wire-protocol.ts).
const agentPathNotFoundCode = -33003

// isMissingPathOfFileMethod is true for the agent's answer "that path is not there" to a file method (fs.stat, fs.readFile...).
// It is kept as a typed error so the backend can tell it from a relay failure; other file errors stay raw.
func isMissingPathOfFileMethod(method string, e *JSONRPCError) bool {
	return strings.HasPrefix(method, "fs.") && e.Code == agentPathNotFoundCode
}
