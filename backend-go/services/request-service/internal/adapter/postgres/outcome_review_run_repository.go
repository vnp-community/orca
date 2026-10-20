package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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

const runColumns = `id::text, review_id::text, request_id::text, kind, status, scope, agent_run_ref, cost_estimate, cost_actual, error_code, started_at, finished_at, reason, COALESCE(client_key, ''), COALESCE(created_by::text, '')`

func scanRun(row pgx.Row, tenantID string) (usecase.ReviewRun, error) {
	var r usecase.ReviewRun
	var kind, status string
	if err := row.Scan(&r.ID, &r.ReviewID, &r.RequestID, &kind, &status, &r.Scope, &r.AgentRunRef, &r.CostEstimate, &r.CostActual, &r.ErrorCode, &r.StartedAt, &r.FinishedAt, &r.Reason, &r.ClientKey, &r.CreatedBy); err != nil {
		return usecase.ReviewRun{}, err
	}
	r.TenantID, r.Kind, r.Status, r.StartedAt = tenantID, usecase.ReviewRunKind(kind), usecase.ReviewRunStatus(status), r.StartedAt.UTC()
	if r.FinishedAt != nil {
		t := r.FinishedAt.UTC()
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
		// ON CONFLICT keeps the transaction usable: a unique violation would abort it and the active run could not be read back.
		row := db.QueryRow(ctx, `INSERT INTO request.outcome_review_runs (id, tenant_id, review_id, request_id, kind, status, scope, cost_estimate, active_key, reason, client_key, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT (active_key) DO NOTHING RETURNING `+runColumns,
			in.ID, tenantID, in.ReviewID, in.RequestID, string(in.Kind), string(status), in.Scope, in.CostEstimate, key, in.Reason, nullIfEmpty(in.ClientKey), nullIfEmpty(in.CreatedBy))
		var err error
		out, err = scanRun(row, tenantID)
		if isUniqueViolation(err, "uq_outcome_review_runs_client") {
			return domain.ErrReviewDuplicateRequest()
		}
		if errors.Is(err, pgx.ErrNoRows) {
			cur, ok, qerr := r.activeIn(ctx, db, tenantID, in.RequestID, in.Kind)
			if qerr == nil && ok {
				return domain.ErrReviewRunInProgress(cur.ID)
			}
			return domain.ErrReviewRunInProgress("unknown")
		}
		return err
	})
	return out, err
}

func (r *OutcomeReviewRunRepository) activeIn(ctx context.Context, db dbExecer, tenantID, requestID string, kind usecase.ReviewRunKind) (usecase.ReviewRun, bool, error) {
	out, err := scanRun(db.QueryRow(ctx, `SELECT `+runColumns+` FROM request.outcome_review_runs WHERE tenant_id = $1 AND active_key = $2`,
		tenantID, activeKey(tenantID, requestID, kind)), tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return usecase.ReviewRun{}, false, nil
	}
	return out, err == nil, err
}

func (r *OutcomeReviewRunRepository) Finish(ctx context.Context, id string, status usecase.ReviewRunStatus, errorCode, costActual string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(id) {
			return domain.ErrReviewNotFound(id)
		}
		tag, err := db.Exec(ctx, `UPDATE request.outcome_review_runs SET status = $3, error_code = $4, cost_actual = $5, active_key = NULL, finished_at = clock_timestamp()
			WHERE tenant_id = $1 AND id = $2 AND active_key IS NOT NULL`, tenantID, id, string(status), errorCode, costActual)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
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
		out, err = scanRun(db.QueryRow(ctx, `SELECT `+runColumns+` FROM request.outcome_review_runs WHERE tenant_id = $1 AND id = $2`, tenantID, id), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
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
		rows, err := db.Query(ctx, `SELECT `+runColumns+` FROM request.outcome_review_runs WHERE tenant_id = $1 AND review_id = $2 ORDER BY started_at DESC, id DESC`, tenantID, reviewID)
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
		return db.QueryRow(ctx, `SELECT COUNT(*) FROM request.outcome_review_runs WHERE tenant_id = $1 AND request_id = $2 AND kind = $3 AND started_at >= $4`,
			tenantID, requestID, string(kind), since.UTC()).Scan(&n)
	})
	return n, err
}

func (r *OutcomeReviewRunRepository) CountRegenerations(ctx context.Context, requestID string, since time.Time) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		return db.QueryRow(ctx, `SELECT COUNT(*) FROM request.outcome_review_runs WHERE tenant_id = $1 AND request_id = $2 AND reason <> '' AND started_at >= $3`,
			tenantID, requestID, since.UTC()).Scan(&n)
	})
	return n, err
}

func (r *OutcomeReviewRunRepository) CountActive(ctx context.Context) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT COUNT(*) FROM request.outcome_review_runs WHERE tenant_id = $1 AND active_key IS NOT NULL`, tenantID).Scan(&n)
	})
	return n, err
}

func (r *OutcomeReviewRunRepository) FindByClientKey(ctx context.Context, requestID, clientKey string) (usecase.ReviewRun, bool, error) {
	var out usecase.ReviewRun
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) || clientKey == "" {
			return nil
		}
		var err error
		out, err = scanRun(db.QueryRow(ctx, `SELECT `+runColumns+` FROM request.outcome_review_runs WHERE tenant_id = $1 AND request_id = $2 AND client_key = $3`, tenantID, requestID, clientKey), tenantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return out, found, err
}
