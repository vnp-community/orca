package domain

import "time"

// MaxMcpTokenDays is the hard lifetime ceiling of an MCP personal access
// token; a tenant may only lower it (enforced at the gateway/mcp policy).
const MaxMcpTokenDays = 90

// McpToken is the stored record of a personal access token. The signed token
// itself is never stored; TokenSHA256 is the hex SHA-256 of the full secret.
type McpToken struct {
	JTI         string
	TenantID    string
	UserID      string
	Name        string
	Scope       string // space-delimited
	TokenSHA256 string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	FirstUsedAt *time.Time
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	RevokedBy   string
}
