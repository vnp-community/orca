// Package postgres implements workflow-service's TemplateRepository and
// ExecutionRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in workflow-service
// that knows SQL exists.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// Repository implements both usecase.TemplateRepository and
// usecase.ExecutionRepository against Postgres via pgx — hand-written SQL
// (see architecture/04-tech-stack.md: sqlc codegen is the eventual target,
// this scaffold hand-writes the equivalent queries directly, matching
// usage-service's reference pattern).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow.templates (id, tenant_id, name, dag_json, scope, parent_template_id)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
	`, tmpl.ID, tmpl.TenantID, tmpl.Name, tmpl.DAGJSON, string(tmpl.Scope), nullableString(tmpl.ParentTemplateID))
	if err != nil {
		return fmt.Errorf("postgres: insert template: %w", err)
	}
	return nil
}

func (r *Repository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
		FROM workflow.templates
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var tmpl domain.WorkflowTemplate
	var scope string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: query template: %w", err)
	}
	tmpl.Scope = domain.Scope(scope)
	return tmpl, nil
}

// Update performs the version-bump-on-write conditional UPDATE — the
// versioning rule this solution adds (SOL-030), mirroring SOL-001's
// AccessPolicy pattern. pgx.ErrNoRows here is unambiguous: the caller
// (usecase.UpdateTemplate) already confirmed the row exists via GetTemplate
// before calling this, so a zero-row UPDATE can only mean the version
// moved between that read and this write.
func (r *Repository) Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE workflow.templates
		SET name = $1, dag_json = $2::jsonb, scope = $3, parent_template_id = NULLIF($4, ''),
		    version = version + 1, updated_at = now()
		WHERE id = $5 AND tenant_id = $6 AND version = $7
		RETURNING id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
	`, t.Name, t.DAGJSON, string(t.Scope), t.ParentTemplateID, t.ID, t.TenantID, expectedVersion)

	var updated domain.WorkflowTemplate
	var scope string
	err := row.Scan(&updated.ID, &updated.TenantID, &updated.Name, &updated.DAGJSON, &scope, &updated.ParentTemplateID, &updated.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateVersionConflict
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: update template: %w", err)
	}
	updated.Scope = domain.Scope(scope)
	return updated, nil
}

// ListTemplates backs usecase.ListTemplates — keyset pagination, same
// shape as annotation-service's ListAnnotations (id::text > pageToken
// cursor, ORDER BY id, next = last row's id iff the page came back full).
func (r *Repository) ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
		FROM workflow.templates
		WHERE tenant_id = $1 AND ($2 = '' OR scope = $2) AND id::text > $3
		ORDER BY id
		LIMIT $4
	`, tenantID, scope, pageToken, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query templates: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		var tmpl domain.WorkflowTemplate
		var s string
		if err := rows.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &s, &tmpl.ParentTemplateID, &tmpl.Version); err != nil {
			return nil, "", fmt.Errorf("postgres: scan template row: %w", err)
		}
		tmpl.Scope = domain.Scope(s)
		out = append(out, tmpl)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// ResolveChain backs usecase.ResolveTemplate — the one genuine
// recursive-SQL query in this service (workflow-service.md §6), depth
// capped inline in the WHERE clause. ORDER BY depth DESC returns
// root-first (furthest ancestor first, the requested template itself
// last), matching usecase.ResolveTemplateOutput.Chain's documented order.
func (r *Repository) ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error) {
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, tenant_id, name, dag_json, scope, parent_template_id, 0 AS depth
			FROM workflow.templates
			WHERE tenant_id = $1 AND id = $2

			UNION ALL

			SELECT t.id, t.tenant_id, t.name, t.dag_json, t.scope, t.parent_template_id, c.depth + 1
			FROM workflow.templates t
			JOIN chain c ON t.id = c.parent_template_id
			WHERE c.depth + 1 < $3
		)
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, '')
		FROM chain
		ORDER BY depth DESC
	`, tenantID, templateID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("postgres: query template chain: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		var tmpl domain.WorkflowTemplate
		var s string
		if err := rows.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &s, &tmpl.ParentTemplateID); err != nil {
			return nil, fmt.Errorf("postgres: scan template chain row: %w", err)
		}
		tmpl.Scope = domain.Scope(s)
		out = append(out, tmpl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate template chain rows: %w", err)
	}
	if len(out) == 0 {
		return nil, domain.ErrTemplateNotFound
	}
	return out, nil
}

