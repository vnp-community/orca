// Package mysql implements task-service's TaskRepository, EdgeRepository,
// GrantRepository, CommentRepository, ExecutionLinkRepository, OutboxWriter,
// and VelocityResolver ports (defined in internal/usecase) against MySQL/TiDB
// via database/sql + github.com/go-sql-driver/mysql — the multi-database
// rollout adapter for CR-DB-002/CR-DB-003, mirroring internal/adapter/postgres
// 1:1 (same method set, same tenant scoping, same transaction shapes) against
// the dialect-safe schema in migrations/mysql. See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-015-task-service-mysql-tidb-adapter.md.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// dbtx abstracts over the subset of *sql.DB and *sql.Tx methods this
// package's query methods need — mirrors internal/adapter/postgres's own
// dbtx interface, letting RunInTx hand fn a Repository scoped to an open
// transaction without duplicating any SQL between the pooled and
// transactional paths.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Repository implements every DB-backed usecase port task-service defines
// (TaskRepository, EdgeRepository, GrantRepository, CommentRepository,
// ExecutionLinkRepository, OutboxWriter, VelocityResolver, common/outbox.Store)
// against MySQL/TiDB — one struct, one pool, same shape as
// internal/adapter/postgres.Repository. It also implements usecase.TxRunner
// (see RunInTx below).
type Repository struct {
	pool *sql.DB
	db   dbtx // == pool outside a transaction; == a *sql.Tx inside RunInTx's fn
}

func New(pool *sql.DB) *Repository {
	return &Repository{pool: pool, db: pool}
}

