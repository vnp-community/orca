package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// inTenantTx runs fn in a transaction with app.tenant_id set, as the FORCE-d RLS policies of
// task_specs and task_execution_records require. Inside RunInTx* it joins the open transaction;
// set_config(..., true) is transaction-local so a pooled connection never carries a tenant over.
func (r *Repository) inTenantTx(ctx context.Context, tenantID string, fn func(db dbtx) error) error {
	if tenantID == "" {
		return fmt.Errorf("postgres: tenant id is required")
	}
	if tx, ok := r.db.(pgx.Tx); ok {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			return fmt.Errorf("postgres: set tenant: %w", err)
		}
		return fn(tx)
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
			return fmt.Errorf("postgres: set tenant: %w", err)
		}
		return fn(tx)
	})
}
