package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

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
		       last_seen_at, COALESCE(host(ip), ''), COALESCE(user_agent, '')
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

func (r *Repository) RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.sessions SET revoked_at = $2
		WHERE token_hash = $1
	`, tokenHash, revokedAt)
	if err != nil {
		return fmt.Errorf("postgres: revoke session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: revoke session: %w", usecase.ErrSessionNotFound)
	}
	return nil
}

// ListForUser returns every session (revoked or not, expired or not) for
// userID — the admin-console session-inspection view needs the full
// history, not just currently-valid sessions.
func (r *Repository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		       last_seen_at, COALESCE(host(ip), ''), COALESCE(user_agent, '')
		FROM auth.sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query sessions for user: %w", err)
	}
	defer rows.Close()

	var out []domain.Session
	for rows.Next() {
		var s domain.Session
		if err := rows.Scan(&s.TokenHash, &s.UserID, &s.TenantID, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt,
			&s.LastSeenAt, &s.IP, &s.UserAgent); err != nil {
			return nil, fmt.Errorf("postgres: scan session row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate session rows: %w", err)
	}
	return out, nil
}

// RevokeAllForUser force-revokes every currently-unrevoked session for
// userID and returns how many were revoked.
func (r *Repository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.sessions SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, revokedAt)
	if err != nil {
		return 0, fmt.Errorf("postgres: revoke all sessions for user: %w", err)
	}
	return int32(tag.RowsAffected()), nil
}

// CountActive returns the number of currently-valid (unrevoked, unexpired)
// sessions across every tenant, as of now.
func (r *Repository) CountActive(ctx context.Context, now time.Time) (int32, error) {
	var n int32
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM auth.sessions
		WHERE revoked_at IS NULL AND expires_at > $1
	`, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("postgres: count active sessions: %w", err)
	}
	return n, nil
}

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

// ListForTenant returns a page of sessions for tenantID joined with each
// session's owning user's email (denormalized to avoid an N+1 lookup in
// the admin dashboard).
func (r *Repository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.token_hash, s.user_id, s.tenant_id, s.created_at, s.expires_at,
		       s.revoked_at, s.last_seen_at, COALESCE(host(s.ip), ''), COALESCE(s.user_agent, ''), u.email
		FROM auth.sessions s
		JOIN auth.users u ON u.id = s.user_id
		WHERE s.tenant_id = $1 AND s.token_hash > $2
		ORDER BY s.token_hash
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query sessions for tenant: %w", err)
	}
	defer rows.Close()

	var out []domain.SessionWithUser
	for rows.Next() {
		var sw domain.SessionWithUser
		if err := rows.Scan(&sw.Session.TokenHash, &sw.Session.UserID, &sw.Session.TenantID,
			&sw.Session.CreatedAt, &sw.Session.ExpiresAt, &sw.Session.RevokedAt,
			&sw.Session.LastSeenAt, &sw.Session.IP, &sw.Session.UserAgent, &sw.UserEmail); err != nil {
			return nil, "", fmt.Errorf("postgres: scan session-with-user row: %w", err)
		}
		out = append(out, sw)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate session-with-user rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].Session.TokenHash
	}
	return out, next, nil
}

// nullIfEmpty returns nil for an empty string (so an empty/unresolved
// IP/UserAgent stores as SQL NULL rather than an empty string or, for the
// INET column, a value that fails to parse), else the string unchanged.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