// RunInTx implements usecase.TxRunner: opens one MySQL transaction and hands
// fn a Repository scoped to it — mirrors postgres.Repository.RunInTx's
// pgx.BeginFunc shape using database/sql's plain BeginTx/Commit/Rollback,
// since database/sql has no equivalent helper that auto-rolls-back on a
// non-nil error.
func (r *Repository) RunInTx(ctx context.Context, fn func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error) error {
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	scoped := &Repository{pool: r.pool, db: tx}
	if err := fn(ctx, scoped, scoped); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// taskColumns is the bare column list every query in this file that reads a
// full Task row must select, in the exact order scanTask expects. Unlike
// internal/adapter/postgres's taskColumns, this has NO COALESCE/cast
// wrapping — scanTask scans every nullable column into a sql.NullX holder
// and applies the same "NULL -> zero value" defaulting in Go instead, which
// sidesteps a real MySQL pitfall: COALESCE(json_col, ”) on a JSON column
// either errors or coerces ” into (invalid) JSON, since ” is not a
// document — there is no single SQL-side idiom that works uniformly across
// every column type here (TEXT, JSON, numeric, temporal), so scanning
// nullable Go types is both simpler and safer than replicating Postgres's
// per-column casts.
const taskColumns = `
	id, tenant_id, title, status, parent_id, project_id,
	description, task_type, priority, assignee_id, owner_id,
	due_date, estimated_hours, actual_hours, prompt_template, ai_context,
	ai_plan_json, visibility, worktree_id, agent_session_id,
	progress_percent, active_execution_id, last_execution_output,
	task_number, pr_url, workflow_template_id,
	active_execution_link_id,
	labels, reporter_id, workflow_exec_id, done_subtasks, total_subtasks, share_token
`

// rowScanner abstracts over *sql.Row/*sql.Rows — both satisfy
// Scan(...any) error, letting scanTask serve both a single-row
// QueryRowContext result and a Rows cursor's per-iteration Scan.
type rowScanner interface {
	Scan(dest ...any) error
}

// taskRowHolder holds Scan destinations for one taskColumns-shaped row —
// factored out of scanTask so a caller needing TRAILING extra columns
// (subtree.go's GetSubtreeWithChildPercents) can append its own Scan
// destinations after dest() without duplicating this entire nullable-column
// list a second time.
type taskRowHolder struct {
	t                                                                 domain.Task
	parentID, projectID, description, assigneeID, ownerID             sql.NullString
	promptTemplate, aiContext, aiPlanJSON, worktreeID, agentSessionID sql.NullString
	activeExecutionID, lastExecutionOutput, prURL, workflowTemplateID sql.NullString
	activeExecutionLinkID, reporterID, shareToken                     sql.NullString
	dueDate                                                           sql.NullTime
	estimatedHours, actualHours                                       sql.NullFloat64
	taskNumber                                                        sql.NullInt64
	labelsJSON                                                        []byte
}

// dest returns Scan destinations in exactly taskColumns' order.
func (h *taskRowHolder) dest() []any {
	return []any{
		&h.t.ID, &h.t.TenantID, &h.t.Title, &h.t.Status, &h.parentID, &h.projectID,
		&h.description, &h.t.Type, &h.t.Priority, &h.assigneeID, &h.ownerID,
		&h.dueDate, &h.estimatedHours, &h.actualHours, &h.promptTemplate, &h.aiContext,
		&h.aiPlanJSON, &h.t.Visibility, &h.worktreeID, &h.agentSessionID,
		&h.t.ProgressPercent, &h.activeExecutionID, &h.lastExecutionOutput,
		&h.taskNumber, &h.prURL, &h.workflowTemplateID,
		&h.activeExecutionLinkID,
		&h.labelsJSON, &h.reporterID, &h.t.WorkflowExecID, &h.t.DoneSubtasks, &h.t.TotalSubtasks, &h.shareToken,
	}
}

// toTask applies the "NULL -> zero value" defaulting scanTask's doc comment
// describes and returns the assembled domain.Task.
func (h *taskRowHolder) toTask() (domain.Task, error) {
	t := h.t
	t.ParentID = h.parentID.String
	t.ProjectID = h.projectID.String
	t.Description = h.description.String
	t.AssigneeID = h.assigneeID.String
	t.OwnerID = h.ownerID.String
	t.PromptTemplate = h.promptTemplate.String
	t.AIContext = h.aiContext.String
	t.AIPlanJSON = h.aiPlanJSON.String
	t.WorktreeID = h.worktreeID.String
	t.AgentSessionID = h.agentSessionID.String
	t.ActiveExecutionID = h.activeExecutionID.String
	t.LastExecutionOutput = h.lastExecutionOutput.String
	t.PRURL = h.prURL.String
	t.WorkflowTemplateID = h.workflowTemplateID.String
	t.ActiveExecutionLinkID = h.activeExecutionLinkID.String
	t.ReporterID = h.reporterID.String
	t.ShareToken = h.shareToken.String
	if h.dueDate.Valid {
		v := h.dueDate.Time
		t.DueDate = &v
	}
	if h.estimatedHours.Valid {
		v := h.estimatedHours.Float64
		t.EstimatedHours = &v
	}
	if h.actualHours.Valid {
		v := h.actualHours.Float64
		t.ActualHours = &v
	}
	t.TaskNumber = h.taskNumber.Int64
	labels, err := unmarshalLabels(h.labelsJSON)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: unmarshal labels: %w", err)
	}
	t.Labels = labels
	return t, nil
}

// scanTask scans one row shaped like taskColumns into a domain.Task — the
// single place that must change when taskColumns' order changes. Every
// nullable column scans into a sql.NullX holder (taskRowHolder), then
// toTask applies "NULL -> zero value" defaulting in Go — unlike
// internal/adapter/postgres's taskColumns, this deliberately has NO SQL-side
// COALESCE/cast wrapping, since COALESCE(json_col, ”) either errors or
// coerces ” into invalid JSON for the ai_plan_json column, and there is no
// single SQL-side idiom that works uniformly across every column type here.
func scanTask(row rowScanner) (domain.Task, error) {
	var h taskRowHolder
	if err := row.Scan(h.dest()...); err != nil {
		return domain.Task{}, err
	}
	return h.toTask()
}

