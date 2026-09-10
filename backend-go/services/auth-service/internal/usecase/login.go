package usecase

import (
	"context"
	"strings"
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
	Email     string
	Password  string
	IP        string // resolved client IP, see LoginRequest.ip
	UserAgent string
}

// LoginOutput carries the raw session token — the ONLY point in the
// system's lifetime this raw value exists outside the caller's hands; from
// here on only its hash is ever seen again (domain.Session doc comment).
type LoginOutput struct {
	SessionToken string
	// RefreshToken lets api-gateway's /auth/local set an orca_refresh
	// cookie alongside orca_session, so /auth/refresh (TASK-BE-012) has
	// something to rotate from on this session's very first request — a
	// session created without one (as every session was, before this
	// change) can never be refreshed, since RefreshSession looks a session
	// up BY its refresh-token hash (CR-RBAC-003, found while wiring
	// TASK-BE-012).
	RefreshToken string
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
	// Minimal format pre-check (spec: "Zod schema: email format, password
	// min 8 chars") — low severity, a malformed password would otherwise
	// just fail the bcrypt compare; this only saves a wasted user lookup.
	if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		uc.appendFailureAuditBestEffort(ctx, in, "", "AUTH_INVALID_FORMAT")
		return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_FORMAT", "invalid email or password format", nil)
	}

	user, passwordHash, err := uc.users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		// Deliberately the same error for "no such user" and "wrong
		// password" below — do not let Login leak which one it was.
		uc.appendFailureAuditBestEffort(ctx, in, "", "AUTH_INVALID_CREDENTIALS")
		return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
	}
	if !user.IsActive {
		uc.appendFailureAuditBestEffort(ctx, in, user.ID, "AUTH_ACCOUNT_DEACTIVATED")
		return LoginOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_ACCOUNT_DEACTIVATED", "account is deactivated", nil)
	}
	if err := uc.hasher.Compare(passwordHash, in.Password); err != nil {
		uc.appendFailureAuditBestEffort(ctx, in, user.ID, "AUTH_INVALID_CREDENTIALS")
		return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
	}

	rawToken, rawRefreshToken, now, err := createSessionForUser(ctx, uc.sessions, uc.clock, uc.sessionTTL, user, in.IP, in.UserAgent)
	if err != nil {
		return LoginOutput{}, err
	}

	uc.appendAuditBestEffort(ctx, user, in, now)

	return LoginOutput{SessionToken: rawToken, RefreshToken: rawRefreshToken, User: user}, nil
}

// createSessionForUser mints a fresh opaque session token and persists its
// hash — the one piece of Login's original logic LoginOrProvisionSsoUser
// also needs verbatim (SSO login must produce the exact same orca_session-
// cookie-compatible session a local login does), so it's factored out here
// rather than duplicated. Returns the raw session token and raw refresh
// token (see LoginOutput's doc comment on why the session token is the only
// place THAT value ever exists outside the caller's hands — the same
// principle applies to the refresh token) and the "now" instant used, so
// callers can reuse it for their own audit entry without a second clock
// read.
//
// Every session is issued a refresh token at creation (CR-RBAC-003,
// TASK-BE-012) — RefreshSession (TASK-BE-011) looks a session up BY its
// refresh-token hash, so a session minted without one could never be
// refreshed; DefaultRefreshTokenTTL (refresh_session.go) is reused here so
// both places agree on the default lifetime.
func createSessionForUser(ctx context.Context, sessions SessionRepository, clock Clock, sessionTTL time.Duration, user domain.User, ip, userAgent string) (rawToken, rawRefreshToken string, now time.Time, err error) {
	rawToken, err = generateRandomToken(32)
	if err != nil {
		return "", "", time.Time{}, apperrors.New(apperrors.KindInternal, "AUTH_TOKEN_GEN_FAILED", "failed to generate session token", err)
	}
	rawRefreshToken, err = generateRandomToken(32)
	if err != nil {
		return "", "", time.Time{}, apperrors.New(apperrors.KindInternal, "AUTH_TOKEN_GEN_FAILED", "failed to generate refresh token", err)
	}

	now = clock.Now()
	session, err := domain.NewSession(domain.HashSessionToken(rawToken), user.ID, user.TenantID, now, now.Add(sessionTTL))
	if err != nil {
		return "", "", time.Time{}, apperrors.New(apperrors.KindInternal, "AUTH_INVALID_SESSION", err.Error(), err)
	}
	session = session.WithClientInfo(ip, userAgent)
	session.RefreshTokenHash = domain.HashSessionToken(rawRefreshToken)
	session.RefreshExpiresAt = now.Add(DefaultRefreshTokenTTL)
	if err := sessions.CreateSession(ctx, session); err != nil {
		return "", "", time.Time{}, apperrors.New(apperrors.KindInternal, "AUTH_SESSION_CREATE_FAILED", "failed to create session", err)
	}
	return rawToken, rawRefreshToken, now, nil
}

// appendAuditBestEffort mirrors usage-service's "the write that already
// committed doesn't fail because of a best-effort side effect" pattern —
// see that service's RecordUsageSession doc comment for the same rationale
// applied to event publishing.
func (uc *Login) appendAuditBestEffort(ctx context.Context, user domain.User, in LoginInput, now time.Time) {
	metadata := map[string]any{"ip": in.IP, "userAgent": in.UserAgent}
	entry, err := domain.NewAuditEntry(uuid.NewString(), user.TenantID, user.ID, "user.login", "", "user", user.ID, metadata, domain.OutcomeAllowed, in.IP, now)
	if err != nil {
		return
	}
	_ = uc.audit.Append(ctx, entry)
}

// appendFailureAuditBestEffort writes a login.fail entry — mirrors
// appendAuditBestEffort's best-effort pattern so an audit-write failure
// never turns a real auth decision into a 500. ActorID is empty (no
// authenticated user exists on a failed login). userID is the best-effort
// resolved user ID when GetUserByEmail already succeeded by the time this
// failure branch ran (deactivated account, wrong password) — "" when it
// never resolved (invalid format, unknown email).
func (uc *Login) appendFailureAuditBestEffort(ctx context.Context, in LoginInput, userID, reason string) {
	metadata := map[string]any{"ip": in.IP, "email": in.Email, "reason": reason}
	entry, err := domain.NewAuditEntry(uuid.NewString(), tenantIDOrUnknown(in), "", "login.fail", "", "user", userID, metadata, domain.OutcomeDenied, in.IP, uc.clock.Now())
	if err != nil {
		return
	}
	_ = uc.audit.Append(ctx, entry)
}

// tenantIDOrUnknown: a failed login by email alone has no resolved tenant
// (GetUserByEmail's lookup is itself tenant-less at this layer). Uses a
// fixed sentinel so domain.NewAuditEntry's ErrEmptyTenant invariant is
// satisfiable — SOL-AUTH-05 should revisit system-wide audit entries with
// no resolvable tenant properly; this is a stopgap, not a real model.
func tenantIDOrUnknown(in LoginInput) string {
	return "unknown"
}
