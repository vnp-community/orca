# TASK-AUTH-02-03: `SessionRepository` port gains `TouchLastSeen`/`DeleteExpiredBefore`

**From Solution:** SOL-AUTH-02
**Priority:** P0
**Service:** `auth-service` (usecase port)
**File:** `backend-go/services/auth-service/internal/usecase/ports.go`
**Depends on:** TASK-AUTH-02-02
**Status:** `[x]` DONE — added `TouchLastSeen`/`DeleteExpiredBefore` to `SessionRepository` port (build breaks downstream until 02-04, as expected).

---

## Context

The reaper (TASK-AUTH-02-07) and the debounced-touch path (TASK-AUTH-02-06) both need new persistence primitives that don't exist on `SessionRepository` today.

## Changes to make

In `backend-go/services/auth-service/internal/usecase/ports.go`, add two methods to the `SessionRepository` interface (after the existing `CountActive`):

```go
type SessionRepository interface {
	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error)
	RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error
	ListForUser(ctx context.Context, userID string) ([]domain.Session, error)
	RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error)
	CountActive(ctx context.Context, now time.Time) (int32, error)

	// TouchLastSeen updates last_seen_at for tokenHash to now — a no-op,
	// not an error, if tokenHash doesn't exist (the session may have been
	// revoked/expired between the IsValid check and this call landing;
	// ValidateSession has already returned its decision by then).
	TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error

	// DeleteExpiredBefore removes every session row whose expires_at (or
	// revoked_at) is older than cutoff, returning the count removed — the
	// reaper's primitive. Takes a cutoff, not "now": the reaper purges rows
	// expired/revoked more than a retention window ago, not the instant
	// they expire, so a brief "recently expired sessions" admin view still
	// has something to show. This is storage/observability hygiene, not a
	// security boundary — expired rows already fail domain.Session.IsValid
	// at read time regardless of whether they've been purged yet.
	DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... 2>&1 | head -50
```

Expected: this build WILL fail until the postgres adapter (TASK-AUTH-02-04) implements both new methods and any test fakes implementing `SessionRepository` are updated — that's expected at this step; confirm the compiler errors point only at missing-method implementations, not a signature typo.
