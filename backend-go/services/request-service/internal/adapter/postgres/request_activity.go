package postgres

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

// activityUnion lists, as the same twelve columns, every thing that happened to one Request. $1 is the tenant and $2 the Request.
// Each branch reads one table the service already owns; ids carry a suffix when a table yields two lines (a request and its
// decision), so (occurred_at, id) is a total order for the keyset page.
const activityUnion = `
SELECT 'request.created'::text AS kind, id::text AS id, created_at AS occurred_at, actor_kind::text AS actor_kind, COALESCE(actor_id, '')::text AS actor_id,
       'request'::text AS subject_type, request_id::text AS subject_id, ''::text AS a, ''::text AS b, ''::text AS c, ''::text AS d, 0::bigint AS n
  FROM request.request_revisions WHERE tenant_id = $1 AND request_id = $2::uuid AND cause = 'created'
UNION ALL
SELECT CASE WHEN from_type IS NULL THEN 'request.classified' ELSE 'request.type_changed' END, request_id::text || '@' || extract(epoch FROM at)::text, at,
       CASE WHEN actor_kind = 'agent' THEN 'ai' ELSE COALESCE(actor_kind, 'system') END::text, actor_id::text, 'request', request_id::text, COALESCE(from_type, ''), to_type, reason, '', 0
  FROM request.request_type_history WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
SELECT 'analysis.generated', id::text || ':solution', created_at, 'system', '', 'solution', id::text, kind, status, '', '', COALESCE(seq, 0)::bigint
  FROM request.solutions WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
-- The reviewer's feedback is user content (C9): it is tested here and never selected, so the line cannot carry it.
-- A run the system started after a clarification carries a marker, not a reviewer's ask.
SELECT 'analysis.revision_requested', id::text || ':revision', started_at, 'user', COALESCE(actor_id::text, ''), 'solution', COALESCE(solution_id::text, ''), '', '', '', '', 0
  FROM request.analysis_runs WHERE tenant_id = $1 AND request_id = $2::uuid AND kind <> 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
-- The same rule for a Plan run: its feedback shares the column with the analysis runs, so each branch filters on kind.
SELECT 'plan.revision_requested', id::text || ':plan-revision', started_at, 'user', COALESCE(actor_id::text, ''), 'plan', '', '', '', '', '', 0
  FROM request.analysis_runs WHERE tenant_id = $1 AND request_id = $2::uuid AND kind = 'plan' AND feedback IS NOT NULL AND feedback <> '' AND feedback NOT LIKE 'clarification:%'
UNION ALL
SELECT 'clarification.asked', id::text || ':asked', created_at, 'system', created_by, 'clarification', id::text, source, status, resume_status, '', round::bigint
  FROM request.clarifications WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
SELECT 'clarification.answered', id::text || ':answered', answered_at, 'user', '', 'clarification', id::text, source, '', '', '', round::bigint
  FROM request.clarifications WHERE tenant_id = $1 AND request_id = $2::uuid AND answered_at IS NOT NULL
UNION ALL
SELECT 'approval.requested', id::text || ':requested', created_at, CASE WHEN requested_by = 'system' THEN 'system' ELSE 'user' END, requested_by,
       subject_type, subject_id, stage, status, '', '', 0
  FROM request.approvals WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
SELECT 'approval.decided', id::text || ':decided', decided_at, 'user', COALESCE(decided_by, ''), subject_type, subject_id, status, stage, comment, '', 0
  FROM request.approvals WHERE tenant_id = $1 AND request_id = $2::uuid AND decided_at IS NOT NULL AND status IN ('approved', 'rejected')
UNION ALL
SELECT 'phase.started', phase_task_id::text || ':start', started_at, CASE WHEN start_trigger = 'auto' THEN 'system' ELSE 'user' END, started_by::text,
       'phase', phase_task_id::text, start_trigger, '', '', '', 0
  FROM request.phase_starts WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
SELECT CASE WHEN outcome = 'succeeded' AND cause = 'user_update' THEN 'task.accepted'
            ELSE CASE outcome WHEN 'started' THEN 'task.started' WHEN 'succeeded' THEN 'task.succeeded' WHEN 'failed' THEN 'task.failed'
                              WHEN 'cancelled' THEN 'task.cancelled' WHEN 'phase_done' THEN 'phase.done' ELSE 'plan.done' END END,
       id::text, occurred_at, CASE WHEN outcome IN ('phase_done', 'plan_done') THEN 'system' ELSE 'agent' END, '',
       CASE outcome WHEN 'phase_done' THEN 'phase' WHEN 'plan_done' THEN 'plan' ELSE 'task' END, task_id::text, outcome, cause, error_message,
       COALESCE(container_id::text, ''),
       -- Attempt numbers count runs that used the retry budget; a dispatch that never reached an agent is not an attempt.
       (CASE WHEN outcome = 'failed' THEN (CASE WHEN cause = 'dispatch_error' THEN 0 ELSE SUM(CASE WHEN outcome = 'failed' AND cause <> 'dispatch_error' THEN 1 ELSE 0 END) OVER (PARTITION BY task_id ORDER BY occurred_at, id) END)
             WHEN outcome IN ('phase_done', 'plan_done') THEN 0 ELSE SUM(CASE WHEN outcome = 'failed' AND cause <> 'dispatch_error' THEN 1 ELSE 0 END) OVER (PARTITION BY task_id ORDER BY occurred_at, id) + 1 END)::bigint
  FROM request.task_run_outcomes WHERE tenant_id = $1 AND request_id = $2::uuid
UNION ALL
SELECT CASE action WHEN 'returned' THEN 'request.returned' WHEN 'reopened' THEN 'request.reopened' ELSE 'request.cancelled' END, id::text, at, actor_kind,
       COALESCE(actor_id::text, ''), 'request', request_id::text, action, COALESCE(stage, ''), COALESCE(category, ''), reason, 0
  FROM request.request_return_history WHERE tenant_id = $1 AND request_id = $2::uuid
`

