package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type RequestActivityRepository struct {
	*Repository
}

func NewRequestActivityRepository(r *Repository) *RequestActivityRepository {
	return &RequestActivityRepository{Repository: r}
}

var _ usecase.RequestActivityReader = (*RequestActivityRepository)(nil)

// activityUnion lists, as the same twelve columns, every thing that happened to one Request; the Postgres adapter has the same
// query. MySQL has no row level security, so every branch binds the tenant. Placeholders: each branch takes (tenant, request).
const activityUnion = `
SELECT 'request.created' AS kind, id AS id, created_at AS occurred_at, actor_kind AS actor_kind, COALESCE(actor_id, '') AS actor_id,
       'request' AS subject_type, request_id AS subject_id, '' AS a, '' AS b, '' AS c, '' AS d, CAST(0 AS SIGNED) AS n
  FROM request_revisions WHERE tenant_id = ? AND request_id = ? AND cause = 'created'
UNION ALL
SELECT CASE WHEN from_type IS NULL THEN 'request.classified' ELSE 'request.type_changed' END, CONCAT(request_id, '@', UNIX_TIMESTAMP(at)), at,
       CASE WHEN actor_kind = 'agent' THEN 'ai' ELSE COALESCE(actor_kind, 'system') END, actor_id, 'request', request_id, COALESCE(from_type, ''), to_type, reason, '', CAST(0 AS SIGNED)
  FROM request_type_history WHERE tenant_id = ? AND request_id = ?
UNION ALL
SELECT 'analysis.generated', CONCAT(id, ':solution'), created_at, 'system', '', 'solution', id, kind, status, '', '', CAST(COALESCE(seq, 0) AS SIGNED)
  FROM solutions WHERE tenant_id = ? AND request_id = ?
UNION ALL
-- The reviewer's feedback is user content (C9): it is tested here and never selected, so the line cannot carry it.
-- A run the system started after a clarification carries a marker, not a reviewer's ask.
SELECT 'analysis.revision_requested', CONCAT(id, ':revision'), started_at, 'user', COALESCE(actor_id, ''), 'solution', COALESCE(solution_id, ''), '', '', '', '', CAST(0 AS SIGNED)
  FROM analysis_runs WHERE tenant_id = ? AND request_id = ? AND kind <> 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
-- The same rule for a Plan run: its feedback shares the column with the analysis runs, so each branch filters on kind.
SELECT 'plan.revision_requested', CONCAT(id, ':plan-revision'), started_at, 'user', COALESCE(actor_id, ''), 'plan', '', '', '', '', '', CAST(0 AS SIGNED)
  FROM analysis_runs WHERE tenant_id = ? AND request_id = ? AND kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
SELECT 'clarification.asked', CONCAT(id, ':asked'), created_at, 'system', created_by, 'clarification', id, source, status, resume_status, '', CAST(round AS SIGNED)
  FROM clarifications WHERE tenant_id = ? AND request_id = ?
UNION ALL
SELECT 'clarification.answered', CONCAT(id, ':answered'), answered_at, 'user', '', 'clarification', id, source, '', '', '', CAST(round AS SIGNED)
  FROM clarifications WHERE tenant_id = ? AND request_id = ? AND answered_at IS NOT NULL
UNION ALL
SELECT 'approval.requested', CONCAT(id, ':requested'), created_at, CASE WHEN requested_by = 'system' THEN 'system' ELSE 'user' END, requested_by,
       subject_type, subject_id, stage, status, '', '', CAST(0 AS SIGNED)
  FROM approvals WHERE tenant_id = ? AND request_id = ?
UNION ALL
SELECT 'approval.decided', CONCAT(id, ':decided'), decided_at, 'user', COALESCE(decided_by, ''), subject_type, subject_id, status, stage, comment, '', CAST(0 AS SIGNED)
  FROM approvals WHERE tenant_id = ? AND request_id = ? AND decided_at IS NOT NULL AND status IN ('approved', 'rejected')
UNION ALL
SELECT 'phase.started', CONCAT(phase_task_id, ':start'), started_at, CASE WHEN start_trigger = 'auto' THEN 'system' ELSE 'user' END, started_by,
       'phase', phase_task_id, start_trigger, '', '', '', CAST(0 AS SIGNED)
  FROM phase_starts WHERE tenant_id = ? AND request_id = ?
UNION ALL
SELECT CASE WHEN outcome = 'succeeded' AND cause = 'user_update' THEN 'task.accepted'
            ELSE CASE outcome WHEN 'started' THEN 'task.started' WHEN 'succeeded' THEN 'task.succeeded' WHEN 'failed' THEN 'task.failed'
                              WHEN 'cancelled' THEN 'task.cancelled' WHEN 'phase_done' THEN 'phase.done' ELSE 'plan.done' END END,
       id, occurred_at, CASE WHEN outcome IN ('phase_done', 'plan_done') THEN 'system' ELSE 'agent' END, '',
       CASE outcome WHEN 'phase_done' THEN 'phase' WHEN 'plan_done' THEN 'plan' ELSE 'task' END, task_id, outcome, cause, error_message,
       COALESCE(container_id, ''),
       -- Attempt numbers count runs that used the retry budget; a dispatch that never reached an agent is not an attempt.
       CAST(CASE WHEN outcome = 'failed' THEN (CASE WHEN cause = 'dispatch_error' THEN 0 ELSE SUM(CASE WHEN outcome = 'failed' AND cause <> 'dispatch_error' THEN 1 ELSE 0 END) OVER (PARTITION BY task_id ORDER BY occurred_at, id) END)
                 WHEN outcome IN ('phase_done', 'plan_done') THEN 0 ELSE SUM(CASE WHEN outcome = 'failed' AND cause <> 'dispatch_error' THEN 1 ELSE 0 END) OVER (PARTITION BY task_id ORDER BY occurred_at, id) + 1 END AS SIGNED)
  FROM task_run_outcomes WHERE tenant_id = ? AND request_id = ?
UNION ALL
SELECT CASE action WHEN 'returned' THEN 'request.returned' WHEN 'reopened' THEN 'request.reopened' ELSE 'request.cancelled' END, id, at, actor_kind,
       COALESCE(actor_id, ''), 'request', request_id, action, COALESCE(stage, ''), COALESCE(category, ''), reason, CAST(0 AS SIGNED)
  FROM request_return_history WHERE tenant_id = ? AND request_id = ?
`

