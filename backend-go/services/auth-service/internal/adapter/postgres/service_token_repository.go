package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

func (r *Repository) RecordIssuedToken(ctx context.Context, token domain.IssuedServiceToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth.issued_service_tokens (jti, user_id, audience, issued_at, expires_at)
		VALUES ($1,$2,$3,$4,$5)
	`, token.JTI, token.UserID, token.Audience, token.IssuedAt, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("postgres: insert issued service token: %w", err)
	}
	return nil
}

func (r *Repository) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var revokedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT revoked_at FROM auth.issued_service_tokens WHERE jti = $1
	`, jti).Scan(&revokedAt)
	if err == pgx.ErrNoRows {
		// Unknown jti (e.g. a JWT minted before this table existed) reports
		// false, not an error — see ServiceTokenRepository.IsRevoked's doc
		// comment.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("postgres: query issued service token: %w", err)
	}
	return revokedAt != nil, nil
}

func (r *Repository) Revoke(ctx context.Context, jti string, revokedAt time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE auth.issued_service_tokens SET revoked_at = $2 WHERE jti = $1
	`, jti, revokedAt)
	if err != nil {
		return fmt.Errorf("postgres: revoke issued service token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: revoke issued service token: %w", usecase.ErrServiceTokenNotFound)
	}
	return nil
}

func (r *Repository) ListServiceTokensForUser(ctx context.Context, userID string) ([]domain.IssuedServiceToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT jti, user_id, audience, issued_at, expires_at, revoked_at
		FROM auth.issued_service_tokens
		WHERE user_id = $1
		ORDER BY issued_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query issued service tokens for user: %w", err)
	}
	defer rows.Close()

	var out []domain.IssuedServiceToken
	for rows.Next() {
		var t domain.IssuedServiceToken
		if err := rows.Scan(&t.JTI, &t.UserID, &t.Audience, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan issued service token row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate issued service token rows: %w", err)
	}
	return out, nil
}
