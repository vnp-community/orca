package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// inOptionalTx runs fn in a transaction only when needed. A repository already
// inside RunInTx holds a *sql.Tx (no BeginTx), so it reuses that transaction.
func (r *Repository) inOptionalTx(ctx context.Context, needTx bool, fn func(db dbtx) error) error {
	pool, ok := r.db.(*sql.DB)
	if !needTx || !ok {
		return fn(r.db)
	}
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

func insertOutboxEvents(ctx context.Context, db dbtx, tenantID string, events []domain.OutboxEvent) error {
	for _, event := range events {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}
	return nil
}
