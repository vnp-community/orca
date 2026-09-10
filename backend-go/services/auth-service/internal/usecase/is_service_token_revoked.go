package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// IsServiceTokenRevoked backs api-gateway's per-request bearer-JWT
// revocation check (usecase.AuthValidator's RevocationChecker,
// TASK-BE-CLI-005) — deliberately narrow (just a jti -> bool lookup, not
// the full token record). No caller-identity gate: any authenticated
// internal caller may check any jti's revocation status (see
// ServiceTokenRepository.IsRevoked's doc comment).
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
		return false, apperrors.New(apperrors.KindInternal, "AUTH_IS_SERVICE_TOKEN_REVOKED_FAILED", "failed to check token revocation", err)
	}
	return revoked, nil
}
