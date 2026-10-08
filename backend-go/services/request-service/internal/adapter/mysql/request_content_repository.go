package mysql

import (
	"context"
	"fmt"

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
		res, err := db.ExecContext(ctx, `
			UPDATE requests SET title = ?, body = ?, acceptance_criteria = ?, type_fields = ?, content_schema_version = ?,
				content_revision = ?, content_digest = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE id = ? AND tenant_id = ? AND version = ? AND content_revision = ?`,
			req.Title, req.Body, string(req.AcceptanceCriteriaJSON), string(req.TypeFieldsJSON), req.ContentSchemaVersion,
			req.ContentRevision, req.ContentDigest, req.ID, tenantID, expectedVersion, req.ContentRevision-1)
		if err != nil {
			return fmt.Errorf("mysql: update request content: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return r.classifyMissing(ctx, db, "request", tenantID, req.ID, domain.ErrRequestNotFound(req.ID), domain.ErrRequestVersionConflict(req.ID, expectedVersion))
		}
		got, err := scanRequest(db.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM requests WHERE id = ? AND tenant_id = ?`, req.ID, tenantID))
		if err != nil {
			return fmt.Errorf("mysql: reread request: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}
