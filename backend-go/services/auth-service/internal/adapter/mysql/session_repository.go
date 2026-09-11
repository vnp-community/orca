package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// nullableString/nullableTime mirror postgres/session_repository.go's
// identical helpers — an unset refresh_token_hash/RefreshExpiresAt stores
// as SQL NULL, not the zero value, matching domain.Session's own
// empty-means-unset convention.
func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (r *Repository) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		                       last_seen_at, ip, user_agent, refresh_token_hash, refresh_expires_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
	`, session.TokenHash, session.UserID, session.TenantID, session.CreatedAt, session.ExpiresAt, session.RevokedAt,
		session.LastSeenAt, nullIfEmpty(session.IP), nullIfEmpty(session.UserAgent),
		nullableString(session.RefreshTokenHash), nullableTime(session.RefreshExpiresAt))
	if err != nil {
		return fmt.Errorf("mysql: insert session: %w", err)
	}
	return nil
}

// scanSession mirrors postgres/session_repository.go's scanSession — same
// 11-column shape and order, ip/user_agent read directly (no host()-style
// unwrap needed: ip is stored as plain VARCHAR text here, see
// migrations/mysql/0008_sessions_last_seen_ip_ua.up.sql's comment).
func scanSession(row rowScanner) (domain.Session, error) {
	var s domain.Session
	var ip, userAgent *string
	var refreshHash *string
	var refreshExpiresAt *time.Time
	err := row.Scan(&s.TokenHash, &s.UserID, &s.TenantID, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt,
		&s.LastSeenAt, &ip, &userAgent, &refreshHash, &refreshExpiresAt)
	if err != nil {
		return domain.Session{}, err
	}
	if ip != nil {
		s.IP = *ip
	}
	if userAgent != nil {
		s.UserAgent = *userAgent
	}
	if refreshHash != nil {
		s.RefreshTokenHash = *refreshHash
	}
	if refreshExpiresAt != nil {
		s.RefreshExpiresAt = *refreshExpiresAt
	}
	return s, nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		       last_seen_at, ip, user_agent, refresh_token_hash, refresh_expires_at
		FROM sessions
		WHERE token_hash = ?
	`, tokenHash)

	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("mysql: query session: %w", usecase.ErrSessionNotFound)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("mysql: scan session row: %w", err)
	}
	return s, nil
}

func (r *Repository) GetSessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (domain.Session, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		       last_seen_at, ip, user_agent, refresh_token_hash, refresh_expires_at
		FROM sessions
		WHERE refresh_token_hash = ?
	`, refreshTokenHash)

	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, fmt.Errorf("mysql: query session by refresh token: %w", usecase.ErrSessionNotFound)
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("mysql: scan session row: %w", err)
	}
	return s, nil
}

// RevokeSession does NOT branch on UPDATE's RowsAffected()==0 to detect
// "not found" — MySQL's default RowsAffected() counts only rows whose
// VALUES changed, not rows matched by WHERE. A retried revoke of an
// already-revoked session (same revoked_at value, or any value collision)
// would hit RowsAffected()==0 under a naive translation and incorrectly
// report ErrSessionNotFound for a session that unambiguously DOES exist —
// a false negative on a security-relevant revocation path (see this
// rollout's brief: token revocation/session invalidation false negatives
// are a security concern, not just a correctness nit). Instead: UPDATE
// unconditionally, then confirm existence with a follow-up SELECT —
// mirrors UpdateUserRole's identical fix in user_repository.go.
func (r *Repository) RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?
		WHERE token_hash = ?
	`, revokedAt, tokenHash); err != nil {
		return fmt.Errorf("mysql: revoke session: %w", err)
	}

	var exists string
	err := r.db.QueryRowContext(ctx, `SELECT token_hash FROM sessions WHERE token_hash = ?`, tokenHash).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: revoke session: %w", usecase.ErrSessionNotFound)
	}
	if err != nil {
		return fmt.Errorf("mysql: revoke session: confirming session exists: %w", err)
	}
	return nil
}

