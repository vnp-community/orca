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
	"strconv"
	"strings"
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

// CreateTemplate persists description/tags alongside the original
// id/tenant_id/name/dag_json/scope/parent_template_id columns — TASK-WF-03-07
// needed at least these two writable at creation time for
// ListTemplates(query=.../tags=...) to have anything real to filter
// against (both are untyped TEXT/TEXT[] columns, safe to write
// unconditionally). owner_id is deliberately NOT included here despite
// being part of the domain struct at construction: it's a UUID-typed
// column, and this codebase's own test fixtures routinely construct
// templates with a non-UUID placeholder owner id (e.g. "owner-1") — wiring
// it through this INSERT is a real gap (visibility/overrides/inject/
// remove_steps are too) but out of this task's scope; see TASK-WF-01-02's
// status note.
func (r *Repository) CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workflow.templates (id, tenant_id, name, dag_json, scope, parent_template_id, description, tags)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8)
	`, tmpl.ID, tmpl.TenantID, tmpl.Name, tmpl.DAGJSON, string(tmpl.Scope), nullableString(tmpl.ParentTemplateID), tmpl.Description, templateTags(tmpl.Tags))
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

// Update performs the conditional UPDATE, gated by expectedVersion — the
// optimistic-concurrency check this solution adds (SOL-030), mirroring
// SOL-001's AccessPolicy pattern. templates.version itself only increments
// when bumpVersion is true (TASK-WF-01-06: a breaking DAG change to a
// template with active usage) — everything else about the row still
// updates unconditionally. pgx.ErrNoRows here is unambiguous: the caller
// (usecase.UpdateTemplate) already confirmed the row exists via GetTemplate
// before calling this, so a zero-row UPDATE can only mean the version
// moved between that read and this write.
func (r *Repository) Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32, bumpVersion bool) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE workflow.templates
		SET name = $1, dag_json = $2::jsonb, scope = $3, parent_template_id = NULLIF($4, '')::uuid,
		    version = version + (CASE WHEN $8 THEN 1 ELSE 0 END), updated_at = now()
		WHERE id = $5 AND tenant_id = $6 AND version = $7
		RETURNING id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version
	`, t.Name, t.DAGJSON, string(t.Scope), t.ParentTemplateID, t.ID, t.TenantID, expectedVersion, bumpVersion)

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

// listTemplatesColumns is shared by every ListTemplates sort branch below —
// keeping usage_count/rating_sum/rating_count/visibility in the SELECT
// (not just id/tenant_id/name/...) both backs the trending sort's cursor
// and, for free, finally populates those fields on the returned
// domain.WorkflowTemplate — a pre-existing gap (this repository's other
// read methods still don't project them; out of scope to fix everywhere
// in this task) that ListTemplates specifically needed closed anyway.
const listTemplatesColumns = `id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version,
	       visibility, usage_count, rating_sum, rating_count, updated_at`

func scanListedTemplate(row rowScanner) (domain.WorkflowTemplate, time.Time, error) {
	var tmpl domain.WorkflowTemplate
	var scope, visibility string
	var updatedAt time.Time
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version,
		&visibility, &tmpl.UsageCount, &tmpl.RatingSum, &tmpl.RatingCount, &updatedAt)
	if err != nil {
		return domain.WorkflowTemplate{}, time.Time{}, err
	}
	tmpl.Scope = domain.Scope(scope)
	tmpl.Visibility = domain.Visibility(visibility)
	return tmpl, updatedAt, nil
}

