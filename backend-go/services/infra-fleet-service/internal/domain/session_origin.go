package domain

// SessionOrigin records which MCP client session created a terminal or agent
// session (CONTRACT mcp-ui-api §5, BE-MCP-SOL-009). A nil *SessionOrigin means
// the session was created from the UI.
type SessionOrigin struct {
	Type         string // "mcp"
	ClientName   string
	MCPSessionID string // non-secret mcp-service session row id
	UserID       string
}

// IsEmpty reports whether o carries no origin information (nil-safe).
func (o *SessionOrigin) IsEmpty() bool {
	return o == nil || (o.Type == "" && o.ClientName == "" && o.MCPSessionID == "" && o.UserID == "")
}
