package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutcomeReviewRepository stores reviews.
type OutcomeReviewRepository struct {
	*Repository
}

func NewOutcomeReviewRepository(r *Repository) *OutcomeReviewRepository {
	return &OutcomeReviewRepository{Repository: r}
}

var _ usecase.OutcomeReviewRepository = (*OutcomeReviewRepository)(nil)

const reviewColumns = `id::text, request_id::text, phase, head_key, head_commit, base_commit, inputs_digest, state, model, kg_snapshot,
	design_digest, context_digest, COALESCE(created_by::text, ''), created_at, updated_at, version, COALESCE(approved_by::text, ''), approved_at, approval_note`

func scanReview(row pgx.Row, tenantID string) (usecase.StoredReview, error) {
	var r usecase.StoredReview
	var phase, state string
	if err := row.Scan(&r.ID, &r.RequestID, &phase, &r.HeadKey, &r.HeadCommit, &r.BaseCommit, &r.InputsDigest, &state, &r.Model,
		&r.KGSnapshot, &r.DesignDigest, &r.ContextDigest, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.Version, &r.ApprovedBy, &r.ApprovedAt, &r.ApprovalNote); err != nil {
		return usecase.StoredReview{}, err
	}
	r.TenantID, r.Phase, r.State = tenantID, domain.ReviewPhase(phase), domain.ReviewState(state)
	r.CreatedAt, r.UpdatedAt = r.CreatedAt.UTC(), r.UpdatedAt.UTC()
	if r.ApprovedAt != nil {
		t := r.ApprovedAt.UTC()
		r.ApprovedAt = &t
	}
	return r, nil
}

func (r *OutcomeReviewRepository) Create(ctx context.Context, in usecase.StoredReview) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		row := db.QueryRow(ctx, `INSERT INTO request.outcome_reviews
			(id, tenant_id, request_id, phase, head_key, head_commit, base_commit, inputs_digest, state, model, kg_snapshot, design_digest, context_digest, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14) RETURNING `+reviewColumns,
			in.ID, tenantID, in.RequestID, string(in.Phase), in.HeadKey, in.HeadCommit, in.BaseCommit, in.InputsDigest, string(in.State),
			string(in.Model), in.KGSnapshot, in.DesignDigest, in.ContextDigest, nullIfEmpty(in.CreatedBy))
		var err error
		out, err = scanReview(row, tenantID)
		if err != nil {
			if isUniqueViolation(err, "uq_outcome_reviews_key") {
				return domain.ErrReviewAlreadyExists(in.RequestID)
			}
			return fmt.Errorf("postgres: create outcome review: %w", err)
		}
		return nil
	})
	return out, err
}

func (r *OutcomeReviewRepository) Get(ctx context.Context, id string) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `SELECT `+reviewColumns+` FROM request.outcome_reviews WHERE tenant_id = $1 AND id = $2`, tenantID, id), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrReviewNotFound(id)
		}
		return err
	})
	return out, err
}

func (r *OutcomeReviewRepository) GetByKey(ctx context.Context, requestID string, phase domain.ReviewPhase, headKey string) (usecase.StoredReview, bool, error) {
	var out usecase.StoredReview
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `SELECT `+reviewColumns+` FROM request.outcome_reviews
			WHERE tenant_id = $1 AND request_id = $2 AND phase = $3 AND head_key = $4`, tenantID, requestID, string(phase), headKey), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *OutcomeReviewRepository) Latest(ctx context.Context, requestID string) (usecase.StoredReview, bool, error) {
	var out usecase.StoredReview
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `SELECT `+reviewColumns+` FROM request.outcome_reviews
			WHERE tenant_id = $1 AND request_id = $2 AND state <> 'superseded' ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, requestID), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}

func (r *OutcomeReviewRepository) List(ctx context.Context, requestID string, pageSize int, afterID string) ([]usecase.StoredReview, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var out []usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		q := `SELECT ` + reviewColumns + ` FROM request.outcome_reviews WHERE tenant_id = $1 AND request_id = $2`
		args := []any{tenantID, requestID}
		if afterID != "" && !notUUID(afterID) {
			q += ` AND (created_at, id) < (SELECT created_at, id FROM request.outcome_reviews WHERE tenant_id = $1 AND id = $3)`
			args = append(args, afterID)
		}
		q += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT %d`, pageSize)
		rows, err := db.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("postgres: list outcome reviews: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			rv, err := scanReview(rows, tenantID)
			if err != nil {
				return err
			}
			out = append(out, rv)
		}
		return rows.Err()
	})
	return out, err
}

