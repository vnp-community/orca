package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// PairingSessionStore implements usecase.PairingSessionRepository against
// auth.pairing_sessions.
type PairingSessionStore struct {
	pool *pgxpool.Pool
}

func NewPairingSessionStore(pool *pgxpool.Pool) *PairingSessionStore {
	return &PairingSessionStore{pool: pool}
}

func (s *PairingSessionStore) Save(ctx context.Context, session domain.PairingSession) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.pairing_sessions
			(id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		session.ID, session.TenantID, session.UserID, session.DesktopPublicKey,
		session.DesktopPrivateKeyCiphertext, session.VaultKeyRef, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("postgres: insert pairing session: %w", err)
	}
	return nil
}

// GetAndConsume atomically marks the row consumed and returns it — the
// single SQL statement that closes BR-MB-02's one-time-use race between
// two concurrent CompleteDevicePairing calls on the same token.
func (s *PairingSessionStore) GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE auth.pairing_sessions
		SET consumed_at = now()
		WHERE id = $1 AND consumed_at IS NULL
		RETURNING id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at, consumed_at`,
		id)
	var session domain.PairingSession
	err := row.Scan(&session.ID, &session.TenantID, &session.UserID, &session.DesktopPublicKey,
		&session.DesktopPrivateKeyCiphertext, &session.VaultKeyRef, &session.CreatedAt, &session.ExpiresAt, &session.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PairingSession{}, fmt.Errorf("postgres: get and consume pairing session: %w", domain.ErrPairingTokenNotFound)
	}
	if err != nil {
		return domain.PairingSession{}, fmt.Errorf("postgres: get and consume pairing session: %w", err)
	}
	return session, nil
}
