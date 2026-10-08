package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RequestContentRepository writes the content columns of requests; RequestRepository.Update never touches them.
type RequestContentRepository struct {
	*Repository
}

func NewRequestContentRepository(r *Repository) *RequestContentRepository {
	return &RequestContentRepository{Repository: r}
}

var _ usecase.RequestContentWriter = (*RequestContentRepository)(nil)

func (r *RequestContentRepository) UpdateContent(ctx context.Context, req domain.Request, expectedVersion int64) (domain.Request, error) {
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		// content_revision is part of the guard so two writers that both read revision N cannot both produce N+1.
		row := db.QueryRow(ctx, `
			UPDATE request.requests SET title = $4, body = $5, acceptance_criteria = $6::jsonb, type_fields = $7::jsonb,
				content_schema_version = $8, content_revision = $9, content_digest = $10, version = version + 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND version = $3 AND content_revision = $9 - 1
			RETURNING `+requestColumns,
			req.ID, tenantID, expectedVersion, req.Title, req.Body, string(req.AcceptanceCriteriaJSON), string(req.TypeFieldsJSON),
			req.ContentSchemaVersion, req.ContentRevision, req.ContentDigest)
		got, err := scanRequest(row)
		if err == nil {
			out = got
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: update request content: %w", err)
		}
		return r.classifyMissing(ctx, db, "request", tenantID, req.ID, domain.ErrRequestNotFound(req.ID), domain.ErrRequestVersionConflict(req.ID, expectedVersion))
	})
	return out, err
}
