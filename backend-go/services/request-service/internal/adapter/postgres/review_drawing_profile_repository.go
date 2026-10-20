package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

const profileColumns = `id::text, project_key, spec, spec_digest, version, updated_by::text, updated_at`

func scanProfile(row pgx.Row, tenantID string) (usecase.StoredDrawingProfile, error) {
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
		out, err = scanProfile(db.QueryRow(ctx, `SELECT `+profileColumns+` FROM request.review_drawing_profiles WHERE tenant_id = $1 AND project_key = $2`, tenantID, projectKey), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
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
		var err error
		if expectedVersion == 0 {
			out, err = scanProfile(db.QueryRow(ctx, `INSERT INTO request.review_drawing_profiles (id, tenant_id, project_key, spec, spec_digest, updated_by)
				VALUES ($1, $2, $3, $4::jsonb, $5, $6) ON CONFLICT (tenant_id, project_key) DO NOTHING RETURNING `+profileColumns,
				uuid.NewString(), tenantID, in.ProjectKey, string(in.Spec), in.SpecDigest, in.UpdatedBy), tenantID)
		} else {
			out, err = scanProfile(db.QueryRow(ctx, `UPDATE request.review_drawing_profiles SET spec = $4::jsonb, spec_digest = $5, updated_by = $6, updated_at = clock_timestamp(), version = version + 1
				WHERE tenant_id = $1 AND project_key = $2 AND version = $3 RETURNING `+profileColumns,
				tenantID, in.ProjectKey, expectedVersion, string(in.Spec), in.SpecDigest, in.UpdatedBy), tenantID)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrReviewStaleRevision() // somebody saved first, or the version is not the stored one
		}
		return err
	})
	return out, err
}

func (r *ReviewDrawingProfileRepository) Delete(ctx context.Context, projectKey string, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `DELETE FROM request.review_drawing_profiles WHERE tenant_id = $1 AND project_key = $2 AND version = $3`, tenantID, projectKey, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrReviewStaleRevision()
		}
		return nil
	})
}
