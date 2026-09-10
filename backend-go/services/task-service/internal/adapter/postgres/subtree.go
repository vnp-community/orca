package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// RecalculateAncestorProgress implements usecase.TaskRepository's
// progress-cascade method: one WITH RECURSIVE UPDATE walks taskID's parent
// chain (tenant-scoped in both the anchor and recursive terms, same
// correction as GetAncestors's real precedent) and recomputes each
// ancestor's done_subtasks/total_subtasks from its direct children's
// current status — not a per-ancestor round trip. A no-op for a root task
// (the anchor row itself still gets its own counts refreshed).
func (r *Repository) RecalculateAncestorProgress(ctx context.Context, tenantID, taskID string) error {
	_, err := r.db.Exec(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM task.tasks WHERE tenant_id = $1 AND id = $2
			UNION ALL
			SELECT t.id, t.parent_id FROM task.tasks t
			JOIN ancestors a ON t.id = a.parent_id
			WHERE t.tenant_id = $1
		)
		UPDATE task.tasks SET
			done_subtasks  = (SELECT count(*) FROM task.tasks c WHERE c.tenant_id = $1 AND c.parent_id = task.tasks.id AND c.status = 'done'),
			total_subtasks = (SELECT count(*) FROM task.tasks c WHERE c.tenant_id = $1 AND c.parent_id = task.tasks.id)
		WHERE tenant_id = $1 AND id IN (SELECT id FROM ancestors)
	`, tenantID, taskID)
	if err != nil {
		return fmt.Errorf("postgres: recalculate ancestor progress: %w", err)
	}
	return nil
}

// GetSubtree implements usecase.TaskRepository's descendant-read method: one
// WITH RECURSIVE query walks task.tasks.parent_id DOWN from id (the inverse
// direction of GetAncestors's walk UP), tenant-scoped in both the anchor and
// recursive terms per this file's own convention (repository.go's header
// comment: RLS is the secondary backstop, explicit filtering is primary).
// The outer SELECT names every column explicitly (not `SELECT *`, unlike
// BE-SOL-001's own sketch) so column order stays independent of
// task.tasks's physical column order.
func (r *Repository) GetSubtree(ctx context.Context, tenantID, id string) ([]domain.Task, error) {
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id, tenant_id, title, status, parent_id, project_id, workflow_template_id,
				description, type, priority, labels, assignee_id, reporter_id, owner_id,
				due_date, estimated_hours, actual_hours, prompt_template, ai_context, ai_plan_json,
				visibility, worktree_id, agent_session_id, workflow_exec_id, done_subtasks, total_subtasks, share_token
			FROM task.tasks WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT t.id, t.tenant_id, t.title, t.status, t.parent_id, t.project_id, t.workflow_template_id,
				t.description, t.type, t.priority, t.labels, t.assignee_id, t.reporter_id, t.owner_id,
				t.due_date, t.estimated_hours, t.actual_hours, t.prompt_template, t.ai_context, t.ai_plan_json,
				t.visibility, t.worktree_id, t.agent_session_id, t.workflow_exec_id, t.done_subtasks, t.total_subtasks, t.share_token
			FROM task.tasks t
			JOIN subtree s ON t.parent_id = s.id
			WHERE t.tenant_id = $1
		)
		SELECT `+taskSelectColumns+`
		FROM subtree
	`, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: query subtree: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan subtree row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
