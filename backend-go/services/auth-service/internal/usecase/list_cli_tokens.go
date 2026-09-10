package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ListCliTokensInput mirrors RevokeCliTokenInput's UserID/CallerUserID
// split (self-service only, CR-CLI-002/TASK-BE-CLI-004's convention) — see
// that type's doc comment.
type ListCliTokensInput struct {
	UserID       string
	CallerUserID string
}

// ListCliTokens is the self-service counterpart to ListSessionsForUser
// (admin-console only) — a user may list their OWN issued CLI/service
// tokens, no admin/OPA gate, no cross-user override.
type ListCliTokens struct {
	tokens ServiceTokenRepository
}

func NewListCliTokens(tokens ServiceTokenRepository) *ListCliTokens {
	return &ListCliTokens{tokens: tokens}
}

func (uc *ListCliTokens) Execute(ctx context.Context, in ListCliTokensInput) ([]domain.IssuedServiceToken, error) {
	if in.CallerUserID == "" {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_MISSING_CALLER", "internal: caller identity not propagated", nil)
	}
	if in.CallerUserID != in.UserID {
		return nil, apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_FORBIDDEN", "cannot list another user's tokens", nil)
	}

	tokens, err := uc.tokens.ListServiceTokensForUser(ctx, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_LOOKUP_FAILED", "failed to list issued tokens", err)
	}
	return tokens, nil
}