// Create inserts a task, allocating its task_number from task_number_seq
// (migrations/mysql/0008's AUTO_INCREMENT-table emulation of Postgres's
// nextval('task.task_number_seq')) — see that migration's doc comment for
// why this is 2 statements, not 1, and why that's still safe: exactly like
// a Postgres sequence, a task_number_seq row consumed here is never rolled
// back even if the INSERT INTO tasks below fails, so the two statements
// don't need to share an explicit transaction for correctness.
func (r *Repository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO task_number_seq VALUES (NULL)`)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: allocate task_number: %w", err)
	}
	taskNumber, err := res.LastInsertId()
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: read allocated task_number: %w", err)
	}

	labelsJSON, err := marshalLabels(task.Labels)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: marshal labels: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tasks (
			id, tenant_id, title, status, parent_id, project_id,
			description, task_type, priority, assignee_id, owner_id, due_date,
			estimated_hours, prompt_template, ai_context, visibility, task_number,
			labels, reporter_id, workflow_exec_id
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, task.ID, task.TenantID, task.Title, task.Status, nullableUUID(task.ParentID), nullableUUID(task.ProjectID),
		task.Description, orDefault(task.Type, "task"), orDefault(task.Priority, "medium"), nullableUUID(task.AssigneeID),
		nullableUUID(task.OwnerID), task.DueDate, task.EstimatedHours, task.PromptTemplate, task.AIContext, orDefault(task.Visibility, "team"),
		taskNumber, labelsJSON, nullableUUID(task.ReporterID), task.WorkflowExecID)
	if err != nil {
		return domain.Task{}, fmt.Errorf("mysql: insert task: %w", err)
	}
	task.TaskNumber = taskNumber
	return task, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, id)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("mysql: task %s not found: %w", id, err)
		}
		return domain.Task{}, fmt.Errorf("mysql: query task: %w", err)
	}
	return t, nil
}

// FindByNumber resolves a project-scoped "#TG-N" reference to a task via
// idx_tasks_project_task_number.
func (r *Repository) FindByNumber(ctx context.Context, tenantID, projectID string, taskNumber int64) (domain.Task, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE tenant_id = ? AND project_id = ? AND task_number = ?`, tenantID, projectID, taskNumber)
	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("mysql: no task with number %d in project %s: %w", taskNumber, projectID, err)
		}
		return domain.Task{}, fmt.Errorf("mysql: query task by number: %w", err)
	}
	return t, nil
}

// GetAncestors walks tasks.parent_id from id up to the root via one
// WITH RECURSIVE query — MySQL 8.0.1+ supports recursive CTEs, so this
// translates directly from internal/adapter/postgres's identical query
// shape (placeholder style and column list only differ).
func (r *Repository) GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error) {
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxAncestorDepth
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT `+taskColumns+`, 0 AS depth
			FROM tasks
			WHERE tenant_id = ? AND id = ?

			UNION ALL

			SELECT `+prefixedTaskColumns("t")+`, a.depth + 1
			FROM tasks t
			JOIN ancestors a ON t.id = a.parent_id
			WHERE a.depth + 1 < ?
		)
		SELECT `+taskColumns+` FROM ancestors ORDER BY depth
	`, tenantID, id, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("mysql: query ancestors: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan ancestor row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate ancestor rows: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("mysql: task %s not found while resolving ancestors", id)
	}
	return out, nil
}

// prefixedTaskColumns returns taskColumns with every bare column reference
// qualified by alias — used by a recursive CTE's recursive term, which
// joins tasks (aliased) against the CTE itself.
func prefixedTaskColumns(alias string) string {
	return `
	` + alias + `.id, ` + alias + `.tenant_id, ` + alias + `.title, ` + alias + `.status, ` + alias + `.parent_id, ` + alias + `.project_id,
	` + alias + `.description, ` + alias + `.task_type, ` + alias + `.priority, ` + alias + `.assignee_id, ` + alias + `.owner_id,
	` + alias + `.due_date, ` + alias + `.estimated_hours, ` + alias + `.actual_hours, ` + alias + `.prompt_template, ` + alias + `.ai_context,
	` + alias + `.ai_plan_json, ` + alias + `.visibility, ` + alias + `.worktree_id, ` + alias + `.agent_session_id,
	` + alias + `.progress_percent, ` + alias + `.active_execution_id, ` + alias + `.last_execution_output,
	` + alias + `.task_number, ` + alias + `.pr_url, ` + alias + `.workflow_template_id,
	` + alias + `.active_execution_link_id,
	` + alias + `.labels, ` + alias + `.reporter_id, ` + alias + `.workflow_exec_id, ` + alias + `.done_subtasks, ` + alias + `.total_subtasks, ` + alias + `.share_token
