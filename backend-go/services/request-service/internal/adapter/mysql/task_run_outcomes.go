package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// Insert drops repeats through the unique keys (event_id, once-per-container); the no-op ON DUPLICATE KEY UPDATE
// reports 0 affected rows for them without hiding other errors the way INSERT IGNORE would.
func (r *TaskRunOutcomeRepository) Insert(ctx context.Context, o domain.TaskRunOutcome) (bool, error) {
	var once any
	if o.Outcome.CompletesContainer() {
		once = 1
	}
	inserted := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `INSERT INTO task_run_outcomes
			(id, tenant_id, request_id, task_id, container_id, outcome, cause, execution_link_id, error_message, event_id, occurred_at, once)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE id = id`,
			o.ID, tenantID, o.RequestID, o.TaskID, nullIfEmpty(o.ContainerID), string(o.Outcome), o.Cause,
			nullIfEmpty(o.ExecutionLinkID), o.ErrorMessage, o.EventID, o.OccurredAt.UTC(), once)
		if err != nil {
			return fmt.Errorf("mysql: insert task run outcome: %w", err)
		}
		n, err := res.RowsAffected()
		inserted = n == 1
		return err
	})
	return inserted, err
}

func (r *TaskRunOutcomeRepository) CountFailed(ctx context.Context, taskID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_run_outcomes
			WHERE tenant_id = ? AND task_id = ? AND outcome = 'failed' AND cause <> ?`, tenantID, taskID, domain.CauseDispatchError).Scan(&n)
	})
	return n, err
}

func (r *TaskRunOutcomeRepository) LatestFailed(ctx context.Context, taskIDs []string) (map[string]domain.TaskRunOutcome, error) {
	out := map[string]domain.TaskRunOutcome{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		args := []any{tenantID}
		ph := make([]string, len(taskIDs))
		for i, id := range taskIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT id, request_id, task_id, cause, error_message, event_id, occurred_at FROM (
				SELECT o.*, ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY occurred_at DESC, id DESC) AS rn
				FROM task_run_outcomes o WHERE tenant_id = ? AND task_id IN (%s) AND outcome = 'failed') ranked WHERE rn = 1`, strings.Join(ph, ",")), args...)
		if err != nil {
			return fmt.Errorf("mysql: latest failed outcomes: %w", err)
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
	var at sql.NullTime
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT MAX(occurred_at) FROM task_run_outcomes WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Scan(&at)
	})
	if err != nil || !at.Valid {
		return time.Time{}, err
	}
	return at.Time.UTC(), nil
}

func (r *TaskRunOutcomeRepository) Exists(ctx context.Context, taskID string, outcome domain.Outcome) (bool, error) {
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM task_run_outcomes WHERE tenant_id = ? AND task_id = ? AND outcome = ?)`,
			tenantID, taskID, string(outcome)).Scan(&found)
	})
	return found, err
}

func (r *TaskRunOutcomeRepository) DispatchRetrySince(ctx context.Context, taskID string) (time.Time, bool, error) {
	var since sql.NullTime
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		return db.QueryRowContext(ctx, `SELECT MIN(occurred_at) FROM task_run_outcomes
			WHERE tenant_id = ? AND task_id = ? AND outcome = 'failed' AND cause = ?
			AND occurred_at > COALESCE((SELECT MAX(occurred_at) FROM task_run_outcomes WHERE tenant_id = ? AND task_id = ? AND outcome = 'started'), '1970-01-01 00:00:01')`,
			tenantID, taskID, domain.CauseDispatchError, tenantID, taskID).Scan(&since)
	})
	if err != nil || !since.Valid {
		return time.Time{}, false, err
	}
	return since.Time.UTC(), true, nil
}

func (r *TaskRunOutcomeRepository) LatestDispatchError(ctx context.Context, taskID string) (domain.TaskRunOutcome, bool, error) {
	var o domain.TaskRunOutcome
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT id, request_id, task_id, error_message, event_id, occurred_at FROM task_run_outcomes
			WHERE tenant_id = ? AND task_id = ? AND outcome = 'failed' AND cause = ? ORDER BY occurred_at DESC, id DESC LIMIT 1`,
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
