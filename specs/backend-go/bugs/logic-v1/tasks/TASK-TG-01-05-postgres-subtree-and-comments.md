# TASK-TG-01-05: Postgres — `GetSubtree`, `GetSubtreeWithChildPercents`, `BatchUpdateProgress`, comments repository

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/postgres/subtree.go` (new), `backend-go/services/task-service/internal/adapter/postgres/comments.go` (new)
**Depends on:** TASK-TG-01-04
**Status:** `[ ]` TODO

---

## Context

`GetAncestors` (`repository.go:101-143`) walks `parent_id` upward via one
`WITH RECURSIVE` query. `GetSubtree` needs the mirror-image downward walk;
`RecalculateProgress` needs a variant that also returns each node's depth
and its direct children's current `progress_percent`. Neither exists yet,
and `task.task_comments` has no repository at all.

## Changes to make

Create `backend-go/services/task-service/internal/adapter/postgres/subtree.go`:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
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

// subtreeProgressRow is GetSubtreeWithChildPercents's per-node result: the
// task itself, its depth (root=0), and its direct children's CURRENT
// progress_percent values as persisted — usecase.RecalculateProgress
// recomputes from these, deepest-first.
type subtreeProgressRow struct {
	Task          domain.Task
	Depth         int
	ChildPercents []int
}

// GetSubtreeWithChildPercents mirrors GetSubtree's WITH RECURSIVE shape but
// orders DEEPEST-FIRST (ORDER BY depth DESC) and folds in each node's
// direct children's progress_percent via a correlated subquery — one query,
// no N+1, per task-service.md §8's "one WITH RECURSIVE aggregate rather
// than N+1 fetches" NFR.
func (r *Repository) GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]subtreeProgressRow, error) {
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

	var out []subtreeProgressRow
	for rows.Next() {
		// scanTask's columns plus depth + []int32 — Scan directly here since
		// scanTask expects to own the whole row.
		var row subtreeProgressRow
		t, err := scanTaskAndTrailing(rows, &row.Depth, &row.ChildPercents)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan subtree progress row: %w", err)
		}
		row.Task = t
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
```

Note: `prefixedTaskColumns(alias string) string` and
`scanTaskAndTrailing(row rowScanner, extra ...any) (domain.Task, error)` are
small helpers `TASK-TG-01-04` should also add alongside `taskColumns`/
`scanTask` (a `t.`-prefixed column list for the recursive term, and a scan
variant that reads the fixed `Task` columns plus caller-supplied trailing
`dest` args) — add them there if not already present when this task starts.

Create `backend-go/services/task-service/internal/adapter/postgres/comments.go`:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func (r *Repository) AddComment(ctx context.Context, tenantID string, c domain.TaskComment) (domain.TaskComment, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO task.task_comments (id, tenant_id, task_id, author_id, content)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
		RETURNING id, author_id, content, created_at
	`, tenantID, c.TaskID, c.AuthorID, c.Content)
	var out domain.TaskComment
	out.TaskID = c.TaskID
	if err := row.Scan(&out.ID, &out.AuthorID, &out.Content, &out.CreatedAt); err != nil {
		return domain.TaskComment{}, fmt.Errorf("postgres: insert task comment: %w", err)
	}
	return out, nil
}

// ListComments is a plain tenant+task-scoped SELECT, cursor-paginated
// identically to Repository.List (id-based keyset pagination).
func (r *Repository) ListComments(ctx context.Context, tenantID, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, author_id, content, created_at
		FROM task.task_comments
		WHERE tenant_id = $1 AND task_id = $2 AND ($3 = '' OR id::text > $3)
		ORDER BY created_at, id
		LIMIT $4
	`, tenantID, taskID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query task comments: %w", err)
	}
	defer rows.Close()

	var out []domain.TaskComment
	for rows.Next() {
		var c domain.TaskComment
		c.TaskID = taskID
		if err := rows.Scan(&c.ID, &c.AuthorID, &c.Content, &c.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("postgres: scan task comment row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate task comment rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}
```

Add a `CommentRepository` port to
`backend-go/services/task-service/internal/usecase/ports.go`:

```go
type CommentRepository interface {
	AddComment(ctx context.Context, tenantID string, c domain.TaskComment) (domain.TaskComment, error)
	ListComments(ctx context.Context, tenantID, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/postgres/... -run 'TestGetSubtree|TestBatchUpdateProgress|TestComments' -v
```

Expected: `GetSubtree` against a real multi-branch tree returns every
descendant plus its `depends_on` edges; `GetSubtreeWithChildPercents`
returns deepest-first with correct child percent arrays;
`BatchUpdateProgress` updates every row in one call; `ListComments`
cursor-pagination round-trips correctly.