func (r *Repository) ListForUser(ctx context.Context, userID string) ([]domain.Session, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at,
		       last_seen_at, ip, user_agent, refresh_token_hash, refresh_expires_at
		FROM sessions
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query sessions for user: %w", err)
	}
	defer rows.Close()

	var out []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan session row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate session rows: %w", err)
	}
	return out, nil
}

// RevokeAllForUser force-revokes every currently-unrevoked session for
// userID and returns how many were revoked. No not-found detection here in
// either dialect (0 sessions revoked is a valid, non-error outcome), so no
// RowsAffected ambiguity to translate — RowsAffected's count itself is
// still meaningful here (mass revoke's own return value), not used as a
// not-found signal.
func (r *Repository) RevokeAllForUser(ctx context.Context, userID string, revokedAt time.Time) (int32, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?
		WHERE user_id = ? AND revoked_at IS NULL
	`, revokedAt, userID)
	if err != nil {
		return 0, fmt.Errorf("mysql: revoke all sessions for user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mysql: revoke all sessions for user: rows affected: %w", err)
	}
	return int32(n), nil
}

func (r *Repository) CountActive(ctx context.Context, now time.Time) (int32, error) {
	var n int32
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sessions
		WHERE revoked_at IS NULL AND expires_at > ?
	`, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("mysql: count active sessions: %w", err)
	}
	return n, nil
}

// TouchLastSeen — 0 rows affected is not an error (see interface doc
// comment); no RowsAffected branch exists here in either dialect.
func (r *Repository) TouchLastSeen(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?
	`, now, tokenHash)
	if err != nil {
		return fmt.Errorf("mysql: touch session last_seen_at: %w", err)
	}
	return nil
}

// DeleteExpiredBefore removes rows expired/revoked before cutoff, returns
// the count removed. MySQL's DELETE RowsAffected() always counts matched
// rows (there is no "value changed" ambiguity for DELETE, only UPDATE), so
// this is a direct translation — no RowsAffected pitfall here, matching
// annotation-service's BE-DB-SOL-005 §4's identical DeleteAnnotation
// finding.
func (r *Repository) DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions WHERE expires_at < ? OR revoked_at < ?
	`, cutoff, cutoff)
	if err != nil {
		return 0, fmt.Errorf("mysql: delete expired sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("mysql: delete expired sessions: rows affected: %w", err)
	}
	return n, nil
}

// ListForTenant returns a page of sessions for tenantID joined with each
// session's owning user's email.
func (r *Repository) ListForTenant(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.SessionWithUser, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.token_hash, s.user_id, s.tenant_id, s.created_at, s.expires_at,
		       s.revoked_at, s.last_seen_at, s.ip, s.user_agent, u.email
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.tenant_id = ? AND s.token_hash > ?
		ORDER BY s.token_hash
		LIMIT ?
	`, tenantID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query sessions for tenant: %w", err)
	}
	defer rows.Close()

	var out []domain.SessionWithUser
	for rows.Next() {
		var sw domain.SessionWithUser
		var ip, userAgent *string
		if err := rows.Scan(&sw.Session.TokenHash, &sw.Session.UserID, &sw.Session.TenantID,
			&sw.Session.CreatedAt, &sw.Session.ExpiresAt, &sw.Session.RevokedAt,
			&sw.Session.LastSeenAt, &ip, &userAgent, &sw.UserEmail); err != nil {
			return nil, "", fmt.Errorf("mysql: scan session-with-user row: %w", err)
		}
		if ip != nil {
			sw.Session.IP = *ip
		}
		if userAgent != nil {
			sw.Session.UserAgent = *userAgent
		}
		out = append(out, sw)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate session-with-user rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].Session.TokenHash
	}
	return out, next, nil
}

// nullIfEmpty returns nil for an empty string, else the string unchanged —
// mirrors postgres/session_repository.go's identical helper (there, it
// also avoids a Postgres INET parse failure; here, ip is a plain VARCHAR
// so this purely controls NULL-vs-empty-string storage).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
