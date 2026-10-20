package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// OutcomeReviewRunRepository records the agent runs of a review.
type OutcomeReviewRunRepository struct {
	*Repository
}

func NewOutcomeReviewRunRepository(r *Repository) *OutcomeReviewRunRepository {
	return &OutcomeReviewRunRepository{Repository: r}
}

var _ usecase.OutcomeReviewRunRepository = (*OutcomeReviewRunRepository)(nil)

const runColumns = `id, review_id, request_id, kind, status, scope, agent_run_ref, cost_estimate, cost_actual, error_code, started_at, finished_at`

func scanRun(row interface{ Scan(dest ...any) error }, tenantID string) (usecase.ReviewRun, error) {
	var r usecase.ReviewRun
	var kind, status string
	var finished sql.NullTime
	if err := row.Scan(&r.ID, &r.ReviewID, &r.RequestID, &kind, &status, &r.Scope, &r.AgentRunRef, &r.CostEstimate, &r.CostActual, &r.ErrorCode, &r.StartedAt, &finished); err != nil {
		return usecase.ReviewRun{}, err
	}
	r.TenantID, r.Kind, r.Status, r.StartedAt = tenantID, usecase.ReviewRunKind(kind), usecase.ReviewRunStatus(status), r.StartedAt.UTC()
	if finished.Valid {
		t := finished.Time.UTC()
		r.FinishedAt = &t
	}
	return r, nil
}

func activeKey(tenantID, requestID string, kind usecase.ReviewRunKind) string {
	return tenantID + ":" + requestID + ":" + string(kind)
}

func (r *OutcomeReviewRunRepository) Start(ctx context.Context, in usecase.ReviewRun) (usecase.ReviewRun, error) {
	var out usecase.ReviewRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		status := in.Status
		if status == "" {
			status = usecase.ReviewRunRunning
		}
		var key any
		if status.Active() {
			key = activeKey(tenantID, in.RequestID, in.Kind)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO outcome_review_runs (id, tenant_id, review_id, request_id, kind, status, scope, cost_estimate, active_key)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, in.ID, tenantID, in.ReviewID, in.RequestID, string(in.Kind), string(status), in.Scope, in.CostEstimate, key); err != nil {
			if isDuplicateKey(err, "uq_outcome_review_runs_active") {
				cur, ok, qerr := r.activeIn(ctx, db, tenantID, in.RequestID, in.Kind)
				if qerr == nil && ok {
					return domain.ErrReviewRunInProgress(cur.ID)
				}
				return domain.ErrReviewRunInProgress("unknown")
			}
			return err
		}
		var err error
		out, err = scanRun(db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM outcome_review_runs WHERE tenant_id = ? AND id = ?`, tenantID, in.ID), tenantID)
		return err
	})
	return out, err
}

func (r *OutcomeReviewRunRepository) activeIn(ctx context.Context, db dbExecer, tenantID, requestID string, kind usecase.ReviewRunKind) (usecase.ReviewRun, bool, error) {
	out, err := scanRun(db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM outcome_review_runs WHERE tenant_id = ? AND active_key = ?`,
		tenantID, activeKey(tenantID, requestID, kind)), tenantID)
	if errors.Is(err, sql.ErrNoRows) {
		return usecase.ReviewRun{}, false, nil
	}
	return out, err == nil, err
}

func (r *OutcomeReviewRunRepository) Finish(ctx context.Context, id string, status usecase.ReviewRunStatus, errorCode, costActual string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		res, err := db.ExecContext(ctx, `UPDATE outcome_review_runs SET status = ?, error_code = ?, cost_actual = ?, active_key = NULL, finished_at = CURRENT_TIMESTAMP(6)
			WHERE tenant_id = ? AND id = ? AND active_key IS NOT NULL`, string(status), errorCode, costActual, tenantID, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return domain.ErrReviewNotFound(id)
		}
		return nil
	})
}

func (r *OutcomeReviewRunRepository) Get(ctx context.Context, id string) (usecase.ReviewRun, error) {
	var out usecase.ReviewRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		var err error
		out, err = scanRun(db.QueryRowContext(ctx, `SELECT `+runColumns+` FROM outcome_review_runs WHERE tenant_id = ? AND id = ?`, tenantID, id), tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrReviewNotFound(id)
		}
		return err
	})
	return out, err
}

func (r *OutcomeReviewRunRepository) ActiveForRequest(ctx context.Context, requestID string, kind usecase.ReviewRunKind) (usecase.ReviewRun, bool, error) {
	var out usecase.ReviewRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		out, found, err = r.activeIn(ctx, db, tenantID, requestID, kind)
		return err
	})
	return out, found, err
}

func (r *OutcomeReviewRunRepository) ListByReview(ctx context.Context, reviewID string) ([]usecase.ReviewRun, error) {
	var out []usecase.ReviewRun
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(reviewID) {
			return nil
		}
		rows, err := db.QueryContext(ctx, `SELECT `+runColumns+` FROM outcome_review_runs WHERE tenant_id = ? AND review_id = ? ORDER BY started_at DESC, id DESC`, tenantID, reviewID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			run, err := scanRun(rows, tenantID)
			if err != nil {
				return err
			}
			out = append(out, run)
		}
		return rows.Err()
	})
	return out, err
}

func (r *OutcomeReviewRunRepository) CountSince(ctx context.Context, requestID string, kind usecase.ReviewRunKind, since time.Time) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outcome_review_runs WHERE tenant_id = ? AND request_id = ? AND kind = ? AND started_at >= ?`,
			tenantID, requestID, string(kind), since.UTC()).Scan(&n)
	})
	return n, err
}
