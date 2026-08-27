# TASK-AUTH-01-02: `Login.Execute` gains `IP`/`UserAgent` input, format pre-check, and `login.fail` audit

**From Solution:** SOL-AUTH-01
**Priority:** P0
**Service:** `auth-service` (usecase)
**File:** `backend-go/services/auth-service/internal/usecase/login.go`
**Depends on:** TASK-AUTH-01-01
**Status:** `[x]` DONE — `LoginInput.IP/UserAgent`, format pre-check, and `login.fail` audit on every failure branch implemented; `TestLogin_*` (incl. new `TestLogin_InvalidFormatFails`) pass.

---

## Context

`Login.Execute` already returns distinct `Kind`s for a deactivated account (`KindPermissionDenied`) vs. bad credentials (`KindUnauthenticated`), but never writes an audit entry on any failure path, and has no minimal email/password format check. This task adds `LoginInput.IP`/`.UserAgent`, a format pre-check, and a best-effort `login.fail` audit write on every failure branch — using `AuditEntry`'s current single-`Target` shape (SOL-AUTH-05 later widens it to carry real `{ip, email}` metadata; this task must not block on that).

## Changes to make

In `backend-go/services/auth-service/internal/usecase/login.go`:

```go
import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// LoginInput mirrors the gRPC LoginRequest 1:1 — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods.
type LoginInput struct {
	Email     string
	Password  string
	IP        string // resolved client IP, see LoginRequest.ip
	UserAgent string
}
```

Replace `Execute`'s body:

```go
func (uc *Login) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	if in.Email == "" || in.Password == "" {
		return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_MISSING_CREDENTIALS", "email and password are required", nil)
	}
	// Minimal format pre-check (spec: "Zod schema: email format, password
	// min 8 chars") — low severity, a malformed password would otherwise
	// just fail the bcrypt compare; this only saves a wasted user lookup.
	if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
		uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_FORMAT")
		return LoginOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_INVALID_FORMAT", "invalid email or password format", nil)
	}

	user, passwordHash, err := uc.users.GetUserByEmail(ctx, in.Email)
	if err != nil {
		// Deliberately the same error for "no such user" and "wrong
		// password" below — do not let Login leak which one it was.
		uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_CREDENTIALS")
		return LoginOutput{}, apperrors.New(apperrors.KindUnauthenticated, "AUTH_INVALID_CREDENTIALS", "invalid email or password", nil)
	}
	if !user.IsActive {
		uc.appendFailureAuditBestEffort(ctx, in, "AUTH_ACCOUNT_DEACTIVATED")
		return LoginOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_ACCOUNT_DEACTIVATED", "account is deactivated", nil)
	}
	if err := uc.hasher.Compare(passwordHash, in.Password); err != nil {
		uc.appendFailureAuditBestEffort(ctx, in, "AUTH_INVALID_CREDENTIALS")
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

// appendFailureAuditBestEffort writes a login.fail entry — mirrors
// appendAuditBestEffort's best-effort pattern so an audit-write failure
// never turns a real auth decision into a 500. ActorID is empty (no
// authenticated user exists on a failed login) — the attempted email is
// carried as the target until SOL-AUTH-05's Metadata field exists to carry
// {ip, email} together.
func (uc *Login) appendFailureAuditBestEffort(ctx context.Context, in LoginInput, reason string) {
	entry, err := domain.NewAuditEntry(uuid.NewString(), tenantIDOrUnknown(in), "", "login.fail", in.Email, uc.clock.Now())
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
```

`reason` is currently unused inside `appendFailureAuditBestEffort` beyond being passed in — keep the parameter (SOL-AUTH-05 wires it into `Metadata["reason"]` once that field exists); do not drop it from the call sites.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestLogin -v
```

Expected: build succeeds; add/update `login_test.go` per the SOL's test plan (deactivated user, wrong password, unknown email, and format-invalid input each call `audit.Append` with `action: "login.fail"` exactly once via the fake `AuditRepository`; success path unaffected).
