# SOL-AUTH-02: Sliding `last_seen_at` RENEW + background EXPIRE reaper for sessions

**Resolves:** [BUG-AUTH-02](../BUG-AUTH-02-session-lifecycle-partial.md)
**Service:** `auth-service`
**Affected files (proposed):**
- `backend-go/services/auth-service/migrations/000X_sessions_last_seen_ip_ua.up.sql` (+ `.down.sql`)
- `backend-go/proto/orca/auth/v1/auth.proto` — `LoginRequest` already gains `ip`/`user_agent` per [SOL-AUTH-01](./SOL-AUTH-01-local-login-error-mapping-rate-limit.md); this solution is what actually persists them
- `backend-go/services/auth-service/internal/domain/session.go` — `Session` gains `LastSeenAt`, `IP`, `UserAgent`
- `backend-go/services/auth-service/internal/usecase/ports.go` — `SessionRepository` gains `TouchLastSeen`, `DeleteExpiredBefore`
- `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go` — implement the new methods, extend existing scans
- `backend-go/services/auth-service/internal/usecase/login.go` — pass `IP`/`UserAgent` into `NewSession`
- `backend-go/services/auth-service/internal/usecase/validate_session.go` — best-effort touch on every valid read
- `backend-go/services/auth-service/internal/usecase/reap_expired_sessions.go` (new usecase)
- `backend-go/services/auth-service/cmd/server/main.go` — start the reaper ticker
- `backend-go/services/auth-service/internal/adapter/grpc/server.go` — `toProtoSession` populates the proto's already-declared `last_seen_at`/`ip`/`user_agent` fields
- `backend-go/services/auth-service/internal/domain/session_test.go`, `internal/usecase/validate_session_test.go`, `internal/usecase/reap_expired_sessions_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

- **RENEW and the reaper are both explicitly specified, not inferred.** `auth-service.md` §4's `Session` domain model already lists `lastSeenAt`, `ip`, `userAgent` as invariant fields (`auth-service.md:138-142`: "**`Session`** — `id`..., `lastSeenAt`, `ip`, `userAgent`. Invariant: `expiresAt` is always set at creation... the TS system's idle-timeout logic is preserved as an absolute TTL plus sliding `lastSeenAt`-based extension"), and §5's `sessions` table already lists `last_seen_at`, `ip inet`, `user_agent TEXT` as columns (`auth-service.md:170`). §8's NFRs separately mandate "**Session/refresh-token reaper**: a background job expiring rows past `expires_at`... not a correctness requirement... but an operational one" (`auth-service.md:262-265`). Both gaps this bug reports are the codebase simply not having built what its own target doc already specifies — this is a pure implementation gap, not a design decision to make.
- **The proto already has the fields; only the domain/DB layer is missing them.** `authv1.Session` (`backend-go/proto/gen/go/orca/auth/v1/auth.pb.go:1503-1514`) already declares `LastSeenAt`, `Ip`, `UserAgent` — confirming BUG-AUTH-02's finding that these are "simply never populated" rather than absent from the contract. This solution closes the gap from the domain/DB layer outward; no proto change is needed for `Session` itself (only `LoginRequest`, already covered by SOL-AUTH-01).
- **Touching `last_seen_at` on every `ValidateSession` call must not violate the p99 < 20ms SLO.** `auth-service.md` §8 states `ValidateSession` "is the one RPC on literally every browser request's path" with p99 < 20ms, cached at the gateway for a few seconds (`auth-service.md:245-249`). A synchronous `UPDATE` on every call would add a write to the hottest read path in the system. This solution follows the same "best-effort side effect, never blocks the real decision" shape `login.go`'s `appendAuditBestEffort` already establishes (`login.go:89-92`'s doc comment, itself citing `usage-service`'s `RecordUsageSession` precedent) — fire off the touch asynchronously and debounce it, rather than writing on every call.
- **EXPIRE stays a cleanup job, not a correctness mechanism** — per `auth-service.md:263-265` explicitly ("not a correctness requirement... expired rows already fail validation... an operational one"). `domain.Session.IsValid` (`session.go:65-72`) already enforces expiry at read time; the reaper only bounds table growth, so it can run on a coarse interval and tolerate being briefly behind without any security consequence.

## Design — schema

```sql
-- 000X_sessions_last_seen_ip_ua.up.sql
ALTER TABLE auth.sessions
  ADD COLUMN last_seen_at TIMESTAMPTZ,
  ADD COLUMN ip           INET,
  ADD COLUMN user_agent   TEXT;

