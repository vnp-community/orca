package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ErrMcpTokenNotFound: no such token for this tenant/user (also returned when
// it exists but belongs to someone else, or is already revoked).
var ErrMcpTokenNotFound = errors.New("usecase: mcp token not found")

// McpTokenRepository persists MCP personal access tokens (BE-MCP-SOL-006).
// Only the SHA-256 of the secret is ever stored. Every method binds tenantID.
type McpTokenRepository interface {
	CreateMcpToken(ctx context.Context, t domain.McpToken) error
	// ListMcpTokens returns the user's tokens, newest first.
	ListMcpTokens(ctx context.Context, tenantID, userID string) ([]domain.McpToken, error)
	GetMcpToken(ctx context.Context, tenantID, jti string) (domain.McpToken, error)
	// RevokeMcpToken revokes an active token owned by userID; ErrMcpTokenNotFound otherwise.
	RevokeMcpToken(ctx context.Context, tenantID, userID, jti string, at time.Time) error
	// MarkMcpTokenFirstUsed sets first_used_at only if it is still NULL and
	// reports whether this call was the one that set it.
	MarkMcpTokenFirstUsed(ctx context.Context, tenantID, jti string, at time.Time) (bool, error)
	// TouchMcpTokenLastUsed sets last_used_at when it is NULL or older than staleBefore.
	TouchMcpTokenLastUsed(ctx context.Context, tenantID, jti string, at, staleBefore time.Time) error
}
