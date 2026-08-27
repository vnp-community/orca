// Package postgres implements task-service's TaskRepository, EdgeRepository,
// and GrantRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in task-service that
// knows SQL exists.
//
// Hand-written SQL via pgx, not sqlc codegen — task-service.md §6 names
// sqlc (with hand-written recursive CTEs) as the chosen approach for this
// service specifically, but this scaffold hand-writes the equivalent
// queries directly to avoid an extra build-time toolchain dependency, same
// posture as usage-service's adapter/postgres. Add a sqlc.yaml + regenerate
// once the query set stabilizes — see this service's README.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// dbtx abstracts over the subset of *pgxpool.Pool and pgx.Tx methods this
// package's query methods need — mirrors credential-broker-service's own
// postgres.dbtx (internal/adapter/postgres/repository.go in that service)
// so RunInTx below can hand AIApply a Repository scoped to an open
// transaction without duplicating any SQL between the pooled and
// transactional paths.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Repository implements usecase.TaskRepository, usecase.EdgeRepository, and
// usecase.GrantRepository against Postgres via pgx — one struct, one pool,
// same shape as usage-service/internal/adapter/postgres.Repository. It also
// implements usecase.TxRunner (see RunInTx below), closing TASK-224 Gap 2 —
// see ai_apply.go's doc comment for why AIApply needs this.
type Repository struct {
	pool *pgxpool.Pool
	db   dbtx // == pool outside a transaction; == a pgx.Tx inside RunInTx's fn
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// RunInTx implements usecase.TxRunner: opens one Postgres transaction via
// pgx.BeginFunc (commits on fn returning nil, rolls back and returns fn's
// error otherwise) and hands fn a Repository scoped to that transaction —
// same pattern as credential-broker-service.Repository.RunInTx, reusing the
// exact TaskRepository/EdgeRepository port shapes AIApply's CreateTask/
// AddEdge sub-usecases already call, rather than introducing
// transaction-specific interfaces.
func (r *Repository) RunInTx(ctx context.Context, fn func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		scoped := &Repository{pool: r.pool, db: tx}
		return fn(ctx, scoped, scoped)
	})
}

// taskColumns is the widened column list every query in this file that
// reads a full Task row must select, in the exact order scanTask expects —
// keeps Create/Get/List/GetAncestors's scanning consistent as fields are
// added. Nullable/optional columns come back through COALESCE (or, for
// due_date/estimated_hours/actual_hours, scanned directly into pointer
// fields — pgx scans SQL NULL into a nil *time.Time/*float64 natively).
const taskColumns = `
	id, tenant_id, title, status, COALESCE(parent_id::text, ''), COALESCE(project_id::text, ''),
	COALESCE(description, ''), task_type, priority, COALESCE(assignee_id::text, ''), COALESCE(owner_id::text, ''),
	due_date, estimated_hours, actual_hours, COALESCE(prompt_template, ''), COALESCE(ai_context, ''),
	COALESCE(ai_plan_json::text, ''), visibility, COALESCE(worktree_id::text, ''), COALESCE(agent_session_id, ''),
	progress_percent, COALESCE(active_execution_id, '')
`

// rowScanner abstracts over pgx.Row/pgx.Rows — both satisfy Scan(...any)
// error, letting scanTask serve both a single-row QueryRow result and a
// Rows cursor's per-iteration Scan.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanTask scans one row shaped like taskColumns into a domain.Task — the
// single place that must change when taskColumns' order changes.
func scanTask(row rowScanner) (domain.Task, error) {
	var t domain.Task
	err := row.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID, &t.ProjectID,
		&t.Description, &t.Type, &t.Priority, &t.AssigneeID, &t.OwnerID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours, &t.PromptTemplate, &t.AIContext,
		&t.AIPlanJSON, &t.Visibility, &t.WorktreeID, &t.AgentSessionID, &t.ProgressPercent, &t.ActiveExecutionID)
	return t, err
}

// scanTaskAndTrailing scans a row shaped like taskColumns followed by
// caller-supplied trailing columns (e.g. depth, an aggregated array) into a
// domain.Task plus those extra destinations — used by queries that widen
// taskColumns with extra computed columns (GetSubtreeWithChildPercents).
func scanTaskAndTrailing(row rowScanner, extra ...any) (domain.Task, error) {
	var t domain.Task
	dest := []any{&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID, &t.ProjectID,
		&t.Description, &t.Type, &t.Priority, &t.AssigneeID, &t.OwnerID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours, &t.PromptTemplate, &t.AIContext,
		&t.AIPlanJSON, &t.Visibility, &t.WorktreeID, &t.AgentSessionID, &t.ProgressPercent, &t.ActiveExecutionID}
	dest = append(dest, extra...)
	err := row.Scan(dest...)
	return t, err
}

