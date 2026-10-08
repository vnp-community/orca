package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type ArtifactIndexRepository struct {
	*Repository
}

func NewArtifactIndexRepository(r *Repository) *ArtifactIndexRepository {
	return &ArtifactIndexRepository{Repository: r}
}

var _ usecase.ArtifactIndexRepository = (*ArtifactIndexRepository)(nil)

func (r *ArtifactIndexRepository) Insert(ctx context.Context, e domain.IndexEntry) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `
			INSERT INTO request.artifact_index (tenant_id, display_id, kind, request_id, artifact_id)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, tenantID, e.DisplayID, string(e.Kind), e.RequestID, e.ArtifactID)
		if err != nil {
			return fmt.Errorf("postgres: insert artifact index: %w", err)
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (r *ArtifactIndexRepository) Resolve(ctx context.Context, displayID string) (domain.IndexEntry, error) {
	var out domain.IndexEntry
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var kind string
		err := db.QueryRow(ctx, `SELECT tenant_id, display_id, kind, request_id, artifact_id, created_at FROM request.artifact_index
			WHERE tenant_id = $1 AND display_id = $2`, tenantID, displayID).
			Scan(&out.TenantID, &out.DisplayID, &kind, &out.RequestID, &out.ArtifactID, &out.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrArtifactNotFound(displayID)
		}
		if err != nil {
			return fmt.Errorf("postgres: resolve artifact: %w", err)
		}
		out.Kind, out.CreatedAt = domain.DisplayKind(kind), out.CreatedAt.UTC()
		return nil
	})
	return out, err
}

// lockRequest serialises seq minting per request; the unique key stays the backstop.
func lockRequest(ctx context.Context, db dbExecer, tenantID, requestID string) error {
	if _, err := uuid.Parse(requestID); err != nil {
		return domain.ErrRequestNotFound(requestID)
	}
	var one int
	err := db.QueryRow(ctx, `SELECT 1 FROM request.requests WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, requestID, tenantID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRequestNotFound(requestID)
	}
	return err
}

func (r *ArtifactIndexRepository) NextSolutionSeq(ctx context.Context, requestID string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if err := lockRequest(ctx, db, tenantID, requestID); err != nil {
			return err
		}
		return db.QueryRow(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM request.solutions WHERE tenant_id = $1 AND request_id = $2`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ArtifactIndexRepository) NextPlanSeq(ctx context.Context, requestID string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if err := lockRequest(ctx, db, tenantID, requestID); err != nil {
			return err
		}
		return db.QueryRow(ctx, `SELECT COUNT(*) + 1 FROM request.artifact_index WHERE tenant_id = $1 AND request_id = $2 AND kind = 'plan'`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ArtifactIndexRepository) SetSolutionSeq(ctx context.Context, solutionID string, seq int) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.solutions SET seq = $3 WHERE id = $1 AND tenant_id = $2 AND seq IS NULL`, solutionID, tenantID, seq)
		if isUniqueViolation(err, "solutions_request_seq") {
			return domain.ErrArtifactSeqConflict()
		}
		if err != nil {
			return fmt.Errorf("postgres: set solution seq: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrSolutionNotFound(solutionID)
		}
		return nil
	})
}
