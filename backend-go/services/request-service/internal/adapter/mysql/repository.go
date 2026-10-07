package mysql

import (
	"context"
	"database/sql"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

var _ usecase.TxRunner = (*Repository)(nil)
var _ usecase.OutboxWriter = (*Repository)(nil)
var _ outbox.Store = (*Repository)(nil)

type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
