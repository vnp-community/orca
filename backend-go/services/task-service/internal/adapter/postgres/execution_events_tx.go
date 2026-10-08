package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// txBeginner is satisfied by both the pool and an open pgx.Tx (savepoint), so a
// repository already inside RunInTx nests instead of escaping its transaction.
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// inOptionalTx runs fn in a transaction only when needed; without events the
// single UPDATE keeps its original one-round-trip path.
func (r *Repository) inOptionalTx(ctx context.Context, needTx bool, fn func(db dbtx) error) error {
	b, ok := r.db.(txBeginner)
	if !needTx || !ok {
		return fn(r.db)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

func insertOutboxEvents(ctx context.Context, db dbtx, tenantID string, events []domain.OutboxEvent) error {
	for _, event := range events {
		if _, err := db.Exec(ctx, `
			INSERT INTO task.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, 1, $5)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}
	return nil
}
