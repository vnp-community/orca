package mysql

import (
	"context"
	"database/sql"
	"fmt"
)

type txKeyType struct{}

var txKey = txKeyType{}

func (r *Repository) InTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return fn(ctx)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	txCtx := context.WithValue(ctx, txKey, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

func (r *Repository) exec(ctx context.Context) dbExecer {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return r.db
}
