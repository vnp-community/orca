package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// serviceTokenJTIBytes is the entropy generateRandomToken reads for a
// service token's "jti" claim — same size Login uses for session tokens
// (see internal/usecase/token.go).
const serviceTokenJTIBytes = 32

type IssueServiceTokenInput struct {
	UserID   string
	Audience string
<<<<<<< HEAD
	// CallerUserID is not yet enforced — see this usecase's KNOWN GAP doc
	// comment below. Threaded through from the gRPC layer (tenant.UserID(ctx))
	// so it's available once that gap is closed.
=======
	// CallerUserID is the identity of whoever is calling this RPC, ALWAYS
	// resolved from the authenticated gRPC caller context (tenant.UserID,
	// populated by grpcmw.TenantExtractionInterceptor from the
	// x-orca-user-id metadata api-gateway's AttachIdentity sets — see
	// adapter/grpc/server.go's IssueServiceToken handler) — NEVER from a
	// field in IssueServiceTokenRequest (no such field exists, and none
	// should ever be added: a client must never be able to declare its own
	// identity).
>>>>>>> feat/team-rbac-implementation
	CallerUserID string
}

type IssueServiceTokenOutput struct {
	JWT       string
	ExpiresAt time.Time
}

// IssueServiceToken mints a real RS256 JWT (signed via Vault Transit,
// TokenSigner) for an existing user. Per this task's design, "sub" is the
// requested user_id (after verifying it exists), "tenant_id" comes from
// that user's own record, and "aud" is the request's audience verbatim.
// Records the mint in ServiceTokenRepository (CR-CLI-002/TASK-BE-CLI-005) so
// IsServiceTokenRevoked/ListCliTokens/RevokeCliToken have a row to act on,
// and appends a best-effort audit entry (same "the write that already
// committed doesn't fail because of a side effect" posture Login's
// appendAuditBestEffort uses).
//
<<<<<<< HEAD
// KNOWN GAP (see this service's README "Known gaps"): the generated
// IssueServiceTokenRequest carries no caller-identity field, so there is no
// check that the *requester* of a token is itself authorized to mint one
// for the given user_id — this usecase only verifies the target user
// exists, not who is asking. Fixing that needs a proto change outside this
// task's scope. CallerUserID is captured (from tenant.UserID(ctx)) for the
// audit trail only, not enforced.
=======
// Caller authorization (CR-CLI-002/TASK-BE-CLI-004, decision 2026-09-09):
// v1 is self-mint-only — a caller may only mint a token for itself, no
// admin-override/target_user_id exception. Enforced by comparing
// CallerUserID (always the authenticated caller, see that field's doc
// comment) against UserID below, fail-closed if the caller identity did not
// propagate at all.
>>>>>>> feat/team-rbac-implementation
type IssueServiceToken struct {
	users  UserRepository
	tokens ServiceTokenRepository
	audit  AuditRepository
	signer TokenSigner
	clock  Clock
	ttl    time.Duration
}

func NewIssueServiceToken(users UserRepository, tokens ServiceTokenRepository, audit AuditRepository, signer TokenSigner, clock Clock, ttl time.Duration) *IssueServiceToken {
	return &IssueServiceToken{users: users, tokens: tokens, audit: audit, signer: signer, clock: clock, ttl: ttl}
}

func (uc *IssueServiceToken) Execute(ctx context.Context, in IssueServiceTokenInput) (IssueServiceTokenOutput, error) {
	if in.UserID == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_USER_ID", "user_id is required", nil)
	}
	if in.Audience == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_AUDIENCE", "audience is required", nil)
	}
	if in.CallerUserID == "" {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_CLI_TOKEN_MISSING_CALLER", "internal: caller identity not propagated", nil)
	}
	if in.CallerUserID != in.UserID {
		// v1: KHÔNG có ngoại lệ nào (không admin-override) — quyết định 2026-09-09.
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_CLI_TOKEN_FORBIDDEN", "cannot mint a token for another user", nil)
	}

	user, err := uc.users.GetUserByID(ctx, in.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindNotFound, "AUTH_USER_NOT_FOUND", "user not found", err)
	}
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_LOOKUP_FAILED", "failed to look up user", err)
	}

	jti, err := generateRandomToken(serviceTokenJTIBytes)
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_JTI_FAILED", "failed to generate token id", err)
	}

	now := uc.clock.Now()
	expiresAt := now.Add(uc.ttl)
	claims := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer:   jwtauth.Issuer,
			Subject:  user.ID,
			Audience: jwt.Audience{in.Audience},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(expiresAt),
			ID:       jti,
		},
		TenantID: user.TenantID,
		Role:     string(user.Role),
	}

	token, err := uc.signer.Sign(ctx, claims)
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_SIGN_FAILED", "failed to sign token", err)
	}

<<<<<<< HEAD
	if err := uc.tokens.RecordIssuedToken(ctx, domain.IssuedServiceToken{
		JTI: jti, UserID: user.ID, Audience: in.Audience, IssuedAt: now, ExpiresAt: expiresAt,
	}); err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_RECORD_FAILED", "failed to record issued token", err)
	}

	if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, in.CallerUserID, "service_token.issued", "", "service_token", jti, nil, domain.OutcomeAllowed, "", now); err == nil {
=======
	// Record the jti AFTER a successful sign — a row with no corresponding
	// JWT ever minted would be harmless dead data, but the reverse (a JWT
	// nothing can ever revoke) is the exact gap this task closes; never
	// swap this ordering.
	issued, err := domain.NewIssuedServiceToken(jti, user.ID, in.Audience, now, expiresAt)
	if err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_INVALID_RECORD", "failed to build issued token record", err)
	}
	if err := uc.tokens.RecordIssuedToken(ctx, issued); err != nil {
		return IssueServiceTokenOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_ISSUE_TOKEN_RECORD_FAILED", "failed to record issued token", err)
	}

	// Audit is best-effort — never fails the mint itself (mirrors every
	// other usecase's audit call, e.g. UpdateUserRole). actor = user.ID:
	// v1 is self-mint-only (CallerUserID == UserID, enforced above), so
	// actor and target user are always the same identity; target = jti
	// (the thing that can actually be revoked later), not user_id (already
	// implied by actor).
	if entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, user.ID, "cli_token.issued", jti, domain.OutcomeAllowed, "", now); err == nil {
>>>>>>> feat/team-rbac-implementation
		_ = uc.audit.Append(ctx, entry)
	}

	return IssueServiceTokenOutput{JWT: token, ExpiresAt: expiresAt}, nil
}
