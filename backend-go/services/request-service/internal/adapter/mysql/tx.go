package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type txKeyType struct{}

var txKey = txKeyType{}

// txState remembers the tenant the transaction was opened for so a nested InTx
// with another tenant fails instead of silently joining it.
type txState struct {
	tx       *sql.Tx
	tenantID string
}

// InTx joins the transaction already in ctx, otherwise opens one. MySQL has no RLS, so
// the tenant is enforced here and in every query's WHERE; a missing tenant is an error.
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := fn(context.WithValue(ctx, txKey, txState{tx: tx, tenantID: tenantID})); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
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

func (r *Repository) exec(ctx context.Context) dbExecer {
	if st, ok := ctx.Value(txKey).(txState); ok {
		return st.tx
	}
	return r.db
}

// scoped runs fn with the ctx tenant and an executor that joins the ctx transaction,
// or a short transaction when the caller is outside InTx.
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
