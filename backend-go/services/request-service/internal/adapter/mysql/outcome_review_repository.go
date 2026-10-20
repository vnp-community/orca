package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
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

const reviewColumns = `id, request_id, phase, head_key, head_commit, base_commit, inputs_digest, state, model, kg_snapshot,
	design_digest, context_digest, COALESCE(created_by, ''), created_at, updated_at, version`

func notUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err != nil
}

func scanReview(row interface{ Scan(dest ...any) error }, tenantID string) (usecase.StoredReview, error) {
	var r usecase.StoredReview
	var phase, state string
	if err := row.Scan(&r.ID, &r.RequestID, &phase, &r.HeadKey, &r.HeadCommit, &r.BaseCommit, &r.InputsDigest, &state, &r.Model,
		&r.KGSnapshot, &r.DesignDigest, &r.ContextDigest, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.Version); err != nil {
		return usecase.StoredReview{}, err
	}
	r.TenantID, r.Phase, r.State = tenantID, domain.ReviewPhase(phase), domain.ReviewState(state)
	r.CreatedAt, r.UpdatedAt = r.CreatedAt.UTC(), r.UpdatedAt.UTC()
	return r, nil
}

func (r *OutcomeReviewRepository) Create(ctx context.Context, in usecase.StoredReview) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `INSERT INTO outcome_reviews
			(id, tenant_id, request_id, phase, head_key, head_commit, base_commit, inputs_digest, state, model, kg_snapshot, design_digest, context_digest, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.ID, tenantID, in.RequestID, string(in.Phase), in.HeadKey, in.HeadCommit, in.BaseCommit, in.InputsDigest, string(in.State),
			string(in.Model), in.KGSnapshot, in.DesignDigest, in.ContextDigest, nullIfEmpty(in.CreatedBy)); err != nil {
			if isDuplicateKey(err, "uq_outcome_reviews_key") {
				return domain.ErrReviewAlreadyExists(in.RequestID)
			}
			return fmt.Errorf("mysql: create outcome review: %w", err)
		}
		var err error
		out, err = scanReview(db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM outcome_reviews WHERE tenant_id = ? AND id = ?`, tenantID, in.ID), tenantID)
		return err
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
		out, err = scanReview(db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM outcome_reviews WHERE tenant_id = ? AND id = ?`, tenantID, id), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
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
		out, err = scanReview(db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM outcome_reviews
			WHERE tenant_id = ? AND request_id = ? AND phase = ? AND head_key = ?`, tenantID, requestID, string(phase), headKey), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
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
		out, err = scanReview(db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM outcome_reviews
			WHERE tenant_id = ? AND request_id = ? AND state <> 'superseded' ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, requestID), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
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
		q := `SELECT ` + reviewColumns + ` FROM outcome_reviews WHERE tenant_id = ? AND request_id = ?`
		args := []any{tenantID, requestID}
		if afterID != "" && !notUUID(afterID) {
			q += ` AND (created_at, id) < (SELECT created_at, id FROM outcome_reviews WHERE tenant_id = ? AND id = ?)`
			args = append(args, tenantID, afterID)
		}
		q += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT %d`, pageSize)
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("mysql: list outcome reviews: %w", err)
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
		res, err := db.ExecContext(ctx, `UPDATE outcome_reviews SET head_commit = ?, base_commit = ?, inputs_digest = ?, state = ?, model = ?, kg_snapshot = ?,
			design_digest = ?, context_digest = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
			WHERE tenant_id = ? AND id = ? AND version = ?`,
			in.HeadCommit, in.BaseCommit, in.InputsDigest, string(in.State), string(in.Model), in.KGSnapshot, in.DesignDigest, in.ContextDigest,
			tenantID, in.ID, expectedVersion)
		if err != nil {
			return err
		}
		return r.afterVersionedWrite(ctx, db, tenantID, in.ID, res, &out)
	})
	return out, err
}

func (r *OutcomeReviewRepository) SetState(ctx context.Context, id string, state domain.ReviewState, expectedVersion int64) (usecase.StoredReview, error) {
	var out usecase.StoredReview
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		res, err := db.ExecContext(ctx, `UPDATE outcome_reviews SET state = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
			WHERE tenant_id = ? AND id = ? AND version = ?`, string(state), tenantID, id, expectedVersion)
		if err != nil {
			return err
		}
		return r.afterVersionedWrite(ctx, db, tenantID, id, res, &out)
	})
	return out, err
}

// afterVersionedWrite reads the row back when the version matched and otherwise says whether the row is missing or the version is stale.
func (r *OutcomeReviewRepository) afterVersionedWrite(ctx context.Context, db dbExecer, tenantID, id string, res sql.Result, out *usecase.StoredReview) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var one int
		qerr := db.QueryRowContext(ctx, `SELECT 1 FROM outcome_reviews WHERE tenant_id = ? AND id = ?`, tenantID, id).Scan(&one)
		if errors.Is(qerr, sql.ErrNoRows) {
			return domain.ErrReviewNotFound(id)
		}
		if qerr != nil {
			return qerr
		}
		return domain.ErrReviewStaleRevision()
	}
	*out, err = scanReview(db.QueryRowContext(ctx, `SELECT `+reviewColumns+` FROM outcome_reviews WHERE tenant_id = ? AND id = ?`, tenantID, id), tenantID)
	return err
}

func (r *OutcomeReviewRepository) SupersedeOthers(ctx context.Context, requestID, exceptID string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) || notUUID(exceptID) {
			return nil
		}
		_, err := db.ExecContext(ctx, `UPDATE outcome_reviews SET state = 'superseded', updated_at = CURRENT_TIMESTAMP(6), version = version + 1
			WHERE tenant_id = ? AND request_id = ? AND id <> ? AND state <> 'superseded'`, tenantID, requestID, exceptID)
		return err
	})
}
