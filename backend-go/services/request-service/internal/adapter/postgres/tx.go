package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/common/tenant"
)

type txKeyType struct{}

var txKey = txKeyType{}

func (r *Repository) InTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if tenantID, err := tenant.RequireTenantID(ctx); err == nil && tenantID != "" {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			return fmt.Errorf("postgres: set tenant config in tx: %w", err)
		}
	}

	txCtx := context.WithValue(ctx, txKey, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

func (r *Repository) exec(ctx context.Context) dbExecer {
	if tx, ok := ctx.Value(txKey).(pgx.Tx); ok {
		return tx
	}
	return r.db
}

func (r *Repository) withRelayTx(ctx context.Context, fn func(context.Context, dbExecer) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.relay', 'on', true)`); err != nil {
		return err
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
