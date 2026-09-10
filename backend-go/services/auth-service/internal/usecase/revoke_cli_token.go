package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// RevokeCliTokenInput mirrors RevokeCliTokenRequest 1:1 — CallerUserID is
// not part of the wire request (see this usecase's doc comment).
type RevokeCliTokenInput struct {
	JTI    string
	UserID string
	// CallerUserID comes from tenant.UserID(ctx), never a request field —
	// see this usecase's doc comment.
	CallerUserID string
}

// RevokeCliToken is self-service only (TASK-BE-CLI-006) — same posture as
// ListCliTokens: UserID is re-verified against CallerUserID rather than
// trusted from the request field alone. ServiceTokenRepository has no
// "get by jti" method (see its own doc comment), so this usecase cannot
// verify jti actually belongs to UserID before revoking — the self-service
// UserID check is the only gate.
type RevokeCliToken struct {
	users  UserRepository
	tokens ServiceTokenRepository
	audit  AuditRepository
	clock  Clock
}

func NewRevokeCliToken(users UserRepository, tokens ServiceTokenRepository, audit AuditRepository, clock Clock) *RevokeCliToken {
	return &RevokeCliToken{users: users, tokens: tokens, audit: audit, clock: clock}
}

func (uc *RevokeCliToken) Execute(ctx context.Context, in RevokeCliTokenInput) error {
	if in.JTI == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_JTI", "jti is required", nil)
	}
	if in.UserID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_USER_ID", "user_id is required", nil)
	}
	if in.CallerUserID == "" || in.CallerUserID != in.UserID {
		return apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_SELF_SERVICE_ONLY", "may only revoke your own tokens", nil)
	}

	// Looked up only to get TenantID for the audit entry below — see this
	// usecase's doc comment for why jti ownership itself can't be verified.
	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "AUTH_USER_NOT_FOUND", "user not found", err)
	}

	now := uc.clock.Now()
	if err := uc.tokens.Revoke(ctx, in.JTI, now); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_REVOKE_CLI_TOKEN_FAILED", "failed to revoke token", err)
	}

	if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, in.CallerUserID, "cli_token.revoked", "", "service_token", in.JTI, nil, domain.OutcomeAllowed, "", now); err == nil {
		_ = uc.audit.Append(ctx, entry)
	}
	return nil
}
