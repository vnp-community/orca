package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ArtifactRelationRepository struct {
	*Repository
}

func NewArtifactRelationRepository(r *Repository) *ArtifactRelationRepository {
	return &ArtifactRelationRepository{Repository: r}
}

var _ usecase.ArtifactRelationRepository = (*ArtifactRelationRepository)(nil)

func (r *ArtifactRelationRepository) Insert(ctx context.Context, rel domain.ArtifactRelation) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		id := rel.ID
		if id == "" {
			id = uuid.NewString()
		}
		tag, err := db.Exec(ctx, `
			INSERT INTO request.artifact_relations (id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id, created_by_run_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::uuid) ON CONFLICT DO NOTHING`,
			id, tenantID, rel.RequestID, string(rel.Rel), string(rel.FromKind), rel.FromID, string(rel.ToKind), rel.ToID, nullIfEmpty(rel.CreatedByRunID))
		if err != nil {
			return fmt.Errorf("postgres: insert artifact relation: %w", err)
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (r *ArtifactRelationRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.ArtifactRelation, error) {
	var out []domain.ArtifactRelation
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(requestID); perr != nil {
			return nil
		}
		rows, err := db.Query(ctx, `
			SELECT id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id, created_by_run_id, created_at
			FROM request.artifact_relations WHERE tenant_id = $1 AND request_id = $2 ORDER BY rel, from_id, to_id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("postgres: list artifact relations: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				a           domain.ArtifactRelation
				rel, fk, tk string
				run         *string
				created     time.Time
			)
			if err := rows.Scan(&a.ID, &a.TenantID, &a.RequestID, &rel, &fk, &a.FromID, &tk, &a.ToID, &run, &created); err != nil {
				return err
			}
			a.Rel, a.FromKind, a.ToKind = domain.Relation(rel), domain.NodeKind(fk), domain.NodeKind(tk)
			a.CreatedByRunID, a.CreatedAt = derefString(run), created.UTC()
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}