// ListTemplates backs usecase.ListTemplates — BUG-WF-03's library search
// (query/tags) plus a per-sort keyset pagination scheme (see
// encodeListCursor/decodeListCursor's doc comment for why default/trending/
// recent each need a different opaque page_token shape).
func (r *Repository) ListTemplates(ctx context.Context, tenantID, scope, query string, tags []string, sort, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	if tags == nil {
		// pgx encodes a nil Go slice as SQL NULL for a text[] param, and
		// cardinality(NULL) is NULL (not 0) — that would make the "no tags
		// filter" OR-branch evaluate to NULL (never true) below, silently
		// excluding every row. A non-nil empty slice encodes as '{}',
		// where cardinality is a real 0.
		tags = []string{}
	}
	switch sort {
	case "trending":
		return r.listTemplatesTrending(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	case "recent":
		return r.listTemplatesRecent(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	default:
		return r.listTemplatesByID(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	}
}

// listTemplatesByID is the original (pre-TASK-WF-03-07) default sort —
// same id-keyset shape as annotation-service's ListAnnotations, now also
// filterable by query (full-text, idx_templates_fts) and tags (AND-filter,
// GIN-indexed).
func (r *Repository) listTemplatesByID(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+listTemplatesColumns+`
		FROM workflow.templates
		WHERE tenant_id = $1 AND ($2 = '' OR scope = $2) AND id::text > $3
		  AND ($4 = '' OR to_tsvector('english', name || ' ' || coalesce(description,'')) @@ plainto_tsquery('english', $4))
		  AND (cardinality($5::text[]) = 0 OR tags @> $5::text[])
		ORDER BY id
		LIMIT $6
	`, tenantID, scope, pageToken, query, tags, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query templates: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		tmpl, _, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan template row: %w", err)
		}
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

// listTemplatesTrending sorts by usage_count DESC, rating_sum DESC (backed
// by idx_templates_trending), id ASC as the tiebreak every keyset
// pagination scheme needs for a deterministic, non-overlapping second
// page. The WHERE predicate below is the standard multi-column keyset
// pagination shape for that exact ORDER BY (see encodeListCursor's doc
// comment for the token this decodes).
func (r *Repository) listTemplatesTrending(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	cursor, err := decodeListCursor(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: decode trending page_token: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+listTemplatesColumns+`
		FROM workflow.templates
		WHERE tenant_id = $1 AND ($2 = '' OR scope = $2)
		  AND ($3 = '' OR to_tsvector('english', name || ' ' || coalesce(description,'')) @@ plainto_tsquery('english', $3))
		  AND (cardinality($4::text[]) = 0 OR tags @> $4::text[])
		  AND (
		    $5 = '' OR
		    usage_count < $6 OR
		    (usage_count = $6 AND rating_sum < $7) OR
		    (usage_count = $6 AND rating_sum = $7 AND id::text > $8)
		  )
		ORDER BY usage_count DESC, rating_sum DESC, id ASC
		LIMIT $9
	`, tenantID, scope, query, tags, pageToken, cursor.usageCount, cursor.ratingSum, cursor.id, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query templates (trending): %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	var last domain.WorkflowTemplate
	for rows.Next() {
		tmpl, _, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan template row: %w", err)
		}
		out = append(out, tmpl)
		last = tmpl
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = encodeListCursor(listCursor{usageCount: last.UsageCount, ratingSum: last.RatingSum, id: last.ID})
	}
	return out, next, nil
}

// listTemplatesRecent sorts by updated_at DESC, id ASC tiebreak.
func (r *Repository) listTemplatesRecent(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	cursor, err := decodeListCursor(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: decode recent page_token: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+listTemplatesColumns+`
		FROM workflow.templates
		WHERE tenant_id = $1 AND ($2 = '' OR scope = $2)
		  AND ($3 = '' OR to_tsvector('english', name || ' ' || coalesce(description,'')) @@ plainto_tsquery('english', $3))
		  AND (cardinality($4::text[]) = 0 OR tags @> $4::text[])
		  AND (
		    $5 = '' OR
		    updated_at < $6 OR
		    (updated_at = $6 AND id::text > $7)
		  )
		ORDER BY updated_at DESC, id ASC
		LIMIT $8
	`, tenantID, scope, query, tags, pageToken, cursor.updatedAt, cursor.id, pageSize)
	if err != nil {
		return nil, "", fmt.Errorf("postgres: query templates (recent): %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	var lastUpdatedAt time.Time
	var lastID string
	for rows.Next() {
		tmpl, updatedAt, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("postgres: scan template row: %w", err)
		}
		out = append(out, tmpl)
		lastUpdatedAt, lastID = updatedAt, tmpl.ID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("postgres: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = encodeListCursor(listCursor{updatedAt: lastUpdatedAt, id: lastID})
	}
	return out, next, nil
}

// listCursor is the decoded shape of a trending/recent page_token — see
// encodeListCursor's doc comment.
type listCursor struct {
	usageCount int32
	ratingSum  int32
	updatedAt  time.Time
	id         string
}

// encodeListCursor/decodeListCursor: keyset pagination and a non-id sort
// don't compose for free — a bare last-seen id (the default sort's whole
// cursor) can't resume a trending/recent ORDER BY, since two different
// templates can tie on usage_count/rating_sum/updated_at and id alone
// doesn't say where in THAT ordering the page stopped. The opaque token
// therefore encodes the full sort key the query resumes from
// ("usageCount:ratingSum:id" for trending, "updatedAtRFC3339Nano:id" for
// recent) — still opaque to callers (an implementation detail of this
// adapter, never interpreted by usecase/grpc layers), just not a bare id
// for these two sorts.
func encodeListCursor(c listCursor) string {
	if !c.updatedAt.IsZero() {
		return c.updatedAt.Format(time.RFC3339Nano) + ":" + c.id
	}
	return fmt.Sprintf("%d:%d:%s", c.usageCount, c.ratingSum, c.id)
}

func decodeListCursor(token string) (listCursor, error) {
	if token == "" {
		return listCursor{}, nil
	}
	parts := strings.SplitN(token, ":", 3)
	switch len(parts) {
	case 2: // recent: updatedAt:id
		ts, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return listCursor{}, fmt.Errorf("invalid page_token: %w", err)
		}
		return listCursor{updatedAt: ts, id: parts[1]}, nil
	case 3: // trending: usageCount:ratingSum:id
		usageCount, err := strconv.ParseInt(parts[0], 10, 32)
		if err != nil {
			return listCursor{}, fmt.Errorf("invalid page_token: %w", err)
		}
		ratingSum, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return listCursor{}, fmt.Errorf("invalid page_token: %w", err)
		}
		return listCursor{usageCount: int32(usageCount), ratingSum: int32(ratingSum), id: parts[2]}, nil
	default:
		return listCursor{}, fmt.Errorf("invalid page_token: %q", token)
	}
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

// templateTags normalizes a nil Tags slice to an empty (non-nil) one —
// pgx encodes a nil Go slice as SQL NULL for a text[] param, which would
// violate workflow.templates.tags' NOT NULL constraint on INSERT.
func templateTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
