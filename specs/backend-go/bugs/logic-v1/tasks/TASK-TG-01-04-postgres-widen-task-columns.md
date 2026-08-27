# TASK-TG-01-04: Postgres — widen `Create`/`Get`/`List`/`Update` to read/write the new `tasks` columns

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-TG-01-02 (migration), TASK-TG-01-03 (domain fields)
**Status:** `[ ]` TODO

---

## Context

`Repository.Create`/`Get`/`List`/`Update` in `repository.go` only
read/write the original 6 columns. This task widens the column list on
each, and adds the narrow setter methods later usecases need
(`UpdateWorktreeID`, `UpdatePromptTemplate`, `UpdateAIPlanJSON`) — the
generic `Update` stays scoped to title/status (client field-mask edit) per
its existing doc comment; server-computed fields get their own single-column
setters instead of going through the generic path.

## Changes to make

In `backend-go/services/task-service/internal/adapter/postgres/repository.go`,
widen the column list used by `Create`, `Get`, and `List` to select/insert
the new columns (nullable ones via `COALESCE(...,'')`/pointer-scan helpers
matching this file's existing `nullableUUID` pattern):

```go
func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO task.tasks (
			id, tenant_id, title, status, parent_id, project_id,
			description, task_type, priority, assignee_id, due_date,
			estimated_hours, prompt_template, ai_context, visibility
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, task.ID, task.TenantID, task.Title, task.Status, nullableUUID(task.ParentID), nullableUUID(task.ProjectID),
		task.Description, orDefault(task.Type, "task"), orDefault(task.Priority, "medium"), nullableUUID(task.AssigneeID),
		task.DueDate, task.EstimatedHours, task.PromptTemplate, task.AIContext, orDefault(task.Visibility, "team"))
	if err != nil {
		return domain.Task{}, fmt.Errorf("postgres: insert task: %w", err)
	}
	return task, nil
}
```

Add the small `orDefault` helper (mirrors `nullableUUID`'s file-local-helper
placement, near the bottom of `repository.go`):

```go
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
```

Widen `Get`'s (and `List`'s / `GetAncestors`'s) `SELECT` column list and
`Scan` call identically — every query in this file that reads a `Task` row
must select the same widened set so `toDomainTask`-shaped scanning stays
consistent:

```go
const taskColumns = `
	id, tenant_id, title, status, COALESCE(parent_id::text, ''), COALESCE(project_id::text, ''),
	COALESCE(description, ''), task_type, priority, COALESCE(assignee_id::text, ''), COALESCE(owner_id::text, ''),
	due_date, estimated_hours, actual_hours, COALESCE(prompt_template, ''), COALESCE(ai_context, ''),
	COALESCE(ai_plan_json::text, ''), visibility, COALESCE(worktree_id::text, ''), COALESCE(agent_session_id, ''),
	progress_percent
`

func scanTask(row rowScanner) (domain.Task, error) {
	var t domain.Task
	err := row.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID, &t.ProjectID,
		&t.Description, &t.Type, &t.Priority, &t.AssigneeID, &t.OwnerID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours, &t.PromptTemplate, &t.AIContext,
		&t.AIPlanJSON, &t.Visibility, &t.WorktreeID, &t.AgentSessionID, &t.ProgressPercent)
	return t, err
}
```

Replace `Get`/`List`/`GetAncestors`'s inline `SELECT ...` column lists with
`SELECT ` + `taskColumns` + ` FROM ...`, and their manual `Scan(...)` calls
with `scanTask(row)` / `scanTask(rows)` (a `rowScanner` interface already
exists in this codebase per `project-service`'s equivalent pattern — define
one locally in this package if not already present: `type rowScanner
interface { Scan(dest ...any) error }`).

Widen `Update` to accept the new client-settable fields (title/status stay
mandatory-shaped; new fields follow the same unconditional-overwrite
semantics as the existing two — the domain-layer field-mask logic in
`usecase.UpdateTask` decides what actually changed before calling this):

```go
func (r *Repository) Update(ctx context.Context, tenantID string, t domain.Task) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET
			title = $3, status = $4, description = $5, task_type = $6, priority = $7,
			assignee_id = $8, due_date = $9, estimated_hours = $10, prompt_template = $11,
			ai_context = $12, visibility = $13, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, t.ID, t.Title, t.Status, t.Description, t.Type, t.Priority,
		nullableUUID(t.AssigneeID), t.DueDate, t.EstimatedHours, t.PromptTemplate, t.AIContext, t.Visibility)
	if err != nil {
		return fmt.Errorf("postgres: update task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found for tenant %s", t.ID, tenantID)
	}
	return nil
}
```

Add three narrow setters (new methods on `Repository`, appended to
`repository.go`) and their `usecase.TaskRepository` port declarations in
`backend-go/services/task-service/internal/usecase/ports.go`:

```go
// ports.go — TaskRepository interface, append:
	UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error
	UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error
	UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error
```

```go
// repository.go
func (r *Repository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET worktree_id = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, nullableUUID(worktreeID))
	if err != nil {
		return fmt.Errorf("postgres: update task worktree_id: %w", err)
	}
	return nil
}

func (r *Repository) UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET prompt_template = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, promptTemplate)
	if err != nil {
		return fmt.Errorf("postgres: update task prompt_template: %w", err)
	}
	return nil
}

func (r *Repository) UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET ai_plan_json = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, aiPlanJSON)
	if err != nil {
		return fmt.Errorf("postgres: update task ai_plan_json: %w", err)
	}
	return nil
}
```

Update every fake `TaskRepository` in `*_test.go` files under
`internal/usecase/` (`fakes_test.go`) to implement the 3 new interface
methods (no-op or map-backed, matching that file's existing fake style) so
the package still compiles.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
go test ./services/task-service/internal/usecase/... -v
```

Expected: clean build; existing `Create`/`Get`/`List`/`Update`/`GetAncestors`
integration tests still pass against the widened column set (add assertions
for round-tripping the new fields — e.g. `TestRepository_Create_PersistsAllFields`).
