package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type txKeyType struct{}

var txKey = txKeyType{}

// txState remembers the tenant the transaction was opened for so a nested InTx
// with another tenant fails instead of silently running under the outer GUC.
type txState struct {
	tx       pgx.Tx
	tenantID string
}

// InTx joins the transaction already in ctx, otherwise opens one scoped to the
// ctx tenant via set_config. A missing tenant is an error: FORCE RLS would hide
// every row and make the bug hard to see.
func (r *Repository) InTx(ctx context.Context, fn func(context.Context) error) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil || tenantID == "" {
		return domain.ErrRequestTenantRequired()
	}
	if st, ok := ctx.Value(txKey).(txState); ok {
		if st.tenantID != tenantID {
			return apperrors.New(apperrors.KindInternal, "REQUEST_TX_TENANT_MISMATCH", "nested transaction tenant differs from outer transaction", nil)
		}
		return fn(ctx)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID); err != nil {
		return fmt.Errorf("postgres: set tenant config in tx: %w", err)
	}

	if err := fn(context.WithValue(ctx, txKey, txState{tx: tx, tenantID: tenantID})); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

// InTransaction lets the outermost caller know it may retry a lost CAS in a fresh transaction.
func (r *Repository) InTransaction(ctx context.Context) bool {
	return r.inTx(ctx)
}

func (r *Repository) inTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey).(txState)
	return ok
}

// exec returns the ctx transaction, else the bare pool. Tenant-scoped tables
// must go through scoped() instead because the pool carries no tenant GUC.
func (r *Repository) exec(ctx context.Context) dbExecer {
	if st, ok := ctx.Value(txKey).(txState); ok {
		return st.tx
	}
	return r.db
}

// scoped runs fn with an executor bound to the ctx tenant, opening a short
// transaction when the caller is not already inside InTx.
func (r *Repository) scoped(ctx context.Context, fn func(ctx context.Context, tenantID string, db dbExecer) error) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil || tenantID == "" {
		return domain.ErrRequestTenantRequired()
	}
	if st, ok := ctx.Value(txKey).(txState); ok {
		if st.tenantID != tenantID {
			return apperrors.New(apperrors.KindInternal, "REQUEST_TX_TENANT_MISMATCH", "transaction tenant differs from ctx tenant", nil)
		}
		return fn(ctx, tenantID, st.tx)
	}
	return r.InTx(ctx, func(txCtx context.Context) error {
		return fn(txCtx, tenantID, r.exec(txCtx))
	})
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
