package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
		tag, err := db.Exec(ctx, `INSERT INTO request.request_idempotency (tenant_id, source_provider, source_site, source_ref, request_id)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, tenantID, sourceProvider, sourceSite, sourceRef, requestID)
		if err != nil {
			return fmt.Errorf("postgres: claim idempotency: %w", err)
		}
		if tag.RowsAffected() == 1 {
			claimed = true
			return nil
		}
		return db.QueryRow(ctx, `SELECT request_id FROM request.request_idempotency
			WHERE tenant_id = $1 AND source_provider = $2 AND source_site = $3 AND source_ref = $4`,
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
		err := db.QueryRow(ctx, `SELECT request_id FROM request.request_idempotency
			WHERE tenant_id = $1 AND source_provider = $2 AND source_site = $3 AND source_ref = $4`,
			tenantID, sourceProvider, sourceSite, sourceRef).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
	return id, err
}
