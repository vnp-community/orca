package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ListCliTokensInput mirrors ListCliTokensRequest 1:1 — CallerUserID is not
// part of the wire request (see this usecase's doc comment).
type ListCliTokensInput struct {
	UserID string
	// CallerUserID comes from tenant.UserID(ctx), never a request field —
	// see this usecase's doc comment.
	CallerUserID string
}

// ListCliTokens is self-service only (TASK-BE-CLI-006) — the caller may
// only list their own tokens. UserID comes from the request field
// api-gateway's route sets from the caller's own resolved identity (never a
// client-controlled value in practice), but this usecase re-verifies it
// against CallerUserID rather than trusting the request field alone
// (CR-CLI-002/TASK-BE-CLI-004's "never trust identity from a request field"
// rule applied to this sibling RPC) — same self-mint-only posture as
// IssueServiceToken.
type ListCliTokens struct {
	tokens ServiceTokenRepository
}

func NewListCliTokens(tokens ServiceTokenRepository) *ListCliTokens {
	return &ListCliTokens{tokens: tokens}
}

func (uc *ListCliTokens) Execute(ctx context.Context, in ListCliTokensInput) ([]domain.IssuedServiceToken, error) {
	if in.UserID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_USER_ID", "user_id is required", nil)
	}
	if in.CallerUserID == "" || in.CallerUserID != in.UserID {
		return nil, apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_SELF_SERVICE_ONLY", "may only list your own tokens", nil)
	}

	tokens, err := uc.tokens.ListServiceTokensForUser(ctx, in.UserID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "AUTH_LIST_CLI_TOKENS_FAILED", "failed to list issued tokens", err)
	}
	return tokens, nil
}
