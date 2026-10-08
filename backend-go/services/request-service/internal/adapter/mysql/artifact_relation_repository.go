package mysql

import (
	"context"
	"database/sql"
	"fmt"

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
		res, err := db.ExecContext(ctx, `
			INSERT INTO artifact_relations (id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id, created_by_run_id)
			VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE tenant_id = tenant_id`,
			id, tenantID, rel.RequestID, string(rel.Rel), string(rel.FromKind), rel.FromID, string(rel.ToKind), rel.ToID, nullIfEmpty(rel.CreatedByRunID))
		if err != nil {
			return fmt.Errorf("mysql: insert artifact relation: %w", err)
		}
		n, _ := res.RowsAffected()
		inserted = n == 1
		return nil
	})
	return inserted, err
}

func (r *ArtifactRelationRepository) ListByRequest(ctx context.Context, requestID string) ([]domain.ArtifactRelation, error) {
	var out []domain.ArtifactRelation
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `
			SELECT id, tenant_id, request_id, rel, from_kind, from_id, to_kind, to_id, created_by_run_id, created_at
			FROM artifact_relations WHERE tenant_id = ? AND request_id = ? ORDER BY rel, from_id, to_id`, tenantID, requestID)
		if err != nil {
			return fmt.Errorf("mysql: list artifact relations: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.ArtifactRelation
			var rel, fk, tk string
			var run sql.NullString
			if err := rows.Scan(&a.ID, &a.TenantID, &a.RequestID, &rel, &fk, &a.FromID, &tk, &a.ToID, &run, &a.CreatedAt); err != nil {
				return err
			}
			a.Rel, a.FromKind, a.ToKind = domain.Relation(rel), domain.NodeKind(fk), domain.NodeKind(tk)
			a.CreatedByRunID, a.CreatedAt = run.String, a.CreatedAt.UTC()
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}
