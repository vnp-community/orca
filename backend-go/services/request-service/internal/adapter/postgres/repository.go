package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{db: pool}
}

var _ usecase.TxRunner = (*Repository)(nil)
var _ usecase.OutboxWriter = (*Repository)(nil)
var _ outbox.Store = (*Repository)(nil)

type dbExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
