# TASK-AUTH-02-07: `ReapExpiredSessions` usecase + hourly ticker in `main.go`

**From Solution:** SOL-AUTH-02
**Priority:** P1
**Service:** `auth-service` (usecase + cmd/server)
**File:** `backend-go/services/auth-service/internal/usecase/reap_expired_sessions.go` (new), `backend-go/services/auth-service/cmd/server/main.go`
**Depends on:** TASK-AUTH-02-03, TASK-AUTH-02-04
**Status:** `[x]` DONE — added `ReapExpiredSessions` usecase + hourly ticker in `main.go` (7-day retention); `reap_expired_sessions_test.go` (cutoff computation, error propagation) passes.

---

## Context

`auth-service.md` §8 explicitly calls for a "Session/refresh-token reaper: a background job expiring rows past `expires_at`... not a correctness requirement (expired rows already fail validation)... but an operational one" bounding table growth. Nothing implements this today. `domain.Session.IsValid` already enforces expiry at read time, so the reaper can run on a coarse interval and tolerate being briefly behind without any security consequence.

## Changes to make

Create `backend-go/services/auth-service/internal/usecase/reap_expired_sessions.go`:

```go
package usecase

import (
	"context"
	"time"
)

// ReapExpiredSessions is a background-job usecase — purges session rows
// expired/revoked more than retention ago. Not a correctness mechanism
// (domain.Session.IsValid already enforces expiry at read time); this only
// bounds table growth, per auth-service.md §8's reaper NFR.
type ReapExpiredSessions struct {
	sessions  SessionRepository
	clock     Clock
	retention time.Duration
}

func NewReapExpiredSessions(sessions SessionRepository, clock Clock, retention time.Duration) *ReapExpiredSessions {
	return &ReapExpiredSessions{sessions: sessions, clock: clock, retention: retention}
}

func (uc *ReapExpiredSessions) Execute(ctx context.Context) (int64, error) {
	cutoff := uc.clock.Now().Add(-uc.retention)
	return uc.sessions.DeleteExpiredBefore(ctx, cutoff)
}
```

In `backend-go/services/auth-service/cmd/server/main.go`, alongside existing server startup, start the reaper ticker:

```go
reaper := usecase.NewReapExpiredSessions(sessionRepo, usecase.SystemClock{}, 7*24*time.Hour)
go func() {
	ticker := time.NewTicker(1 * time.Hour) // frequent enough per the reaper's "operational, not correctness" NFR
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := reaper.Execute(ctx); err != nil {
				log.Error("session reaper failed", "err", err)
			} else if n > 0 {
				log.Info("session reaper purged rows", "count", n)
			}
		}
	}
}()
```

Adjust variable names (`sessionRepo`, `ctx`, `log`) to match whatever `main.go` already calls its constructed dependencies — do not introduce a second logger/context construction if one already exists in scope.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/usecase/... -run TestReapExpiredSessions -v
```

Expected: `reap_expired_sessions_test.go` — fake clock + fake repo: `Execute` computes `cutoff = now - retention` correctly and returns the repo's count unchanged; a repo error propagates (the ticker loop in `main.go` is responsible for not crashing on it, not this usecase).
