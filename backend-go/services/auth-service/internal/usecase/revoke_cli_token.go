package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// RevokeCliTokenInput identifies the token to revoke (JTI) and the user it
// must belong to. Both UserID and CallerUserID ultimately trace back to
// the authenticated caller (api-gateway's identityFromContext /
// tenant.UserID(ctx) respectively — see IssueServiceTokenInput's doc
// comment for the same split) — self-service only, mirrors
// IssueServiceToken's CallerUserID convention (CR-CLI-002/TASK-BE-CLI-004):
// this is the same class of "never trust a request field as identity"
// guard, applied to the sibling revoke/list RPCs introduced alongside it.
type RevokeCliTokenInput struct {
	JTI string
	// UserID is who the route claims owns this token — api-gateway sets it
	// from identity.UserID, never a request body/path field a client
	// controls directly.
	UserID string
	// CallerUserID is the actual authenticated caller, from
	// tenant.UserID(ctx) — see IssueServiceTokenInput.CallerUserID's doc
	// comment for the full propagation chain.
	CallerUserID string
}

// RevokeCliToken is the self-service counterpart to RevokeSession
// (admin-console only) — a user may revoke their OWN issued CLI/service
// tokens, no admin/OPA gate, no cross-user override (CR-CLI-002).
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
	if in.CallerUserID == "" {
		return apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_MISSING_CALLER", "internal: caller identity not propagated", nil)
	}
	if in.CallerUserID != in.UserID {
		return apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_FORBIDDEN", "cannot act on another user's token", nil)
	}

	tokens, err := uc.tokens.ListServiceTokensForUser(ctx, in.UserID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_LOOKUP_FAILED", "failed to look up issued tokens", err)
	}
	owned := false
	for _, t := range tokens {
		if t.JTI == in.JTI {
			owned = true
			break
		}
	}
	if !owned {
		// A jti that exists but belongs to a DIFFERENT user must look
		// identical to a jti that doesn't exist at all — NotFound, never
		// PermissionDenied, so this endpoint cannot be used to enumerate
		// other users' token ids by observing 403 vs 404.
		return apperrors.New(apperrors.KindNotFound, "AUTH_CLI_TOKEN_NOT_FOUND", "issued token not found", nil)
	}

	now := uc.clock.Now()
	if err := uc.tokens.Revoke(ctx, in.JTI, now); err != nil {
		if errors.Is(err, ErrServiceTokenNotFound) {
			return apperrors.New(apperrors.KindNotFound, "AUTH_CLI_TOKEN_NOT_FOUND", "issued token not found", err)
		}
		return apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_REVOKE_FAILED", "failed to revoke issued token", err)
	}

	// Audit is best-effort — never fails the revoke itself, mirrors
	// IssueServiceToken's own audit call. tenantID looked up via the user
	// record (IssuedServiceToken carries no tenant_id of its own — see that
	// migration's schema, unchanged from the task's original design).
	if user, err := uc.users.GetUserByID(ctx, in.UserID); err == nil {
		if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, in.UserID, "cli_token.revoked", in.JTI, domain.OutcomeAllowed, "", now); err == nil {
			_ = uc.audit.Append(ctx, entry)
		}
	}
	return nil
}
