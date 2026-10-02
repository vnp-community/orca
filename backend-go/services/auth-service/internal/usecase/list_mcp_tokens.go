package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ListMcpTokens lists the caller's own tokens (never anyone else's).
type ListMcpTokens struct{ tokens McpTokenRepository }

func NewListMcpTokens(tokens McpTokenRepository) *ListMcpTokens {
	return &ListMcpTokens{tokens: tokens}
}

func (uc *ListMcpTokens) Execute(ctx context.Context, tenantID, userID string) ([]domain.McpToken, error) {
	if tenantID == "" || userID == "" {
		return nil, errMcp(apperrors.KindUnauthenticated, CodeMcpTokenNotFound, "caller identity is required")
	}
	out, err := uc.tokens.ListMcpTokens(ctx, tenantID, userID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_MCP_TOKEN_LIST_FAILED", "failed to list tokens", err)
	}
	return out, nil
}
