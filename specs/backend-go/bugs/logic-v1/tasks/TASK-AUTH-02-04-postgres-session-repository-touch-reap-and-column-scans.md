# TASK-AUTH-02-04: Implement `TouchLastSeen`/`DeleteExpiredBefore`, extend existing session scans

**From Solution:** SOL-AUTH-02
**Priority:** P0
**Service:** `auth-service` (postgres adapter)
**File:** `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go`
**Depends on:** TASK-AUTH-02-01, TASK-AUTH-02-03
**Status:** `[x]` DONE — implemented `TouchLastSeen`/`DeleteExpiredBefore`, extended scans; fixed spec's `ip::text` (which prints `/32` for inet — verified against real Postgres) to `host(ip)`; integration tests (`-tags=integration`, testcontainers) pass against real Postgres 16 — `TestSessionRepository_RoundTripsClientInfo/TouchLastSeen/DeleteExpiredBefore`.

---

## Context

Implements the two new `SessionRepository` port methods and extends `CreateSession`/`GetSessionByTokenHash`/`ListForUser`'s `INSERT`/`SELECT`/`Scan` to carry `last_seen_at`/`ip`/`user_agent`, using nullable scanning since both may be `NULL` for pre-migration rows or an unresolved IP.

## Changes to make

In `backend-go/services/auth-service/internal/adapter/postgres/session_repository.go`:

```go
func (r *Repository) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.sessions (token_hash, user_id, tenant_id, created_at, expires_at, revoked_at, last_seen_at, ip, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, session.TokenHash, session.UserID, session.TenantID, session.CreatedAt, session.ExpiresAt, session.RevokedAt,
		session.LastSeenAt, nullIfEmpty(session.IP), nullIfEmpty(session.UserAgent))
	if err != nil {
		return fmt.Errorf("postgres: insert session: %w", err)
	}
	return nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		       last_seen_at, COALESCE(ip::text, ''), COALESCE(user_agent, '')
		FROM auth.sessions
		WHERE token_hash = $1
	`, tokenHash)

	var s domain.Session
	err := row.Scan(&s.TokenHash, &s.UserID, &s.TenantID, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt,
		&s.LastSeenAt, &s.IP, &s.UserAgent)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("postgres: query session: %w", usecase.ErrSessionNotFound)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("postgres: scan session row: %w", err)
	}
	return s, nil
}

// ListForUser: SELECT/Scan gains the same three columns as
// GetSessionByTokenHash above, same COALESCE-to-empty-string pattern for ip/user_agent.

// TouchLastSeen updates last_seen_at for tokenHash to now — 0 rows affected
// is not an error (see interface doc comment).
func (r *Repository) TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE auth.sessions SET last_seen_at = $2 WHERE token_hash = $1
	`, tokenHash, now)
	if err != nil {
		return fmt.Errorf("postgres: touch session last_seen_at: %w", err)
	}
	return nil
}

// DeleteExpiredBefore removes rows expired/revoked before cutoff, returns
// the count removed.
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

Add a small `nullIfEmpty(s string) any` helper in this package if one doesn't already exist elsewhere in `postgres/` (returns `nil` for `""`, else `s`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/...
go test ./services/auth-service/internal/adapter/postgres/... -run TestSessionRepository -v
```

Expected: `TouchLastSeen` on an existing token updates `last_seen_at`; on a nonexistent token is a no-op, no error; `DeleteExpiredBefore` removes only rows with `expires_at`/`revoked_at` before the cutoff, leaves active sessions untouched; `CreateSession`/`GetSessionByTokenHash`/`ListForUser` round-trip `IP`/`UserAgent`/`LastSeenAt` including the `NULL` case (run against a real/test Postgres instance — these are integration tests).