// ListActivity reads the newest lines of one Request first. Filters that are plain columns run in SQL so a page is full when
// the history is long; the severity filter needs the lines around a row and is applied by the use case.
func (r *RequestActivityRepository) ListActivity(ctx context.Context, requestID string, q domain.ActivityQuery) ([]domain.ActivityRow, error) {
	var out []domain.ActivityRow
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		args := []any{tenantID, requestID}
		var where []string
		add := func(cond string, v any) {
			args = append(args, v)
			where = append(where, fmt.Sprintf(cond, len(args)))
		}
		if len(q.Kinds) > 0 {
			add("kind = ANY($%d::text[])", q.Kinds)
		}
		if q.ActorKind != "" {
			add("actor_kind = $%d", q.ActorKind)
		}
		if q.SubjectID != "" {
			add("subject_id = $%d", q.SubjectID)
		}
		if q.Since != nil {
			add("occurred_at >= $%d", q.Since.UTC())
		}
		if q.Before != nil {
			args = append(args, q.Before.At.UTC(), q.Before.ID)
			where = append(where, fmt.Sprintf("(occurred_at, id) < ($%d, $%d)", len(args)-1, len(args)))
		}
		sqlText := `SELECT kind, id, occurred_at, actor_kind, actor_id, subject_type, subject_id, a, b, c, d, n FROM (` + activityUnion + `) u`
		if len(where) > 0 {
			sqlText += ` WHERE ` + strings.Join(where, " AND ")
		}
		limit := q.Limit
		if limit <= 0 {
			limit = 51
		}
		args = append(args, limit)
		sqlText += fmt.Sprintf(` ORDER BY occurred_at DESC, id DESC LIMIT $%d`, len(args))

		rows, err := db.Query(ctx, sqlText, args...)
		if err != nil {
			return fmt.Errorf("postgres: list request activity: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var row domain.ActivityRow
			if err := rows.Scan(&row.Kind, &row.ID, &row.OccurredAt, &row.ActorKind, &row.ActorID, &row.SubjectType, &row.SubjectID, &row.A, &row.B, &row.C, &row.D, &row.N); err != nil {
				return fmt.Errorf("postgres: scan request activity: %w", err)
			}
			row.OccurredAt = row.OccurredAt.UTC()
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}
