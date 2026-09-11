package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// PairingSessionStore implements usecase.PairingSessionRepository against
// pairing_sessions — mirrors internal/adapter/postgres.PairingSessionStore's
// shape (its own struct/constructor, not part of Repository, matching the
// Postgres variant's split).
type PairingSessionStore struct {
	db *sql.DB
}

func NewPairingSessionStore(db *sql.DB) *PairingSessionStore {
	return &PairingSessionStore{db: db}
}

func (s *PairingSessionStore) Save(ctx context.Context, session domain.PairingSession) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pairing_sessions
			(id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		session.ID, session.TenantID, session.UserID, session.DesktopPublicKey,
		session.DesktopPrivateKeyCiphertext, session.VaultKeyRef, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("mysql: insert pairing session: %w", err)
	}
	return nil
}

// GetAndConsume atomically marks the row consumed and returns it —
// BR-MB-02's one-time-use enforcement. Unlike UpdateUserRole/RevokeSession
// elsewhere in this package, this DOES safely use RowsAffected()==0 as its
// not-found/already-consumed signal: consumed_at only ever transitions
// NULL -> non-NULL, exactly once (enforced by the `WHERE consumed_at IS
// NULL` guard), so there is no idempotent-no-op case where the column
// value would be unchanged — MySQL's "rows changed" RowsAffected semantics
// and Postgres's "rows matched" semantics agree on every call here. This
// also reproduces the Postgres variant's collapsed error case exactly:
// RowsAffected()==0 means EITHER id doesn't exist OR it was already
// consumed — same as pgx.ErrNoRows on the Postgres RETURNING statement,
// not a behavior change.
func (s *PairingSessionStore) GetAndConsume(ctx context.Context, id string) (domain.PairingSession, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE pairing_sessions
		SET consumed_at = NOW(6)
		WHERE id = ? AND consumed_at IS NULL`, id)
	if err != nil {
		return domain.PairingSession{}, fmt.Errorf("mysql: get and consume pairing session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return domain.PairingSession{}, fmt.Errorf("mysql: get and consume pairing session: rows affected: %w", err)
	}
	if n == 0 {
		return domain.PairingSession{}, fmt.Errorf("mysql: get and consume pairing session: %w", domain.ErrPairingTokenNotFound)
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, user_id, desktop_public_key, desktop_private_key_ciphertext, vault_key_ref, created_at, expires_at, consumed_at
		FROM pairing_sessions WHERE id = ?`, id)
	var session domain.PairingSession
	err = row.Scan(&session.ID, &session.TenantID, &session.UserID, &session.DesktopPublicKey,
		&session.DesktopPrivateKeyCiphertext, &session.VaultKeyRef, &session.CreatedAt, &session.ExpiresAt, &session.ConsumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// Can't happen in practice (we just updated this row), but handled
		// for completeness rather than assumed away.
		return domain.PairingSession{}, fmt.Errorf("mysql: get and consume pairing session: %w", domain.ErrPairingTokenNotFound)
	}
	if err != nil {
		return domain.PairingSession{}, fmt.Errorf("mysql: get and consume pairing session: re-reading row: %w", err)
	}
	return session, nil
}
