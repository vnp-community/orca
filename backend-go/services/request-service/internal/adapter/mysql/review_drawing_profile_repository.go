package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ReviewDrawingProfileRepository keeps the drawing profile layers of a tenant and its projects.
type ReviewDrawingProfileRepository struct {
	*Repository
}

func NewReviewDrawingProfileRepository(r *Repository) *ReviewDrawingProfileRepository {
	return &ReviewDrawingProfileRepository{Repository: r}
}

var _ usecase.ReviewDrawingProfileRepository = (*ReviewDrawingProfileRepository)(nil)

const profileColumns = `id, project_key, spec, spec_digest, version, updated_by, updated_at`

func scanProfile(row interface{ Scan(dest ...any) error }, tenantID string) (usecase.StoredDrawingProfile, error) {
	var p usecase.StoredDrawingProfile
	if err := row.Scan(&p.ID, &p.ProjectKey, &p.Spec, &p.SpecDigest, &p.Version, &p.UpdatedBy, &p.UpdatedAt); err != nil {
		return usecase.StoredDrawingProfile{}, err
	}
	p.TenantID, p.UpdatedAt = tenantID, p.UpdatedAt.UTC()
	return p, nil
}

func (r *ReviewDrawingProfileRepository) Get(ctx context.Context, projectKey string) (usecase.StoredDrawingProfile, bool, error) {
	var out usecase.StoredDrawingProfile
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		out, err = scanProfile(db.QueryRowContext(ctx, `SELECT `+profileColumns+` FROM review_drawing_profiles WHERE tenant_id = ? AND project_key = ?`, tenantID, projectKey), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *ReviewDrawingProfileRepository) Save(ctx context.Context, in usecase.StoredDrawingProfile, expectedVersion int64) (usecase.StoredDrawingProfile, error) {
	var out usecase.StoredDrawingProfile
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if expectedVersion == 0 {
			if _, err := db.ExecContext(ctx, `INSERT INTO review_drawing_profiles (id, tenant_id, project_key, spec, spec_digest, updated_by) VALUES (?, ?, ?, ?, ?, ?)`,
				uuid.NewString(), tenantID, in.ProjectKey, string(in.Spec), in.SpecDigest, in.UpdatedBy); err != nil {
				if isDuplicateKey(err, "uq_review_drawing_profiles_scope") {
					return domain.ErrReviewStaleRevision() // somebody saved first
				}
				return err
			}
		} else {
			res, err := db.ExecContext(ctx, `UPDATE review_drawing_profiles SET spec = ?, spec_digest = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
				WHERE tenant_id = ? AND project_key = ? AND version = ?`, string(in.Spec), in.SpecDigest, in.UpdatedBy, tenantID, in.ProjectKey, expectedVersion)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return domain.ErrReviewStaleRevision()
			}
		}
		var err error
		out, err = scanProfile(db.QueryRowContext(ctx, `SELECT `+profileColumns+` FROM review_drawing_profiles WHERE tenant_id = ? AND project_key = ?`, tenantID, in.ProjectKey), tenantID)
		return err
	})
	return out, err
}

func (r *ReviewDrawingProfileRepository) Delete(ctx context.Context, projectKey string, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `DELETE FROM review_drawing_profiles WHERE tenant_id = ? AND project_key = ? AND version = ?`, tenantID, projectKey, expectedVersion)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrReviewStaleRevision()
		}
		return nil
	})
}
