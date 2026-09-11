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

func (r *Repository) RecordIssuedToken(ctx context.Context, token domain.IssuedServiceToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO issued_service_tokens (jti, user_id, audience, issued_at, expires_at)
		VALUES (?,?,?,?,?)
	`, token.JTI, token.UserID, token.Audience, token.IssuedAt, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("mysql: insert issued service token: %w", err)
	}
	return nil
}

func (r *Repository) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var revokedAt *time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT revoked_at FROM issued_service_tokens WHERE jti = ?
	`, jti).Scan(&revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Unknown jti reports false, not an error — see
		// ServiceTokenRepository.IsRevoked's doc comment.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mysql: query issued service token: %w", err)
	}
	return revokedAt != nil, nil
}

// Revoke does NOT branch on UPDATE's RowsAffected()==0 to detect "not
// found" — same class of bug as RevokeSession (session_repository.go):
// MySQL's default RowsAffected() counts only rows whose VALUES changed,
// so a retried revoke of an already-revoked jti (same revoked_at, or any
// value collision) would incorrectly report ErrServiceTokenNotFound for a
// jti that DOES exist. CLI-token revocation is a security-relevant path
// (RevokeCliToken) — a false not-found here must not mask "the token row
// actually exists and needs revoking". Instead: UPDATE unconditionally,
// then confirm existence with a follow-up SELECT.
func (r *Repository) Revoke(ctx context.Context, jti string, revokedAt time.Time) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE issued_service_tokens SET revoked_at = ? WHERE jti = ?
	`, revokedAt, jti); err != nil {
		return fmt.Errorf("mysql: revoke issued service token: %w", err)
	}

	var exists string
	err := r.db.QueryRowContext(ctx, `SELECT jti FROM issued_service_tokens WHERE jti = ?`, jti).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: revoke issued service token: %w", usecase.ErrServiceTokenNotFound)
	}
	if err != nil {
		return fmt.Errorf("mysql: revoke issued service token: confirming token exists: %w", err)
	}
	return nil
}

func (r *Repository) ListServiceTokensForUser(ctx context.Context, userID string) ([]domain.IssuedServiceToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jti, user_id, audience, issued_at, expires_at, revoked_at
		FROM issued_service_tokens
		WHERE user_id = ?
		ORDER BY issued_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query issued service tokens for user: %w", err)
	}
	defer rows.Close()

	var out []domain.IssuedServiceToken
	for rows.Next() {
		var t domain.IssuedServiceToken
		if err := rows.Scan(&t.JTI, &t.UserID, &t.Audience, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan issued service token row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate issued service token rows: %w", err)
	}
	return out, nil
}
