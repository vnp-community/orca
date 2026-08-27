package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// DefaultSessionTTL is used when no explicit TTL is configured — see
// cmd/server/main.go / internal/config.
const DefaultSessionTTL = 24 * time.Hour

// LoginInput mirrors the gRPC LoginRequest 1:1 — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods.
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput carries the raw session token — the ONLY point in the
// system's lifetime this raw value exists outside the caller's hands; from
// here on only its hash is ever seen again (domain.Session doc comment).
type LoginOutput struct {
	SessionToken string
	User         domain.User
}

// Login verifies the caller's credentials, creates a new session (always a
// fresh one — never reuses a pre-auth session ID, per auth-service.md §9),
// and appends an audit entry.
type Login struct {
	users      UserRepository
	sessions   SessionRepository
	audit      AuditRepository
	hasher     PasswordHasher
	clock      Clock
	sessionTTL time.Duration
}

func NewLogin(users UserRepository, sessions SessionRepository, audit AuditRepository, hasher PasswordHasher, clock Clock, sessionTTL time.Duration) *Login {
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}
	return &Login{users: users, sessions: sessions, audit: audit, hasher: hasher, clock: clock, sessionTTL: sessionTTL}
}

func (uc *Login) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	if in.Email == "" || in.Password == "" {
		return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_CREDENTIALS", "email and password are required", nil)
	}

	user, passwordHash, err := uc.users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		// Deliberately the same error for "no such user" and "wrong
		// password" below — do not let Login leak which one it was.
		return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
	}
	if !user.IsActive {
		return LoginOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_ACCOUNT_DEACTIVATED", "account is deactivated", nil)
	}
	if err := uc.hasher.Compare(passwordHash, in.Password); err != nil {
		return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
	}

	rawToken, err := generateRandomToken(32)
	if err != nil {
		return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_TOKEN_GEN_FAILED", "failed to generate session token", err)
	}

	now := uc.clock.Now()
	session, err := domain.NewSession(domain.HashSessionToken(rawToken), user.ID, user.TenantID, now, now.Add(uc.sessionTTL))
	if err != nil {
		return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_INVALID_SESSION", err.Error(), err)
	}
	if err := uc.sessions.CreateSession(ctx, session); err != nil {
		return LoginOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_SESSION_CREATE_FAILED", "failed to create session", err)
	}

	uc.appendAuditBestEffort(ctx, user, now)

	return LoginOutput{SessionToken: rawToken, User: user}, nil
}

// appendAuditBestEffort mirrors usage-service's "the write that already
// committed doesn't fail because of a best-effort side effect" pattern —
// see that service's RecordUsageSession doc comment for the same rationale
// applied to event publishing.
func (uc *Login) appendAuditBestEffort(ctx context.Context, user domain.User, now time.Time) {
	entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, user.ID, "user.login", user.ID, now)
	if err != nil {
		return
	}
	_ = uc.audit.Append(ctx, entry)
}