`
}

func (r *Repository) UpdateStatus(ctx context.Context, tenantID, id string, status domain.Status) error {
	res, err := r.db.ExecContext(ctx, `UPDATE tasks SET status = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, status, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task status: %w", err)
	}
	// DELETE/plain-UPDATE-not-found checks are safe to base on RowsAffected
	// here specifically because a status transition changes the status
	// column's value on every legitimate call site (ExecuteTask never
	// calls this with the task's current status) — unlike
	// annotation-service's UpdateAnnotation (BE-DB-SOL-005 §3.1), this is
	// not a client-retriable idempotent write, so the
	// "RowsAffected counts changed rows, not matched rows" driver
	// behavior does not create a false-not-found risk in practice. See
	// this service's solution doc for the general pitfall this comment
	// flags.
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: update task status rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: task %s not found", id)
	}
	return nil
}

func (r *Repository) UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET worktree_id = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, nullableUUID(worktreeID), tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task worktree_id: %w", err)
	}
	return nil
}

func (r *Repository) SetActiveExecutionLink(ctx context.Context, tenantID, taskID, linkID string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE tasks SET active_execution_link_id = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, nullableUUID(linkID), tenantID, taskID)
	if err != nil {
		return fmt.Errorf("mysql: set active execution link: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: set active execution link rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: task %s not found", taskID)
	}
	return nil
}

func (r *Repository) CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE tasks SET status = ?, actual_hours = ?, agent_session_id = NULL, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, status, actualHours, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: complete task execution: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: complete task execution rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: task %s not found", id)
	}
	return nil
}

func (r *Repository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE tenant_id = ? AND project_id = ? AND status = 'in_progress')`, tenantID, projectID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("mysql: query has-active-executions: %w", err)
	}
	return exists, nil
}

// List returns tasks for tenantID, optionally filtered by projectID (empty
// = no filter), cursor-paginated by id. Placeholders repeat per MySQL's '?'
// style (unlike Postgres's $N, which can reference the same bound param
// twice) — each `?` is its own positional argument.
func (r *Repository) List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE tenant_id = ?
		  AND (? = '' OR project_id = ?)
		  AND (? = '' OR id > ?)
		ORDER BY id
		LIMIT ?
	`, tenantID, projectID, projectID, pageToken, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan task row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate task rows: %w", err)
	}
	nextToken := ""
	if len(out) == int(pageSize) {
		nextToken = out[len(out)-1].ID
	}
	return out, nextToken, nil
}

