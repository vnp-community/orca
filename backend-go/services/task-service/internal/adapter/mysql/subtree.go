package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// GetSubtree walks tasks.parent_id DOWNWARD from rootID via one
// WITH RECURSIVE query — MySQL 8.0.1+ supports recursive CTEs, so this
// translates directly from internal/adapter/postgres's identical shape.
func (r *Repository) GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error) {
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxAncestorDepth
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT `+taskColumns+`, 0 AS depth
			FROM tasks
			WHERE tenant_id = ? AND id = ?

			UNION ALL

			SELECT `+prefixedTaskColumns("t")+`, s.depth + 1
			FROM tasks t
			JOIN subtree s ON t.parent_id = s.id
			WHERE s.depth + 1 < ?
		)
		SELECT `+taskColumns+` FROM subtree ORDER BY depth
	`, tenantID, rootID, maxDepth)
	if err != nil {
		return nil, nil, fmt.Errorf("mysql: query subtree: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	var ids []string
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("mysql: scan subtree row: %w", err)
		}
		tasks = append(tasks, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("mysql: iterate subtree rows: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil, fmt.Errorf("mysql: task %s not found while resolving subtree", rootID)
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, tenantID)
	for _, id := range ids {
		args = append(args, id)
	}
	edgeRows, err := r.db.QueryContext(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task_edges
		WHERE tenant_id = ? AND edge_type = 'depends_on' AND from_task_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("mysql: query subtree depends_on edges: %w", err)
	}
	defer edgeRows.Close()
	edges, err := scanEdges(edgeRows)
	if err != nil {
		return nil, nil, fmt.Errorf("mysql: scan subtree edges: %w", err)
	}
	return tasks, edges, nil
}

// GetSubtreeWithChildPercents mirrors GetSubtree's WITH RECURSIVE shape but
// orders DEEPEST-FIRST and folds in each node's direct children's current
// progress_percent via a correlated subquery — JSON_ARRAYAGG is MySQL's
// equivalent of Postgres's array_agg (both return NULL, not an empty
// collection, when the subquery matches zero rows — COALESCE'd to an empty
// JSON array here, matching Postgres's COALESCE(..., '{}')).
func (r *Repository) GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]usecase.SubtreeProgressNode, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT `+taskColumns+`, 0 AS depth
			FROM tasks
			WHERE tenant_id = ? AND id = ?

			UNION ALL

			SELECT `+prefixedTaskColumns("t")+`, s.depth + 1
			FROM tasks t
			JOIN subtree s ON t.parent_id = s.id
		)
		SELECT `+taskColumns+`, depth,
			COALESCE((SELECT JSON_ARRAYAGG(c.progress_percent) FROM tasks c WHERE c.parent_id = subtree.id), JSON_ARRAY())
		FROM subtree
		ORDER BY depth DESC
	`, tenantID, rootID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query subtree with child percents: %w", err)
	}
	defer rows.Close()

	var out []usecase.SubtreeProgressNode
	for rows.Next() {
		var h taskRowHolder
		var depth int
		var childPercentsJSON []byte
		dest := append(h.dest(), &depth, &childPercentsJSON)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("mysql: scan subtree progress row: %w", err)
		}
		t, err := h.toTask()
		if err != nil {
			return nil, fmt.Errorf("mysql: build subtree progress task: %w", err)
		}

		var childPercents []int
		if len(childPercentsJSON) > 0 {
			if err := json.Unmarshal(childPercentsJSON, &childPercents); err != nil {
				return nil, fmt.Errorf("mysql: unmarshal child percents: %w", err)
			}
		}
		out = append(out, usecase.SubtreeProgressNode{Task: t, Depth: depth, ChildPercents: childPercents})
	}
	return out, rows.Err()
}

// BatchUpdateProgress persists every (taskID -> progress_percent) pair in
// updates via one UPDATE ... CASE statement — MySQL has no `UPDATE ... FROM
// (SELECT unnest(...))` (that's Postgres-specific set-returning-function
// syntax); a single `CASE id WHEN ? THEN ? ...` UPDATE achieves the same
// "one call, not one per node" batching guarantee
// usecase.RecalculateProgress's regression test asserts, just via a
// different SQL shape.
func (r *Repository) BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error {
	if len(updates) == 0 {
		return nil
	}
	var caseSQL strings.Builder
	caseSQL.WriteString("UPDATE tasks SET progress_percent = CASE id ")
	args := make([]any, 0, len(updates)*2+len(updates)+1)
	ids := make([]string, 0, len(updates))
	for id, percent := range updates {
		caseSQL.WriteString("WHEN ? THEN ? ")
		args = append(args, id, percent)
		ids = append(ids, id)
	}
	caseSQL.WriteString("END, updated_at = NOW(6) WHERE tenant_id = ? AND id IN (")
	args = append(args, tenantID)
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	caseSQL.WriteString(strings.Join(placeholders, ","))
	caseSQL.WriteString(")")

	_, err := r.db.ExecContext(ctx, caseSQL.String(), args...)
	if err != nil {
		return fmt.Errorf("mysql: batch update progress: %w", err)
	}
	return nil
}
