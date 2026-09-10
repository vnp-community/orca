package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// GetSubtree walks tasks.parent_id DOWNWARD from rootID via one
// WITH RECURSIVE query — the mirror image of GetAncestors's upward walk
// (repository.go). Returns every task in the subtree (including the root)
// plus every depends_on edge whose FromTaskID is one of those tasks, so the
// caller (usecase.GetSubtree) can filter both by access without a second
// round trip. maxDepth <= 0 means domain.DefaultMaxAncestorDepth.
func (r *Repository) GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error) {
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxAncestorDepth
	}
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT `+taskColumns+`, 0 AS depth
			FROM task.tasks
			WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT `+prefixedTaskColumns("t")+`, s.depth + 1
			FROM task.tasks t
			JOIN subtree s ON t.parent_id = s.id
			WHERE s.depth + 1 < $3
		)
		SELECT `+taskColumns+` FROM subtree ORDER BY depth
	`, tenantID, rootID, maxDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: query subtree: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	var ids []string
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("postgres: scan subtree row: %w", err)
		}
		tasks = append(tasks, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("postgres: iterate subtree rows: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil, fmt.Errorf("postgres: task %s not found while resolving subtree", rootID)
	}

	edgeRows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND edge_type = 'depends_on' AND from_task_id = ANY($2)
	`, tenantID, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: query subtree depends_on edges: %w", err)
	}
	defer edgeRows.Close()
	edges, err := scanEdges(edgeRows)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: scan subtree edges: %w", err)
	}
	return tasks, edges, nil
}

// GetSubtreeWithChildPercents mirrors GetSubtree's WITH RECURSIVE shape but
// orders DEEPEST-FIRST (ORDER BY depth DESC) and folds in each node's
// direct children's progress_percent via a correlated subquery — one query,
// no N+1, per task-service.md §8's "one WITH RECURSIVE aggregate rather
// than N+1 fetches" NFR. Returns usecase.SubtreeProgressNode directly so
// RecalculateProgress can consume it via the TaskRepository port without an
// adapter-package type leaking across the boundary.
func (r *Repository) GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]usecase.SubtreeProgressNode, error) {
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT `+taskColumns+`, 0 AS depth
			FROM task.tasks
			WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT `+prefixedTaskColumns("t")+`, s.depth + 1
			FROM task.tasks t
			JOIN subtree s ON t.parent_id = s.id
		)
		SELECT `+taskColumns+`, depth,
			COALESCE((SELECT array_agg(c.progress_percent) FROM task.tasks c WHERE c.parent_id = subtree.id), '{}')
		FROM subtree
		ORDER BY depth DESC
	`, tenantID, rootID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query subtree with child percents: %w", err)
	}
	defer rows.Close()

	var out []usecase.SubtreeProgressNode
	for rows.Next() {
		// scanTask's columns plus depth + []int32 — Scan directly here since
		// scanTask expects to own the whole row.
		var row usecase.SubtreeProgressNode
		var childPercents32 []int32
		t, err := scanTaskAndTrailing(rows, &row.Depth, &childPercents32)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan subtree progress row: %w", err)
		}
		row.Task = t
		row.ChildPercents = make([]int, len(childPercents32))
		for i, p := range childPercents32 {
			row.ChildPercents[i] = int(p)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// BatchUpdateProgress persists every (taskID -> progress_percent) pair in
// updates via one UPDATE ... FROM (VALUES ...) statement — the batching
// usecase.RecalculateProgress's regression test asserts (one call, not one
// per node).
func (r *Repository) BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error {
	if len(updates) == 0 {
		return nil
	}
	ids := make([]string, 0, len(updates))
	percents := make([]int32, 0, len(updates))
	for id, p := range updates {
		ids = append(ids, id)
		percents = append(percents, int32(p))
	}
	_, err := r.db.Exec(ctx, `
		UPDATE task.tasks t SET progress_percent = v.percent, updated_at = now()
		FROM (SELECT unnest($2::uuid[]) AS id, unnest($3::int[]) AS percent) v
		WHERE t.tenant_id = $1 AND t.id = v.id
	`, tenantID, ids, percents)
	if err != nil {
		return fmt.Errorf("postgres: batch update progress: %w", err)
	}
	return nil
}
