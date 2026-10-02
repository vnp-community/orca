package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// withTenantTx runs fn in a transaction with app.tenant_id set for RLS.
// Every tenant-scoped query must go through here; set_config(..., true) is
// transaction-local so a pooled connection never carries a tenant over.
func (r *Repository) withTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			return fmt.Errorf("postgres: set tenant: %w", err)
		}
		return fn(tx)
	})
}

// withRelayTx opts the transaction into the cross-tenant outbox policies.
// Only the two common/outbox.Store methods may use it.
func (r *Repository) withRelayTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.relay', 'on', true)`); err != nil {
			return fmt.Errorf("postgres: set relay: %w", err)
		}
		return fn(tx)
	})
}

func (r *Repository) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}
