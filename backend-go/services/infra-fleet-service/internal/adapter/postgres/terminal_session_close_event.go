package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CloseWithEvent implements usecase.TerminalSessionCloser: the closed_at
// transition and the outbox row commit together or not at all. Only the
// UPDATE that actually flips closed_at from NULL enqueues, and the event id is
// deterministic, so a retried or concurrent close never duplicates it.
func (s *TerminalSessionStore) CloseWithEvent(ctx context.Context, tenantID, ptyID string, closedAt time.Time, event domain.OutboxEvent) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("postgres: begin close terminal session: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE infra.terminal_sessions SET closed_at = $3
		WHERE tenant_id = $1 AND pty_id = $2 AND closed_at IS NULL
	`, tenantID, ptyID, closedAt)
	if err != nil {
		return false, fmt.Errorf("postgres: close terminal session: %w", err)
	}
	transitioned := tag.RowsAffected() > 0
	if transitioned {
		if _, err := tx.Exec(ctx, `
			INSERT INTO infra.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, 1, $5)
			ON CONFLICT (id) DO NOTHING
		`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return false, fmt.Errorf("postgres: enqueue terminal closed event: %w", err)
		}
	} else {
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM infra.terminal_sessions WHERE tenant_id = $1 AND pty_id = $2`, tenantID, ptyID).Scan(&one)
		if err == pgx.ErrNoRows {
			return false, fmt.Errorf("postgres: terminal session %q not found for tenant", ptyID)
		}
		if err != nil {
			return false, fmt.Errorf("postgres: check terminal session: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("postgres: commit close terminal session: %w", err)
	}
	return transitioned, nil
}
