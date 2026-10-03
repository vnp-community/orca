package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CloseWithEvent implements usecase.TerminalSessionCloser — see the Postgres
// variant: the closed_at transition and the outbox row share one transaction,
// and only the NULL -> set UPDATE enqueues.
func (s *TerminalSessionStore) CloseWithEvent(ctx context.Context, tenantID, ptyID string, closedAt time.Time, event domain.OutboxEvent) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("mysql: begin close terminal session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE terminal_sessions SET closed_at = ?
		WHERE tenant_id = ? AND pty_id = ? AND closed_at IS NULL
	`, closedAt, tenantID, ptyID)
	if err != nil {
		return false, fmt.Errorf("mysql: close terminal session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: rows affected: %w", err)
	}
	transitioned := n > 0
	if transitioned {
		// INSERT IGNORE would also swallow data errors; a duplicate id is the
		// only conflict we tolerate, so no-op the update instead.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
			ON DUPLICATE KEY UPDATE id = id
		`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return false, fmt.Errorf("mysql: enqueue terminal closed event: %w", err)
		}
	} else {
		var one int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM terminal_sessions WHERE tenant_id = ? AND pty_id = ?`, tenantID, ptyID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("mysql: terminal session %q not found for tenant", ptyID)
		}
		if err != nil {
			return false, fmt.Errorf("mysql: check terminal session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("mysql: commit close terminal session: %w", err)
	}
	return transitioned, nil
}
