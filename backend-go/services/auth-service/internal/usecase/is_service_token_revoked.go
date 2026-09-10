package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

<<<<<<< HEAD
// IsServiceTokenRevoked backs api-gateway's per-request bearer-JWT
// revocation check (usecase.AuthValidator's RevocationChecker,
// TASK-BE-CLI-005) — deliberately narrow (just a jti -> bool lookup, not
// the full token record). No caller-identity gate: any authenticated
// internal caller may check any jti's revocation status (see
// ServiceTokenRepository.IsRevoked's doc comment).
=======
// IsServiceTokenRevoked backs api-gateway's per-request revocation check
// (usecase.AuthValidator's RevocationChecker, api-gateway side —
// TASK-BE-CLI-005). No caller-identity gate, matching the RPC's proto doc
// comment: a jti is a random 32-byte id, checking its revocation status
// reveals nothing about the owning user.
>>>>>>> feat/team-rbac-implementation
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
<<<<<<< HEAD
		return false, apperrors.New(apperrors.KindInternal, "AUTH_IS_SERVICE_TOKEN_REVOKED_FAILED", "failed to check token revocation", err)
=======
		return false, apperrors.New(apperrors.KindInternal, "AUTH_CHECK_REVOKED_FAILED", "failed to check token revocation", err)
>>>>>>> feat/team-rbac-implementation
	}
	return revoked, nil
}
