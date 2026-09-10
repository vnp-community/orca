# TASK-AUTH-02-06: `ValidateSession` fires a debounced best-effort `TouchLastSeen`

**From Solution:** SOL-AUTH-02
**Priority:** P0
**Service:** `auth-service` (usecase)
**File:** `backend-go/services/auth-service/internal/usecase/validate_session.go`
**Depends on:** TASK-AUTH-02-02, TASK-AUTH-02-03
**Status:** `[x]` DONE — debounced fire-and-forget `touchBestEffort` added; 4 new deterministic tests (nil-LastSeenAt touches, <60s no-touch, >60s touches, touch failure doesn't affect output) pass under `go test -race`.

---

## Context

`ValidateSession` is on literally every browser request's path with a p99 < 20ms SLO (`auth-service.md` §8). A synchronous `UPDATE last_seen_at` on every call would add a write to the hottest read path in the system, so the touch must be fire-and-forget and debounced (at most once per 60s per session) — same "best-effort side effect" shape as `login.go`'s `appendAuditBestEffort`.

## Changes to make

In `backend-go/services/auth-service/internal/usecase/validate_session.go`, add near the top:

```go
const touchDebounce = 60 * time.Second // avoid a write on every single request
```

Change `Execute` (after the existing `IsValid`/`user.IsActive` checks, right before the final `return`):

```go
func (uc *ValidateSession) Execute(ctx context.Context, sessionToken string) (ValidateSessionOutput, error) {
	if sessionToken == "" {
		return ValidateSessionOutput{}, nil
	}

	tokenHash := domain.HashSessionToken(sessionToken)
	session, err := uc.sessions.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return ValidateSessionOutput{}, nil // not found is "invalid", not an error
	}
	if !session.IsValid(uc.clock.Now()) {
		return ValidateSessionOutput{}, nil
	}

	user, err := uc.users.GetUserByID(ctx, session.UserID)
	if err != nil {
		return ValidateSessionOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_USER_LOOKUP_FAILED", "failed to load session's user", err)
	}
	if !user.IsActive {
		return ValidateSessionOutput{}, nil
	}

	if session.LastSeenAt == nil || uc.clock.Now().Sub(*session.LastSeenAt) > touchDebounce {
		uc.touchBestEffort(session.TokenHash) // fire-and-forget, does not block the response
	}

	return ValidateSessionOutput{Valid: true, User: user}, nil
}

// touchBestEffort mirrors login.go's appendAuditBestEffort pattern applied
// to a write instead of an audit append — a failed or slow touch must never
// turn a valid session into a failed request, and per auth-service.md §8's
// p99<20ms budget, must not add a synchronous round trip to the hot path at
// all. Uses context.Background() (not the request ctx) so it isn't
// cancelled the instant the RPC handler returns.
func (uc *ValidateSession) touchBestEffort(tokenHash string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = uc.sessions.TouchLastSeen(ctx, tokenHash, uc.clock.Now())
	}()
}
```

Add `"time"` to the file's imports.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestValidateSession -v
```

Expected: add test cases in `validate_session_test.go` using a synchronous fake `SessionRepository` (not a real goroutine race, for determinism) — valid session with `LastSeenAt == nil` → touch attempted; touched 10s ago (< 60s debounce) → touch NOT attempted; touched 90s ago → touch attempted; a `TouchLastSeen` failure never changes `Execute`'s returned `ValidateSessionOutput`.
