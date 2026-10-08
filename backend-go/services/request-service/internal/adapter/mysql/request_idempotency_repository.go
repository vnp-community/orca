package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestIdempotencyRepository struct {
	*Repository
}

func NewRequestIdempotencyRepository(r *Repository) *RequestIdempotencyRepository {
	return &RequestIdempotencyRepository{Repository: r}
}

var _ usecase.RequestIdempotencyRepository = (*RequestIdempotencyRepository)(nil)

// Claim reserves a source key for requestID; the loser gets the winner's id back.
func (r *RequestIdempotencyRepository) Claim(ctx context.Context, sourceProvider, sourceSite, sourceRef, requestID string) (string, bool, error) {
	var existing string
	var claimed bool
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		// ON DUPLICATE KEY no-op instead of INSERT IGNORE, which would also swallow CHECK violations.
		res, err := db.ExecContext(ctx, `INSERT INTO request_idempotency (tenant_id, source_provider, source_site, source_ref, request_id)
			VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, tenantID, sourceProvider, sourceSite, sourceRef, requestID)
		if err != nil {
			return fmt.Errorf("mysql: claim idempotency: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			claimed = true
			return nil
		}
		return db.QueryRowContext(ctx, `SELECT request_id FROM request_idempotency
			WHERE tenant_id = ? AND source_provider = ? AND source_site = ? AND source_ref = ?`,
			tenantID, sourceProvider, sourceSite, sourceRef).Scan(&existing)
	})
	if err != nil {
		return "", false, err
	}
	return existing, claimed, nil
}

// Find returns "" when the key was never claimed.
func (r *RequestIdempotencyRepository) Find(ctx context.Context, sourceProvider, sourceSite, sourceRef string) (string, error) {
	var id string
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRowContext(ctx, `SELECT request_id FROM request_idempotency
			WHERE tenant_id = ? AND source_provider = ? AND source_site = ? AND source_ref = ?`,
			tenantID, sourceProvider, sourceSite, sourceRef).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	return id, err
}
