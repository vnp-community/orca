package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type TaskRunOutcomeRepository struct {
	*Repository
}

func NewTaskRunOutcomeRepository(r *Repository) *TaskRunOutcomeRepository {
	return &TaskRunOutcomeRepository{Repository: r}
}

var _ usecase.TaskRunOutcomeRepository = (*TaskRunOutcomeRepository)(nil)

// Insert relies on the unique indexes (event_id, and once-per-container) to drop repeats; no target is named so either one counts.
func (r *TaskRunOutcomeRepository) Insert(ctx context.Context, o domain.TaskRunOutcome) (bool, error) {
	var once any
	if o.Outcome.CompletesContainer() {
		once = 1
	}
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `INSERT INTO request.task_run_outcomes
			(id, tenant_id, request_id, task_id, container_id, outcome, cause, execution_link_id, error_message, event_id, occurred_at, once)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT DO NOTHING`,
			o.ID, tenantID, o.RequestID, o.TaskID, nullIfEmpty(o.ContainerID), string(o.Outcome), o.Cause,
			nullIfEmpty(o.ExecutionLinkID), o.ErrorMessage, o.EventID, o.OccurredAt.UTC(), once)
		if err != nil {
			return fmt.Errorf("postgres: insert task run outcome: %w", err)
		}
		inserted = tag.RowsAffected() == 1
		return nil
	})
	return inserted, err
}

func (r *TaskRunOutcomeRepository) CountFailed(ctx context.Context, taskID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT count(*) FROM request.task_run_outcomes
			WHERE tenant_id = $1 AND task_id = $2::uuid AND outcome = 'failed' AND cause <> $3`, tenantID, taskID, domain.CauseDispatchError).Scan(&n)
	})
	return n, err
}

func (r *TaskRunOutcomeRepository) LatestFailed(ctx context.Context, taskIDs []string) (map[string]domain.TaskRunOutcome, error) {
	out := map[string]domain.TaskRunOutcome{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT DISTINCT ON (task_id) id, request_id, task_id, cause, error_message, event_id, occurred_at
			FROM request.task_run_outcomes WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) AND outcome = 'failed'
			ORDER BY task_id, occurred_at DESC, id DESC`, tenantID, taskIDs)
		if err != nil {
			return fmt.Errorf("postgres: latest failed outcomes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			o := domain.TaskRunOutcome{TenantID: tenantID, Outcome: domain.OutcomeFailed}
			if err := rows.Scan(&o.ID, &o.RequestID, &o.TaskID, &o.Cause, &o.ErrorMessage, &o.EventID, &o.OccurredAt); err != nil {
				return err
			}
			o.OccurredAt = o.OccurredAt.UTC()
			out[o.TaskID] = o
		}
		return rows.Err()
	})
	return out, err
}

func (r *TaskRunOutcomeRepository) LastEventAt(ctx context.Context, requestID string) (time.Time, error) {
	var at *time.Time
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT max(occurred_at) FROM request.task_run_outcomes WHERE tenant_id = $1 AND request_id = $2::uuid`, tenantID, requestID).Scan(&at)
	})
	if err != nil || at == nil {
		return time.Time{}, err
	}
	return at.UTC(), nil
}

func (r *TaskRunOutcomeRepository) Exists(ctx context.Context, taskID string, outcome domain.Outcome) (bool, error) {
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM request.task_run_outcomes WHERE tenant_id = $1 AND task_id = $2::uuid AND outcome = $3)`,
			tenantID, taskID, string(outcome)).Scan(&found)
	})
	return found, err
}

func (r *TaskRunOutcomeRepository) DispatchRetrySince(ctx context.Context, taskID string) (time.Time, bool, error) {
	var since *time.Time
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT min(occurred_at) FROM request.task_run_outcomes
			WHERE tenant_id = $1 AND task_id = $2::uuid AND outcome = 'failed' AND cause = $3
			AND occurred_at > COALESCE((SELECT max(occurred_at) FROM request.task_run_outcomes WHERE tenant_id = $1 AND task_id = $2::uuid AND outcome = 'started'), '-infinity'::timestamptz)`,
			tenantID, taskID, domain.CauseDispatchError).Scan(&since)
	})
	if err != nil || since == nil {
		return time.Time{}, false, err
	}
	return since.UTC(), true, nil
}

func (r *TaskRunOutcomeRepository) LatestDispatchError(ctx context.Context, taskID string) (domain.TaskRunOutcome, bool, error) {
	var o domain.TaskRunOutcome
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT id, request_id, task_id, error_message, event_id, occurred_at FROM request.task_run_outcomes
			WHERE tenant_id = $1 AND task_id = $2::uuid AND outcome = 'failed' AND cause = $3 ORDER BY occurred_at DESC, id DESC LIMIT 1`,
			tenantID, taskID, domain.CauseDispatchError)
		if err != nil {
			return err
		}
		defer rows.Close()
		if rows.Next() {
			found = true
			o = domain.TaskRunOutcome{TenantID: tenantID, Outcome: domain.OutcomeFailed, Cause: domain.CauseDispatchError}
			if err := rows.Scan(&o.ID, &o.RequestID, &o.TaskID, &o.ErrorMessage, &o.EventID, &o.OccurredAt); err != nil {
				return err
			}
			o.OccurredAt = o.OccurredAt.UTC()
		}
		return rows.Err()
	})
	return o, found, err
}
