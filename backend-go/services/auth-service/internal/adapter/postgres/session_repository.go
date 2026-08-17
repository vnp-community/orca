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
		INSERT INTO auth.sessions (token_hash, user_id, tenant_id, created_at, expires_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, session.TokenHash, session.UserID, session.TenantID, session.CreatedAt, session.ExpiresAt, session.RevokedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert session: %w", err)
	}
	return nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT token_hash, user_id, tenant_id, created_at, expires_at, revoked_at
		FROM auth.sessions
		WHERE token_hash = $1
	`, tokenHash)

	var s domain.Session
	err := row.Scan(&s.TokenHash, &s.UserID, &s.TenantID, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt)
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