-- Index the reaper's scan predicate — mirrors the existing
-- idx_sessions_expires_at pattern auth-service.md:170 already documents.
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON auth.sessions (expires_at);
```

`last_seen_at` starts `NULL` (never touched since creation — distinct from
"touched at creation," matching the TS reference implementation's
`orca_sessions.last_seen_at` shape at `desktop/src/main/auth/auth-session-store.ts:38`,
which this codebase is the Go replacement for).

## Design — `domain/session.go`

```go
type Session struct {
    TokenHash  string
    UserID     string
    TenantID   string
    CreatedAt  time.Time
    ExpiresAt  time.Time
    RevokedAt  *time.Time
    LastSeenAt *time.Time // nil until first touch — see ValidateSession below
    IP         string     // may be "" for a session created before this migration, or if unresolved
    UserAgent  string
}

// NewSession keeps its existing five required params (session.go:40) —
// IP/UserAgent are optional, set via WithClientInfo rather than growing the
// constructor's required-argument list, since they're metadata, not an
// invariant NewSession's error returns (ErrEmptyTokenHash etc.) need to
// enforce.
func (s Session) WithClientInfo(ip, userAgent string) Session {
    s.IP, s.UserAgent = ip, userAgent
    return s
}
```

`Login.Execute` becomes:

```go
session, err := domain.NewSession(domain.HashSessionToken(rawToken), user.ID, user.TenantID, now, now.Add(uc.sessionTTL))
if err != nil { /* unchanged */ }
session = session.WithClientInfo(in.IP, in.UserAgent) // in.IP/in.UserAgent from SOL-AUTH-01
if err := uc.sessions.CreateSession(ctx, session); err != nil { /* unchanged */ }
```

## Design — `usecase/ports.go` (`SessionRepository` additions)

```go
type SessionRepository interface {
    // ... existing methods unchanged (session_repository.go / ports.go:72-90)

    // TouchLastSeen updates last_seen_at for tokenHash to now — a no-op,
    // not an error, if tokenHash doesn't exist (the session may have been
    // revoked/expired between the IsValid check and this call landing;
    // ValidateSession has already returned its decision by then, see below).
    TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error

    // DeleteExpiredBefore removes every session row whose expires_at (or
    // revoked_at) is older than cutoff, returning the count removed — the
    // reaper's primitive. Deliberately takes a cutoff, not "now": the
    // reaper purges rows expired/revoked more than a retention window ago
    // (e.g. 7 days), not the instant they expire, so a brief admin-console
    // "recently expired sessions" view (if ever built) still has something
    // to show — auth-service.md:263-265 frames this as storage/observability
    // hygiene, not a security boundary.
    DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
```

## Design — `postgres/session_repository.go`

```go
func (r *Repository) TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE auth.sessions SET last_seen_at = $2 WHERE token_hash = $1
    `, tokenHash, now)
    if err != nil {
        return fmt.Errorf("postgres: touch session last_seen_at: %w", err)
    }
    return nil // 0 rows affected is not an error — see interface doc comment
}

func (r *Repository) DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error) {
    tag, err := r.pool.Exec(ctx, `
        DELETE FROM auth.sessions WHERE expires_at < $1 OR revoked_at < $1
    `, cutoff)
    if err != nil {
        return 0, fmt.Errorf("postgres: delete expired sessions: %w", err)
    }
    return tag.RowsAffected(), nil
}
```

`CreateSession`, `GetSessionByTokenHash`, and `ListForUser` (`session_repository.go:15-24,26-42,61-85`)
each extend their `SELECT`/`INSERT` column lists and `Scan` targets with
`last_seen_at, ip, user_agent`, using `pgtype`-nullable scanning for
`last_seen_at`/`ip` (both may be `NULL` for pre-migration rows or a session
whose IP wasn't resolved).

## Design — `usecase/validate_session.go` (debounced async touch)

```go
const touchDebounce = 60 * time.Second // avoid a write on every single request

func (uc *ValidateSession) Execute(ctx context.Context, sessionToken string) (ValidateSessionOutput, error) {
    // ... unchanged through the IsValid/IsActive checks (validate_session.go:31-51)

    if session.LastSeenAt == nil || uc.clock.Now().Sub(*session.LastSeenAt) > touchDebounce {
        uc.touchBestEffort(session.TokenHash) // fire-and-forget, does not block the response
    }
    return ValidateSessionOutput{Valid: true, User: user}, nil
}

// touchBestEffort mirrors login.go's appendAuditBestEffort pattern
// (login.go:89-92) applied to a write instead of an audit append — a failed
// or slow touch must never turn a valid session into a failed request, and
// per auth-service.md §8's p99<20ms budget, must not add a synchronous
// round trip to the hot path at all. Uses context.Background() (not the
// request ctx) so it isn't cancelled the instant the RPC handler returns.
func (uc *ValidateSession) touchBestEffort(tokenHash string) {
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        _ = uc.sessions.TouchLastSeen(ctx, tokenHash, uc.clock.Now())
    }()
}
```

The 60s debounce bounds write volume to at most one `UPDATE` per session per
minute regardless of request rate, keeping the reaper's target table's
write load proportional to active-session count, not request count.

## Design — reaper usecase + wiring

```go
// internal/usecase/reap_expired_sessions.go
type ReapExpiredSessions struct {
    sessions SessionRepository
    clock    Clock
    retention time.Duration // e.g. 7 * 24 * time.Hour, configurable
}

func (uc *ReapExpiredSessions) Execute(ctx context.Context) (int64, error) {
    cutoff := uc.clock.Now().Add(-uc.retention)
    return uc.sessions.DeleteExpiredBefore(ctx, cutoff)
}
```

```go
// cmd/server/main.go — alongside existing server startup
reaper := usecase.NewReapExpiredSessions(sessionRepo, usecase.SystemClock{}, 7*24*time.Hour)
go func() {
    ticker := time.NewTicker(1 * time.Hour) // frequent enough per auth-service.md:263-265's "not a correctness requirement... an operational one"
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

## Design — `adapter/grpc/server.go` (`toProtoSession`)

```go
func toProtoSession(s domain.Session) *authv1.Session {
    out := &authv1.Session{
        Id: s.TokenHash, UserId: s.UserID,
        CreatedAt: timestamppb.New(s.CreatedAt), ExpiresAt: timestamppb.New(s.ExpiresAt),
        Ip: s.IP, UserAgent: s.UserAgent,
    }
    if s.LastSeenAt != nil {
        out.LastSeenAt = timestamppb.New(*s.LastSeenAt)
    }
    return out
}
```

This is the change that makes `ListSessionsForUser` (the admin-console
session view BUG-AUTH-02 calls out as currently unable to show a real
`last_seen_at`) finally populate that field.

## Not addressed here — per-user process routing

The spec's WS-routing step (`WsSessionRouter.route(userId, ws)` → fork/reuse
a per-user child process) is out of scope for this solution — it depends
entirely on the mechanism [BUG-AUTH-03](../BUG-AUTH-03-per-user-sandbox-not-implemented.md)
covers, resolved by [SOL-AUTH-03](./SOL-AUTH-03-per-user-sandbox-architecture-update.md)
recommending a target-doc update rather than an implementation. Nothing in
this solution attempts a WS→process routing step.

## Test plan

- `domain/session_test.go`: `WithClientInfo` sets `IP`/`UserAgent` without disturbing existing invariant checks; zero-value `Session` still round-trips through `NewSession`'s existing test table.
- `postgres/session_repository_test.go` (or `repository_test.go`): `TouchLastSeen` on an existing token updates `last_seen_at`; on a nonexistent token is a no-op, no error; `DeleteExpiredBefore` removes only rows with `expires_at`/`revoked_at` before the cutoff, leaves active sessions untouched; `CreateSession`/`GetSessionByTokenHash`/`ListForUser` round-trip `IP`/`UserAgent`/`LastSeenAt` including the `NULL` case.
- `validate_session_test.go`: valid session with `LastSeenAt == nil` → touch is attempted (fake `SessionRepository` records a `TouchLastSeen` call, using a synchronous fake — not a real goroutine race — for determinism); valid session touched 10s ago (< 60s debounce) → touch NOT attempted; valid session touched 90s ago → touch attempted; a `TouchLastSeen` failure never changes `Execute`'s returned `ValidateSessionOutput` (best-effort, doesn't propagate).
- `reap_expired_sessions_test.go`: fake clock + fake repo — `Execute` computes `cutoff = now - retention` correctly and returns the repo's count unchanged; a repo error propagates (the ticker loop in `main.go` is responsible for not crashing on it, not this usecase).
- Integration: a fresh session's `ListSessionsForUser` response has `last_seen_at` unset until `ValidateSession` is called at least once, then populated after the debounce window — end-to-end regression guard for the exact gap BUG-AUTH-02 reports.

## References

- `backend-go/services/auth-service/internal/domain/session.go:1-81` — current `Session`/`NewSession`/`IsValid`
- `backend-go/services/auth-service/internal/usecase/ports.go:69-90` — current `SessionRepository`
- `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go:1-112` — current repository implementation
- `backend-go/services/auth-service/internal/usecase/validate_session.go:1-54` — current `ValidateSession.Execute`
- `backend-go/services/auth-service/internal/usecase/login.go:52-99` — current `Login.Execute`, `appendAuditBestEffort`'s best-effort pattern this reuses
- `backend-go/proto/gen/go/orca/auth/v1/auth.pb.go:1503-1514` — `Session` proto message, fields already declared but unpopulated
- `specs/backend-go/tdd/services/auth-service.md:138-142` (§4 `Session` domain model), `:170` (§5 `sessions` table columns), `:245-249` (§8 `ValidateSession` p99 budget), `:262-265` (§8 reaper NFR)
- `specs/backend-go/bugs/logic-v1/BUG-AUTH-03-per-user-sandbox-not-implemented.md` — the WS-routing gap this solution deliberately does not attempt
