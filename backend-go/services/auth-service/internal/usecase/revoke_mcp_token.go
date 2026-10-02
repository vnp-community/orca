package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// RevokeMcpToken revokes one of the caller's own tokens. A jti owned by
// someone else is indistinguishable from an unknown one (unlike RevokeCliToken,
// which cannot verify ownership).
type RevokeMcpToken struct {
	tokens McpTokenRepository
	audit  AuditRepository
	clock  Clock
}

func NewRevokeMcpToken(tokens McpTokenRepository, audit AuditRepository, clock Clock) *RevokeMcpToken {
	return &RevokeMcpToken{tokens: tokens, audit: audit, clock: clock}
}

func (uc *RevokeMcpToken) Execute(ctx context.Context, tenantID, userID, jti string) error {
	if tenantID == "" || userID == "" {
		return errMcp(apperrors.KindUnauthenticated, CodeMcpTokenNotFound, "caller identity is required")
	}
	if jti == "" {
		return errMcp(apperrors.KindNotFound, CodeMcpTokenNotFound, "token not found")
	}
	now := uc.clock.Now()
	if err := uc.tokens.RevokeMcpToken(ctx, tenantID, userID, jti, now); err != nil {
		if errors.Is(err, ErrMcpTokenNotFound) {
			return errMcp(apperrors.KindNotFound, CodeMcpTokenNotFound, "token not found")
		}
		return apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_REVOKE_FAILED", "failed to revoke token", err)
	}
	if e, err := domain.NewAuditEntry(uuid.NewString(), tenantID, userID, "mcp_token.revoked", "", "mcp_token", jti, map[string]any{"by": userID}, domain.OutcomeAllowed, "", now); err == nil {
		_ = uc.audit.Append(ctx, e)
	}
	return nil
}
