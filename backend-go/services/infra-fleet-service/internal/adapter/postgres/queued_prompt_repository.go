package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// QueuedPromptStore implements usecase.QueuedPromptRepository against
// infra.queued_prompts (migrations/0008_queued_prompts) — split into its own
// type over the same pool rather than a method on Repository, same
// rationale as TerminalSessionStore (see Repository's doc comment): this
// table has its own natural key shape (pty_id) unrelated to the dev-server
// registry Repository owns.
type QueuedPromptStore struct{ pool *pgxpool.Pool }

func NewQueuedPromptStore(pool *pgxpool.Pool) *QueuedPromptStore {
	return &QueuedPromptStore{pool: pool}
}

// Get fetches the queued prompt for ptyID — found=false (not an error) when
// no row matches, per usecase.QueuedPromptRepository's doc comment.
func (s *QueuedPromptStore) Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at
		FROM infra.queued_prompts WHERE pty_id = $1`, ptyID)
	p, err := scanQueuedPrompt(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	if err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("postgres: query queued prompt: %w", err)
	}
	return p, true, nil
}

// Upsert inserts or replaces the single queued-prompt row for p.PtyID —
// callers enforce BR-MB-12's overwrite-requires-confirmation rule before
// ever calling this; this store itself does not reject a silent replace.
func (s *QueuedPromptStore) Upsert(ctx context.Context, p domain.QueuedPrompt) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO infra.queued_prompts (pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5)
		ON CONFLICT (pty_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			prompt = EXCLUDED.prompt,
			dispatched_by_device_id = EXCLUDED.dispatched_by_device_id,
			queued_at = EXCLUDED.queued_at`,
		p.PtyID, p.TenantID, p.Prompt, p.DispatchedByDeviceID, p.QueuedAt)
	if err != nil {
		return fmt.Errorf("postgres: upsert queued prompt: %w", err)
	}
	return nil
}

// Delete removes the queued-prompt row for ptyID, if any — idempotent: no
// matching row is not an error.
func (s *QueuedPromptStore) Delete(ctx context.Context, ptyID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM infra.queued_prompts WHERE pty_id = $1`, ptyID)
	if err != nil {
		return fmt.Errorf("postgres: delete queued prompt: %w", err)
	}
	return nil
}

// GetAndDelete atomically reads and removes the row for ptyID — the
// regression guard the DispatchPrompt queue-drain hook (in
// GetTerminalAgentStatus's ready-transition branch) needs against a
// double-delivery race between that hook and a concurrent DispatchPrompt
// call: whichever caller's DELETE...RETURNING wins the row gets it, the
// other observes found=false.
func (s *QueuedPromptStore) GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	row := s.pool.QueryRow(ctx, `
		DELETE FROM infra.queued_prompts WHERE pty_id = $1
		RETURNING pty_id, tenant_id, prompt, dispatched_by_device_id, queued_at`, ptyID)
	p, err := scanQueuedPrompt(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueuedPrompt{}, false, nil
	}
	if err != nil {
		return domain.QueuedPrompt{}, false, fmt.Errorf("postgres: delete-and-return queued prompt: %w", err)
	}
	return p, true, nil
}

func scanQueuedPrompt(row rowScanner) (domain.QueuedPrompt, error) {
	var p domain.QueuedPrompt
	var deviceID *string
	if err := row.Scan(&p.PtyID, &p.TenantID, &p.Prompt, &deviceID, &p.QueuedAt); err != nil {
		return domain.QueuedPrompt{}, err
	}
	if deviceID != nil {
		p.DispatchedByDeviceID = *deviceID
	}
	return p, nil
}
