package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

// Insert uses ON DUPLICATE KEY UPDATE rather than INSERT IGNORE: IGNORE would also swallow a CHECK violation.
func (r *ArtifactIndexRepository) Insert(ctx context.Context, e domain.IndexEntry) (bool, error) {
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `
			INSERT INTO artifact_index (tenant_id, display_id, kind, request_id, artifact_id) VALUES (?,?,?,?,?)
			ON DUPLICATE KEY UPDATE tenant_id = tenant_id`, tenantID, e.DisplayID, string(e.Kind), e.RequestID, e.ArtifactID)
		if err != nil {
			return fmt.Errorf("mysql: insert artifact index: %w", err)
		}
		n, _ := res.RowsAffected()
		inserted = n == 1
		return nil
	})
	return inserted, err
}

func (r *ArtifactIndexRepository) Resolve(ctx context.Context, displayID string) (domain.IndexEntry, error) {
	var out domain.IndexEntry
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var kind string
		err := db.QueryRowContext(ctx, `SELECT tenant_id, display_id, kind, request_id, artifact_id, created_at FROM artifact_index
			WHERE tenant_id = ? AND display_id = ?`, tenantID, displayID).
			Scan(&out.TenantID, &out.DisplayID, &kind, &out.RequestID, &out.ArtifactID, &out.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrArtifactNotFound(displayID)
		}
		if err != nil {
			return fmt.Errorf("mysql: resolve artifact: %w", err)
		}
		out.Kind, out.CreatedAt = domain.DisplayKind(kind), out.CreatedAt.UTC()
		return nil
	})
	return out, err
}

// lockRequest serialises seq minting per request; the unique key stays the backstop.
func lockRequest(ctx context.Context, db dbExecer, tenantID, requestID string) error {
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM requests WHERE id = ? AND tenant_id = ? FOR UPDATE`, requestID, tenantID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
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
		return db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM solutions WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ArtifactIndexRepository) NextPlanSeq(ctx context.Context, requestID string) (int, error) {
	var n int
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if err := lockRequest(ctx, db, tenantID, requestID); err != nil {
			return err
		}
		return db.QueryRowContext(ctx, `SELECT COUNT(*) + 1 FROM artifact_index WHERE tenant_id = ? AND request_id = ? AND kind = 'plan'`, tenantID, requestID).Scan(&n)
	})
	return n, err
}

func (r *ArtifactIndexRepository) SetSolutionSeq(ctx context.Context, solutionID string, seq int) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE solutions SET seq = ? WHERE id = ? AND tenant_id = ? AND seq IS NULL`, seq, solutionID, tenantID)
		if isDuplicateKey(err, "solutions_request_seq") {
			return domain.ErrArtifactSeqConflict()
		}
		if err != nil {
			return fmt.Errorf("mysql: set solution seq: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return domain.ErrSolutionNotFound(solutionID)
		}
		return nil
	})
}
