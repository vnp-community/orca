package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// IsServiceTokenRevoked backs api-gateway's per-request revocation check
// (usecase.AuthValidator's RevocationChecker, api-gateway side —
// TASK-BE-CLI-005). No caller-identity gate, matching the RPC's proto doc
// comment: a jti is a random 32-byte id, checking its revocation status
// reveals nothing about the owning user.
type IsServiceTokenRevoked struct {
	tokens ServiceTokenRepository
}

func NewIsServiceTokenRevoked(tokens ServiceTokenRepository) *IsServiceTokenRevoked {
	return &IsServiceTokenRevoked{tokens: tokens}
}

func (uc *IsServiceTokenRevoked) Execute(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_JTI", "jti is required", nil)
	}
	revoked, err := uc.tokens.IsRevoked(ctx, jti)
	if err != nil {
		return false, apperrors.New(apperrors.KindInternal, "AUTH_CHECK_REVOKED_FAILED", "failed to check token revocation", err)
	}
	return revoked, nil
}