// Update persists a partial field update and, when events is non-empty, one
// outbox row per event — ALL in one MySQL transaction, mirroring
// internal/adapter/postgres.Repository.Update's shape exactly.
func (r *Repository) Update(ctx context.Context, tenantID string, t domain.Task, events []domain.OutboxEvent) error {
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	labelsJSON, err := marshalLabels(t.Labels)
	if err != nil {
		return fmt.Errorf("mysql: marshal labels: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE tasks SET
			title = ?, status = ?, description = ?, task_type = ?, priority = ?,
			assignee_id = ?, due_date = ?, estimated_hours = ?, prompt_template = ?,
			ai_context = ?, visibility = ?, worktree_id = ?, pr_url = ?,
			workflow_template_id = NULLIF(?, ''),
			labels = ?, reporter_id = ?, workflow_exec_id = ?,
			done_subtasks = ?, total_subtasks = ?, share_token = NULLIF(?, ''),
			updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, t.Title, t.Status, t.Description, orDefault(t.Type, "task"), orDefault(t.Priority, "medium"),
		nullableUUID(t.AssigneeID), t.DueDate, t.EstimatedHours, t.PromptTemplate, t.AIContext, orDefault(t.Visibility, "team"),
		nullableUUID(t.WorktreeID), nullableString(t.PRURL), t.WorkflowTemplateID,
		labelsJSON, nullableUUID(t.ReporterID), t.WorkflowExecID,
		t.DoneSubtasks, t.TotalSubtasks, t.ShareToken,
		tenantID, t.ID)
	if err != nil {
		return fmt.Errorf("mysql: update task: %w", err)
	}
	// RowsAffected here is safe against MySQL's "changed rows, not matched
	// rows" semantics for the SAME reason UpdateStatus's comment gives:
	// UpdateTask's usecase (internal/usecase/update_task.go) always applies
	// at least one real field change before calling this — there is no
	// client-facing no-op-retry path through Update the way
	// annotation-service's UpdateAnnotation has, so treating affected == 0
	// as not-found does not risk misclassifying a legitimate retry.
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: update task rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: task %s not found for tenant %s", t.ID, tenantID)
	}

	for _, event := range events {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, ?, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, 1, event.PayloadJSON)
		if err != nil {
			return fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	return tx.Commit()
}

func (r *Repository) UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET active_execution_id = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, activeExecutionID, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task active_execution_id: %w", err)
	}
	return nil
}

// maxLastExecutionOutputBytes mirrors internal/adapter/postgres's identical
// constant — see migrations/mysql/0007's doc comment.
const maxLastExecutionOutputBytes = 8 * 1024

func (r *Repository) UpdateLastExecutionOutput(ctx context.Context, tenantID, id, output string) error {
	if len(output) > maxLastExecutionOutputBytes {
		output = output[:maxLastExecutionOutputBytes]
	}
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET last_execution_output = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, output, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task last_execution_output: %w", err)
	}
	return nil
}

func (r *Repository) UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET prompt_template = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, promptTemplate, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task prompt_template: %w", err)
	}
	return nil
}

func (r *Repository) UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tasks SET ai_plan_json = ?, updated_at = NOW(6) WHERE tenant_id = ? AND id = ?`, aiPlanJSON, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: update task ai_plan_json: %w", err)
	}
	return nil
}

func (r *Repository) ListChildren(ctx context.Context, tenantID, taskID string) ([]domain.Task, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE tenant_id = ? AND parent_id = ?`, tenantID, taskID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query children: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan child row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE tenant_id = ? AND id = ?`, tenantID, id)
	if err != nil {
		return fmt.Errorf("mysql: delete task: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: delete task rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("mysql: task %s not found for tenant %s", id, tenantID)
	}
	return nil
}

func nullableUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// marshalLabels/unmarshalLabels round-trip domain.Task.Labels ([]string)
// through the JSON array migrations/mysql/0011 uses in place of Postgres's
// TEXT[] (MySQL has no native array type) — see that migration's doc
// comment.
func marshalLabels(labels []string) ([]byte, error) {
	if labels == nil {
		labels = []string{}
	}
	return json.Marshal(labels)
}

func unmarshalLabels(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var labels []string
	if err := json.Unmarshal(raw, &labels); err != nil {
		return nil, err
	}
	if labels == nil {
		labels = []string{}
	}
	return labels, nil
}