func (r *OutcomeReviewRepository) Update(ctx context.Context, in usecase.StoredReview, expectedVersion int64) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(in.ID) {
			return domain.ErrReviewNotFound(in.ID)
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `UPDATE request.outcome_reviews SET head_commit = $4, base_commit = $5, inputs_digest = $6, state = $7,
			model = $8::jsonb, kg_snapshot = $9, design_digest = $10, context_digest = $11, updated_at = clock_timestamp(), version = version + 1,
			approved_by = CASE WHEN $7 = 'approved' THEN approved_by END, approved_at = CASE WHEN $7 = 'approved' THEN approved_at END, approval_note = CASE WHEN $7 = 'approved' THEN approval_note ELSE '' END
			WHERE tenant_id = $1 AND id = $2 AND version = $3 RETURNING `+reviewColumns,
			tenantID, in.ID, expectedVersion, in.HeadCommit, in.BaseCommit, in.InputsDigest, string(in.State), string(in.Model),
			in.KGSnapshot, in.DesignDigest, in.ContextDigest), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return r.versionOrMissing(ctx, db, tenantID, in.ID)
		}
		return err
	})
	return out, err
}

func (r *OutcomeReviewRepository) SetState(ctx context.Context, id string, state domain.ReviewState, expectedVersion int64) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `UPDATE request.outcome_reviews SET state = $4, updated_at = clock_timestamp(), version = version + 1,
			approved_by = CASE WHEN $4 = 'approved' THEN approved_by END, approved_at = CASE WHEN $4 = 'approved' THEN approved_at END, approval_note = CASE WHEN $4 = 'approved' THEN approval_note ELSE '' END
			WHERE tenant_id = $1 AND id = $2 AND version = $3 RETURNING `+reviewColumns, tenantID, id, expectedVersion, string(state)), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return r.versionOrMissing(ctx, db, tenantID, id)
		}
		return err
	})
	return out, err
}

func (r *OutcomeReviewRepository) Approve(ctx context.Context, id, by, note string, expectedVersion int64) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		var err error
		out, err = scanReview(db.QueryRow(ctx, `UPDATE request.outcome_reviews SET state = 'approved', approved_by = $4, approved_at = clock_timestamp(), approval_note = $5,
			updated_at = clock_timestamp(), version = version + 1 WHERE tenant_id = $1 AND id = $2 AND version = $3 RETURNING `+reviewColumns,
			tenantID, id, expectedVersion, by, note), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return r.versionOrMissing(ctx, db, tenantID, id)
		}
		return err
	})
	return out, err
}

// versionOrMissing tells a lost race from a row that is not there, so the caller can reload or give up.
func (r *OutcomeReviewRepository) versionOrMissing(ctx context.Context, db dbExecer, tenantID, id string) error {
	var one int
	err := db.QueryRow(ctx, `SELECT 1 FROM request.outcome_reviews WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrReviewNotFound(id)
	}
	if err != nil {
		return err
	}
	return domain.ErrReviewStaleRevision()
}

func (r *OutcomeReviewRepository) SupersedeOthers(ctx context.Context, requestID, exceptID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) || notUUID(exceptID) {
			return nil
		}
		_, err := db.Exec(ctx, `UPDATE request.outcome_reviews SET state = 'superseded', updated_at = clock_timestamp(), version = version + 1
			WHERE tenant_id = $1 AND request_id = $2 AND id <> $3 AND state <> 'superseded'`, tenantID, requestID, exceptID)
		return err
	})
}
