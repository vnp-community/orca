package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutcomeReviewContextRepository stores the locked design and the checked results of context runs.
type OutcomeReviewContextRepository struct {
	*Repository
}

func NewOutcomeReviewContextRepository(r *Repository) *OutcomeReviewContextRepository {
	return &OutcomeReviewContextRepository{Repository: r}
}

var _ usecase.ReviewContextStore = (*OutcomeReviewContextRepository)(nil)

const contextColumns = `id::text, request_id::text, kind, digest, prompt_version, payload, created_at, expires_at`

func scanContext(row pgx.Row, tenantID string) (usecase.ReviewContextRecord, error) {
	var r usecase.ReviewContextRecord
	var kind string
	if err := row.Scan(&r.ID, &r.RequestID, &kind, &r.Digest, &r.PromptVersion, &r.Payload, &r.CreatedAt, &r.ExpiresAt); err != nil {
		return usecase.ReviewContextRecord{}, err
	}
	r.TenantID, r.Kind = tenantID, usecase.ReviewContextKind(kind)
	r.CreatedAt, r.ExpiresAt = r.CreatedAt.UTC(), r.ExpiresAt.UTC()
	return r, nil
}

func (r *OutcomeReviewContextRepository) Put(ctx context.Context, in usecase.ReviewContextRecord) (usecase.ReviewContextRecord, error) {
	var out usecase.ReviewContextRecord
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		out, err = scanContext(db.QueryRow(ctx, `INSERT INTO request.outcome_review_contexts (id, tenant_id, request_id, kind, digest, prompt_version, payload, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
			ON CONFLICT (tenant_id, request_id, kind, digest) DO UPDATE
			SET payload = EXCLUDED.payload, prompt_version = EXCLUDED.prompt_version, expires_at = EXCLUDED.expires_at, created_at = clock_timestamp()
			RETURNING `+contextColumns, in.ID, tenantID, in.RequestID, string(in.Kind), in.Digest, in.PromptVersion, string(in.Payload), in.ExpiresAt), tenantID)
		if err != nil {
			return fmt.Errorf("postgres: put outcome review context: %w", err)
		}
		return nil
	})
	return out, err
}

func (r *OutcomeReviewContextRepository) Get(ctx context.Context, requestID string, kind usecase.ReviewContextKind, digest string, now time.Time) (usecase.ReviewContextRecord, bool, error) {
	var out usecase.ReviewContextRecord
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanContext(db.QueryRow(ctx, `SELECT `+contextColumns+` FROM request.outcome_review_contexts
			WHERE tenant_id = $1 AND request_id = $2 AND kind = $3 AND digest = $4 AND expires_at > $5`, tenantID, requestID, string(kind), digest, now), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *OutcomeReviewContextRepository) Latest(ctx context.Context, requestID string, kind usecase.ReviewContextKind, now time.Time) (usecase.ReviewContextRecord, bool, error) {
	var out usecase.ReviewContextRecord
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanContext(db.QueryRow(ctx, `SELECT `+contextColumns+` FROM request.outcome_review_contexts
			WHERE tenant_id = $1 AND request_id = $2 AND kind = $3 AND expires_at > $4 ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, requestID, string(kind), now), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}