// prefixedTaskColumns returns taskColumns with every bare column reference
// qualified by alias — used by a recursive CTE's recursive term, which
// joins task.tasks (aliased) against the CTE itself and needs its column
// references disambiguated.
func prefixedTaskColumns(alias string) string {
	return `
	` + alias + `.id, ` + alias + `.tenant_id, ` + alias + `.title, ` + alias + `.status, COALESCE(` + alias + `.parent_id::text, ''), COALESCE(` + alias + `.project_id::text, ''),
	COALESCE(` + alias + `.description, ''), ` + alias + `.task_type, ` + alias + `.priority, COALESCE(` + alias + `.assignee_id::text, ''), COALESCE(` + alias + `.owner_id::text, ''),
	` + alias + `.due_date, ` + alias + `.estimated_hours, ` + alias + `.actual_hours, COALESCE(` + alias + `.prompt_template, ''), COALESCE(` + alias + `.ai_context, ''),
	COALESCE(` + alias + `.ai_plan_json::text, ''), ` + alias + `.visibility, COALESCE(` + alias + `.worktree_id::text, ''), COALESCE(` + alias + `.agent_session_id, ''),
	` + alias + `.progress_percent, COALESCE(` + alias + `.active_execution_id, '')
`
}

func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	_, err := r.db.Exec(ctx, `
		INSERT INTO task.tasks (
			id, tenant_id, title, status, parent_id, project_id,
			description, task_type, priority, assignee_id, owner_id, due_date,
			estimated_hours, prompt_template, ai_context, visibility
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`, task.ID, task.TenantID, task.Title, task.Status, nullableUUID(task.ParentID), nullableUUID(task.ProjectID),
		task.Description, orDefault(task.Type, "task"), orDefault(task.Priority, "medium"), nullableUUID(task.AssigneeID),
		nullableUUID(task.OwnerID), task.DueDate, task.EstimatedHours, task.PromptTemplate, task.AIContext, orDefault(task.Visibility, "team"))
	if err != nil {
		return domain.Task{}, fmt.Errorf("postgres: insert task: %w", err)
	}
	return task, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+taskColumns+`
		FROM task.tasks
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("postgres: task %s not found: %w", id, err)
		}
		return domain.Task{}, fmt.Errorf("postgres: query task: %w", err)
	}
	return t, nil
}

// GetAncestors walks tasks.parent_id from id up to the root via one
// WITH RECURSIVE query, per task-service.md §6's query-shape note and §8's
// max-depth guard (bounded by the depth column in the recursive term, not
// re-queried per hop). The first row is id itself; the last is the root.
func (r *Repository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxAncestorDepth
	}

	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, tenant_id, title, status, parent_id, project_id,
				description, task_type, priority, assignee_id, owner_id,
				due_date, estimated_hours, actual_hours, prompt_template, ai_context,
				ai_plan_json, visibility, worktree_id, agent_session_id, progress_percent, active_execution_id, 0 AS depth
			FROM task.tasks
			WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT t.id, t.tenant_id, t.title, t.status, t.parent_id, t.project_id,
				t.description, t.task_type, t.priority, t.assignee_id, t.owner_id,
				t.due_date, t.estimated_hours, t.actual_hours, t.prompt_template, t.ai_context,
				t.ai_plan_json, t.visibility, t.worktree_id, t.agent_session_id, t.progress_percent, t.active_execution_id, a.depth + 1
			FROM task.tasks t
			JOIN ancestors a ON t.id = a.parent_id
			WHERE a.depth + 1 < $3
		)
		SELECT id, tenant_id, title, status, COALESCE(parent_id::text, ''), COALESCE(project_id::text, ''),
			COALESCE(description, ''), task_type, priority, COALESCE(assignee_id::text, ''), COALESCE(owner_id::text, ''),
			due_date, estimated_hours, actual_hours, COALESCE(prompt_template, ''), COALESCE(ai_context, ''),
			COALESCE(ai_plan_json::text, ''), visibility, COALESCE(worktree_id::text, ''), COALESCE(agent_session_id, ''),
			progress_percent, COALESCE(active_execution_id, '')
		FROM ancestors
		ORDER BY depth
	`, tenantID, id, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("postgres: query ancestors: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan ancestor row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate ancestor rows: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("postgres: task %s not found while resolving ancestors", id)
	}
	return out, nil
}