const activityBranches = 12

// ListActivity reads the newest lines of one Request first; see the Postgres adapter for what runs in SQL and what does not.
func (r *RequestActivityRepository) ListActivity(ctx context.Context, requestID string, q domain.ActivityQuery) ([]domain.ActivityRow, error) {
	var out []domain.ActivityRow
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var args []any
		for i := 0; i < activityBranches; i++ {
			args = append(args, tenantID, requestID)
		}
		var where []string
		if len(q.Kinds) > 0 {
			marks := make([]string, len(q.Kinds))
			for i, k := range q.Kinds {
				marks[i] = "?"
				args = append(args, k)
			}
			where = append(where, "kind IN ("+strings.Join(marks, ",")+")")
		}
		if q.ActorKind != "" {
			where = append(where, "actor_kind = ?")
			args = append(args, q.ActorKind)
		}
		if q.SubjectID != "" {
			where = append(where, "subject_id = ?")
			args = append(args, q.SubjectID)
		}
		if q.Since != nil {
			where = append(where, "occurred_at >= ?")
			args = append(args, q.Since.UTC())
		}
		if q.Before != nil {
			where = append(where, "(occurred_at < ? OR (occurred_at = ? AND id < ?))")
			args = append(args, q.Before.At.UTC(), q.Before.At.UTC(), q.Before.ID)
		}
		sqlText := `SELECT kind, id, occurred_at, actor_kind, actor_id, subject_type, subject_id, a, b, c, d, n FROM (` + activityUnion + `) u`
		if len(where) > 0 {
			sqlText += ` WHERE ` + strings.Join(where, " AND ")
		}
		limit := q.Limit
		if limit <= 0 {
			limit = 51
		}
		sqlText += ` ORDER BY occurred_at DESC, id DESC LIMIT ?`
		args = append(args, limit)

		rows, err := db.QueryContext(ctx, sqlText, args...)
		if err != nil {
			return fmt.Errorf("mysql: list request activity: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var row domain.ActivityRow
			if err := rows.Scan(&row.Kind, &row.ID, &row.OccurredAt, &row.ActorKind, &row.ActorID, &row.SubjectType, &row.SubjectID, &row.A, &row.B, &row.C, &row.D, &row.N); err != nil {
				return fmt.Errorf("mysql: scan request activity: %w", err)
			}
			row.OccurredAt = row.OccurredAt.UTC()
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}
