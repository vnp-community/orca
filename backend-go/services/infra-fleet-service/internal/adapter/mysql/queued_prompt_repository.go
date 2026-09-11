package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// QueuedPromptStore implements usecase.QueuedPromptRepository against
// queued_prompts (migrations/mysql/0029).
type QueuedPromptStore struct{ db *sql.DB }

func NewQueuedPromptStore(db *sql.DB) *QueuedPromptStore {
	return &QueuedPromptStore{db: db}
}

// Get fetches the queued prompt for ptyID — found=false (not an error) when
// no row matches, per usecase.QueuedPromptRepository's doc comment.
func (s *QueuedPromptStore) Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at
		FROM queued_prompts WHERE pty_id = ?`, ptyID)
	p, err := scanQueuedPrompt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	if err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("mysql: query queued prompt: %w", err)
	}
	return p, true, nil
}

// Upsert inserts or replaces the single queued-prompt row for p.PtyID —
// ON DUPLICATE KEY UPDATE is MySQL's ON CONFLICT DO UPDATE equivalent (no
// ::uuid cast needed for dispatched_by_device_id, CHAR(36) accepts a plain
// string).
func (s *QueuedPromptStore) Upsert(ctx context.Context, p domain.QueuedPrompt) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO queued_prompts (pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?)
		ON DUPLICATE KEY UPDATE
			tenant_id = VALUES(tenant_id),
			prompt = VALUES(prompt),
			dispatched_by_device_id = VALUES(dispatched_by_device_id),
			queued_at = VALUES(queued_at)`,
		p.PtyID, p.TenantID, p.Prompt, p.DispatchedByDeviceID, p.QueuedAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert queued prompt: %w", err)
	}
	return nil
}

// Delete removes the queued-prompt row for ptyID, if any — idempotent: no
// matching row is not an error.
func (s *QueuedPromptStore) Delete(ctx context.Context, ptyID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM queued_prompts WHERE pty_id = ?`, ptyID)
	if err != nil {
		return fmt.Errorf("mysql: delete queued prompt: %w", err)
	}
	return nil
}

// GetAndDelete atomically reads and removes the row for ptyID — the
// regression guard against a double-delivery race between the queue-drain
// hook and a concurrent DispatchPrompt call (see
// usecase.QueuedPromptRepository's doc comment). MySQL has no DELETE ...
// RETURNING — SELECT ... FOR UPDATE inside a transaction gets the same
// atomicity instead: the row lock blocks a concurrent GetAndDelete until
// this one commits (row gone), so "whichever caller wins gets it, the other
// observes found=false" holds exactly the same as the Postgres
// DELETE...RETURNING race resolution.
func (s *QueuedPromptStore) GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at
		FROM queued_prompts WHERE pty_id = ? FOR UPDATE`, ptyID)
	p, err := scanQueuedPrompt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	if err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("mysql: select queued prompt for delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM queued_prompts WHERE pty_id = ?`, ptyID); err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("mysql: delete-and-return queued prompt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("mysql: commit delete-and-return queued prompt: %w", err)
	}
	return p, true, nil
}

func scanQueuedPrompt(row rowScanner) (domain.QueuedPrompt, error) {
	var p domain.QueuedPrompt
	var deviceID sql.NullString
	if err := row.Scan(&p.PtyID, &p.TenantID, &p.Prompt, &deviceID, &p.QueuedAt); err != nil {
		return domain.QueuedPrompt{}, err
	}
	if deviceID.Valid {
		p.DispatchedByDeviceID = deviceID.String
	}
	return p, nil
}