// UpdateStatus persists a task's status transition — see
// usecase.TaskRepository's doc comment for the one-way-transition caveat
// this method is currently subject to (only ever called with
// StatusInProgress today).
func (r *Repository) UpdateStatus(ctx context.Context, tenantID, id, status string) error {
	tag, err := r.db.Exec(ctx, `UPDATE task.tasks SET status = $1, updated_at = now() WHERE tenant_id = $2 AND id = $3`, status, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: update task status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found", id)
	}
	return nil
}

// HasActiveExecutions reports whether tenantID/projectID has any task
// currently in_progress — see usecase.HasActiveExecutions's doc comment for
// the one-way-transition caveat this answer is subject to today.
func (r *Repository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	row := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task.tasks WHERE tenant_id = $1 AND project_id = $2 AND status = 'in_progress')`, tenantID, projectID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres: query has-active-executions: %w", err)
	}
	return exists, nil
}

// List returns tasks for tenantID, optionally filtered by projectID (empty
// = no filter), ordered and cursor-paginated by id — same shape as
// GetAncestors's plain SELECT (no recursive CTE needed here).
func (r *Repository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+taskColumns+`
		FROM task.tasks
		WHERE tenant_id = $1
		  AND ($2 = '' OR project_id::text = $2)
		  AND ($3 = '' OR id::text > $3)
		ORDER BY id
		LIMIT $4
	`, tenantID, projectID, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan task row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate task rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

// Update persists a partial (title/status) field update — the status guard
// itself runs at the domain layer (domain.Task.SetStatus) before this is
// ever called; this is a plain UPDATE of both columns unconditionally.
func (r *Repository) Update(ctx context.Context, tenantID string, t domain.Task) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET
			title = $3, status = $4, description = $5, task_type = $6, priority = $7,
			assignee_id = $8, due_date = $9, estimated_hours = $10, prompt_template = $11,
			ai_context = $12, visibility = $13, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, t.ID, t.Title, t.Status, t.Description, orDefault(t.Type, "task"), orDefault(t.Priority, "medium"),
		nullableUUID(t.AssigneeID), t.DueDate, t.EstimatedHours, t.PromptTemplate, t.AIContext, orDefault(t.Visibility, "team"))
	if err != nil {
		return fmt.Errorf("postgres: update task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found for tenant %s", t.ID, tenantID)
	}
	return nil
}

// UpdateWorktreeID persists the provisioned worktree a task's execution is
// running in — see SOL-TG-04.
func (r *Repository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET worktree_id = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, nullableUUID(worktreeID))
	if err != nil {
		return fmt.Errorf("postgres: update task worktree_id: %w", err)
	}
	return nil
}

// UpdateActiveExecutionID persists the complex path's coordinator_run id —
// set by ComplexExecutor.Execute right after StartCoordinatorRun succeeds
// (TASK-TG-04-04), read by ReportTaskExecutionResult (TASK-TG-04-05) to
// reject a stale/duplicate callback.
func (r *Repository) UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET active_execution_id = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, activeExecutionID)
	if err != nil {
		return fmt.Errorf("postgres: update task active_execution_id: %w", err)
	}
	return nil
}

// UpdatePromptTemplate persists the "Generate Agent Prompt" output — see
// SOL-TG-02.
func (r *Repository) UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET prompt_template = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, promptTemplate)
	if err != nil {
		return fmt.Errorf("postgres: update task prompt_template: %w", err)
	}
	return nil
}

// UpdateAIPlanJSON persists the raw AIDecompose response for later
// inspection/replay — see SOL-TG-02.
func (r *Repository) UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error {
	_, err := r.db.Exec(ctx, `UPDATE task.tasks SET ai_plan_json = $3, updated_at = now() WHERE tenant_id = $1 AND id = $2`, tenantID, id, aiPlanJSON)
	if err != nil {
		return fmt.Errorf("postgres: update task ai_plan_json: %w", err)
	}
	return nil
}

// CompleteExecution is the simple path's (TASK-TG-04-03) and, via
// TASK-TG-04-05's ReportTaskExecutionResult, the complex path's terminal
// write: sets status, actual_hours, and clears agent_session_id in one
// statement.
func (r *Repository) CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE task.tasks SET status = $3, actual_hours = $4, agent_session_id = NULL, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, status, actualHours)
	if err != nil {
		return fmt.Errorf("postgres: complete task execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found", id)
	}
	return nil
}

// Delete removes a task row. task_edges/task_grants reference tasks(id)
// with ON DELETE CASCADE (migrations/0001_init.up.sql) — no explicit
// edge/grant cleanup needed here.
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM task.tasks WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: task %s not found for tenant %s", id, tenantID)
	}
	return nil
}

func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// orDefault returns fallback when s is empty — used for columns with a
// NOT NULL DEFAULT that a client-facing request may still omit.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
