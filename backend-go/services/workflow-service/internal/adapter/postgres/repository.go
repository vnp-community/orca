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
		INSERT INTO workflow.templates (id, tenant_id, name, dag_json, scope)
		VALUES ($1, $2, $3, $4::jsonb, $5)
	`, tmpl.ID, tmpl.TenantID, tmpl.Name, tmpl.DAGJSON, string(tmpl.Scope))
	if err != nil {
		return fmt.Errorf("postgres: insert template: %w", err)
	}
	return nil
}

func (r *Repository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope
		FROM workflow.templates
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var tmpl domain.WorkflowTemplate
	var scope string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: query template: %w", err)
	}
	tmpl.Scope = domain.Scope(scope)
	return tmpl, nil
}

func (r *Repository) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow.executions (id, template_id, tenant_id, status, root_trace_id, paused_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, exec.ID, exec.TemplateID, exec.TenantID, string(exec.Status), nullableString(exec.RootTraceID), exec.PausedAt)
	if err != nil {
		return fmt.Errorf("postgres: insert execution: %w", err)
	}
	return nil
}

func (r *Repository) GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, template_id, tenant_id, status, COALESCE(root_trace_id, ''), paused_at
		FROM workflow.executions
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var exec domain.WorkflowExecution
	var status string
	var pausedAt *time.Time
	err := row.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt)
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

// UpdateExecution persists an execution's mutable fields — status and
// paused_at, set by Pause/Resume transitions. root_trace_id and
// template_id are immutable after creation and not touched here.
func (r *Repository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	tag, err := r.pool.Exec(ctx, `
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
	return nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