// CreateExecution persists exec. TemplateID is nullable
// (migration 0005_execution_ad_hoc_template): a template-driven Execute
// always sets it, but ExecuteAdHocStep's synthetic execution
// (domain.NewAdHocWorkflowExecution) deliberately leaves it empty, since
// an ad hoc run has no backing WorkflowTemplate — see that constructor's
// doc comment.
func (r *Repository) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow.executions (id, template_id, tenant_id, status, root_trace_id, paused_at, project_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, exec.ID, nullableString(exec.TemplateID), exec.TenantID, string(exec.Status), nullableString(exec.RootTraceID), exec.PausedAt, nullableString(exec.ProjectID))
	if err != nil {
		return fmt.Errorf("postgres: insert execution: %w", err)
	}
	return nil
}

func (r *Repository) GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, COALESCE(template_id::text, ''), tenant_id, status, COALESCE(root_trace_id, ''), paused_at, COALESCE(project_id::text, '')
		FROM workflow.executions
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var exec domain.WorkflowExecution
	var status string
	var pausedAt *time.Time
	err := row.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt, &exec.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowExecution{}, domain.ErrExecutionNotFound
	}
	if err != nil {
		return domain.WorkflowExecution{}, fmt.Errorf("postgres: query execution: %w", err)
	}
	exec.Status = domain.Status(status)
	exec.PausedAt = pausedAt
	return exec, nil
}

// HasActiveExecutions backs usecase.HasActiveExecutions — see that
// usecase's doc comment for why this exists (Epic C,
// backend-go/docs/execution-plan.md).
func (r *Repository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM workflow.executions
			WHERE tenant_id = $1 AND project_id = $2 AND status IN ('pending','running','paused')
		)
	`, tenantID, projectID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres: query has-active-executions: %w", err)
	}
	return exists, nil
}

// UpdateExecution persists an execution's mutable fields — status and
// paused_at, set by Pause/Resume transitions and terminal-status writes —
// and, when event is non-nil, an outbox row, ALL in one Postgres
// transaction (SOL-PW-04, TASK-PW-04-05). root_trace_id and template_id
// are immutable after creation and not touched here. Follows
// usage-service.Repository.SaveSession's exact transaction shape (begin ->
// exec execution update -> exec outbox insert if event != nil -> commit;
// defer tx.Rollback for the error path).
func (r *Repository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution, event *domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE workflow.executions
		SET status = $1, paused_at = $2, updated_at = now()
		WHERE tenant_id = $3 AND id = $4
	`, string(exec.Status), exec.PausedAt, exec.TenantID, exec.ID)
	if err != nil {
		return fmt.Errorf("postgres: update execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExecutionNotFound
	}

	if event != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO workflow.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, event.ID, exec.TenantID, event.Subject, event.OccurredAt, 1, event.PayloadJSON)
		if err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired. Identical query/scan
// shape to usage-service's FetchUnpublished/MarkPublished, against
// workflow.outbox_events.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM workflow.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox event rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE workflow.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

// ListRunning backs usecase.RecoverExecutions' boot-time scan — every
// workflow.executions row, across every tenant, currently in
// status='running'. Deliberately unscoped by tenant_id — see
// usecase.ExecutionRepository.ListRunning's doc comment for why this is
// the one query in this port that must see every tenant. Uses
// idx_workflow_executions_resumable (migrations/0001_init), a partial
// index on status IN ('running', 'paused') that also serves this narrower
// status='running' predicate.
func (r *Repository) ListRunning(ctx context.Context) ([]domain.WorkflowExecution, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(template_id::text, ''), tenant_id, status, COALESCE(root_trace_id, ''), paused_at, COALESCE(project_id::text, '')
		FROM workflow.executions
		WHERE status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query running executions: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowExecution
	for rows.Next() {
		var exec domain.WorkflowExecution
		var status string
		var pausedAt *time.Time
		if err := rows.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt, &exec.ProjectID); err != nil {
			return nil, fmt.Errorf("postgres: scan running execution row: %w", err)
		}
		exec.Status = domain.Status(status)
		exec.PausedAt = pausedAt
		out = append(out, exec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate running execution rows: %w", err)
	}
	return out, nil
}

// CreateStepExecution backs usecase.StepExecutionRepository. Tenant scoping
// isn't a parameter here (unlike Create/GetTemplate/Execution) because a
// step_executions row's tenant is implied entirely by its execution_id —
// the FK to workflow.executions plus that table's own tenant check (the
// usecase layer always fetches/creates the owning execution through a
// tenant-scoped call first) is the enforcement; RLS
// (migration 0004_step_executions) is the defense-in-depth backstop.
func (r *Repository) CreateStepExecution(ctx context.Context, se domain.StepExecution) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow.step_executions (id, execution_id, step_id, wave, status, dispatch_token, output, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
	`, se.ID, se.ExecutionID, se.StepID, se.Wave, string(se.Status), se.DispatchToken, nullableString(se.OutputJSON), nullableString(se.Error))
	if err != nil {
		return fmt.Errorf("postgres: insert step execution: %w", err)
	}
	return nil
}

// UpdateStepExecution persists a step execution's mutable fields — status,
// output, error — set as a step transitions pending->running->completed/failed.
func (r *Repository) UpdateStepExecution(ctx context.Context, se domain.StepExecution) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE workflow.step_executions
		SET status = $1, output = $2::jsonb, error_message = $3, updated_at = now()
		WHERE id = $4
	`, string(se.Status), nullableString(se.OutputJSON), nullableString(se.Error), se.ID)
	if err != nil {
		return fmt.Errorf("postgres: update step execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: update step execution: no row for id %s", se.ID)
	}
	return nil
}

// ListStepExecutions returns every step_executions row for
// tenantID/executionID, ordered by wave then step_id — joins to
// workflow.executions for the tenant check, matching the RLS policy's own
// EXISTS join (migration 0004_step_executions).
func (r *Repository) ListStepExecutions(ctx context.Context, tenantID, executionID string) ([]domain.StepExecution, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT se.id, se.execution_id, se.step_id, se.wave, se.status, se.dispatch_token,
		       COALESCE(se.output::text, ''), COALESCE(se.error_message, '')
		FROM workflow.step_executions se
		JOIN workflow.executions e ON e.id = se.execution_id
		WHERE e.tenant_id = $1 AND se.execution_id = $2
		ORDER BY se.wave, se.step_id
	`, tenantID, executionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query step executions: %w", err)
	}
	defer rows.Close()

	var out []domain.StepExecution
	for rows.Next() {
		var se domain.StepExecution
		var status string
		if err := rows.Scan(&se.ID, &se.ExecutionID, &se.StepID, &se.Wave, &status, &se.DispatchToken, &se.OutputJSON, &se.Error); err != nil {
			return nil, fmt.Errorf("postgres: scan step execution row: %w", err)
		}
		se.Status = domain.StepExecutionStatus(status)
		out = append(out, se)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate step execution rows: %w", err)
	}
	return out, nil
}

// nullableString maps an empty Go string to SQL NULL — used for both plain
// text columns and, with a $N::jsonb cast at the call site, JSONB columns
// like step_executions.output (an empty OutputJSON means "no output yet",
// e.g. a pending/running step, which must insert/update as NULL, not the
// literal string "").
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
