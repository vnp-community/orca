// Package mysql implements workflow-service's TemplateRepository,
// ExecutionRepository, StepExecutionRepository and ApprovalRepository ports
// (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — mirroring internal/adapter/postgres's
// behavior 1:1 against the dialect-safe schema in migrations/mysql, per
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-013.md,
// following the pattern set by BE-DB-SOL-002 (usage-service, the pilot).
//
// MySQL RowsAffected() pitfall (BE-DB-SOL-005 §3.1's finding, true here for
// the same reason): the driver's default RowsAffected() counts rows whose
// VALUES actually changed, not rows matched by WHERE — unlike Postgres/pgx,
// where RowsAffected always counts every row an UPDATE's WHERE matched.
// Every write in this file that needs to distinguish "not found" from
// "found" therefore uses an explicit `SELECT ... FOR UPDATE` existence/
// version check inside the surrounding transaction instead of reading
// RowsAffected() — never the shortcut that bit BE-DB-SOL-005's first draft.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// Repository implements usecase.TemplateRepository, usecase.ExecutionRepository
// and usecase.StepExecutionRepository against MySQL/TiDB via database/sql.
// No RLS equivalent exists in MySQL — every query below filters by
// tenant_id explicitly, which is the ONLY tenant-isolation enforcement for
// this adapter (see TASK-BE-DB-003's finding, true here for the same
// reason it was true for usage-service: no Go code anywhere calls
// SET LOCAL app.tenant_id, so the Postgres adapter's RLS policy never
// actually activated either — this doesn't lower the bar, it just doesn't
// add a backstop that was never real).
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// rowScanner is the common subset of *sql.Rows/*sql.Row this package's scan
// helpers need — lets one scan function serve both QueryRowContext and
// QueryContext call sites, mirroring internal/adapter/postgres's identical
// rowScanner interface.
type rowScanner interface {
	Scan(dest ...any) error
}

// nullableString maps an empty Go string to a nil bind (SQL NULL) — used
// for nullable text/JSON columns, mirrors internal/adapter/postgres's
// identical helper.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// templateTags normalizes a nil Tags slice to an empty (non-nil) one before
// marshaling to the tags JSON column — same reasoning as
// internal/adapter/postgres's templateTags (its NOT NULL text[] column;
// here a NOT NULL JSON column that must never receive Go's `null` for a
// nil slice).
func templateTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

// CreateTemplate mirrors internal/adapter/postgres.Repository.CreateTemplate
// exactly — same column list (owner_id deliberately NOT included; see that
// method's doc comment for why: it's a pre-existing gap in the Postgres
// adapter this MySQL adapter reproduces rather than silently fixes, out of
// this rollout's scope).
func (r *Repository) CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error {
	tagsJSON, err := json.Marshal(templateTags(tmpl.Tags))
	if err != nil {
		return fmt.Errorf("mysql: marshal template tags: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO templates (id, tenant_id, name, dag_json, scope, parent_template_id, description, tags)
		VALUES (?,?,?,?,?,?,?,?)
	`, tmpl.ID, tmpl.TenantID, tmpl.Name, tmpl.DAGJSON, string(tmpl.Scope), nullableString(tmpl.ParentTemplateID), tmpl.Description, string(tagsJSON))
	if err != nil {
		return fmt.Errorf("mysql: insert template: %w", err)
	}
	return nil
}

func (r *Repository) GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, ''), version
		FROM templates
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	var tmpl domain.WorkflowTemplate
	var scope string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: query template: %w", err)
	}
	tmpl.Scope = domain.Scope(scope)
	return tmpl, nil
}

// Update performs the conditional UPDATE, gated by expectedVersion — same
// contract as internal/adapter/postgres.Repository.Update (see that
// method's doc comment for the bumpVersion semantics). Implemented as a
// SELECT ... FOR UPDATE + compare + UPDATE inside one transaction rather
// than a single atomic UPDATE ... RETURNING (MySQL has no RETURNING) — the
// row lock makes this race-free against a concurrent writer, equivalent to
// Postgres's row-level lock implicit in its RETURNING UPDATE. A vanished
// row (sql.ErrNoRows on the initial SELECT) maps to
// ErrTemplateVersionConflict, not ErrTemplateNotFound — mirrors the
// Postgres adapter's own documented reasoning: the caller
// (usecase.UpdateTemplate) already confirmed the row exists via GetTemplate
// before calling this, so a vanished/version-mismatched row can only mean
// the version moved between that read and this write.
func (r *Repository) Update(ctx context.Context, t domain.WorkflowTemplate, expectedVersion int32, bumpVersion bool) (domain.WorkflowTemplate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int32
	err = tx.QueryRowContext(ctx, `SELECT version FROM templates WHERE id = ? AND tenant_id = ? FOR UPDATE`, t.ID, t.TenantID).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateVersionConflict
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: lock template: %w", err)
	}
	if currentVersion != expectedVersion {
		return domain.WorkflowTemplate{}, domain.ErrTemplateVersionConflict
	}

	newVersion := currentVersion
	if bumpVersion {
		newVersion++
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE templates
		SET name = ?, dag_json = ?, scope = ?, parent_template_id = ?, version = ?, updated_at = NOW(6)
		WHERE id = ? AND tenant_id = ?
	`, t.Name, t.DAGJSON, string(t.Scope), nullableString(t.ParentTemplateID), newVersion, t.ID, t.TenantID)
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: update template: %w", err)
	}

	var updated domain.WorkflowTemplate
	var scope string
	err = tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, ''), version
		FROM templates WHERE id = ? AND tenant_id = ?
	`, t.ID, t.TenantID).Scan(&updated.ID, &updated.TenantID, &updated.Name, &updated.DAGJSON, &scope, &updated.ParentTemplateID, &updated.Version)
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: re-read updated template: %w", err)
	}
	updated.Scope = domain.Scope(scope)

	if err := tx.Commit(); err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: commit tx: %w", err)
	}
	return updated, nil
}

// listTemplatesColumns mirrors internal/adapter/postgres's identical
// constant — same column set, same reasoning (ListTemplates' trending sort
// needs usage_count/rating_sum/rating_count/visibility projected; other
// read methods in this file still don't, a pre-existing gap reproduced
// here, not fixed).
const listTemplatesColumns = `id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, ''), version,
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

// tagConditions builds the dynamic AND-filter of JSON_CONTAINS predicates
// this dialect needs in place of Postgres's single `tags @> $N::text[]`
// containment operator — MySQL/TiDB has no array type or GIN-equivalent
// index (see migrations/mysql/0008_template_authoring_fields.up.sql's
// comment), so tags is a JSON array column and every requested tag becomes
// its own JSON_CONTAINS(tags, <tag-as-json-scalar>, '$') predicate, ANDed
// together — same "every listed tag must be present" semantics as the
// Postgres containment operator, just as N separate predicates instead of
// one array operator.
func tagConditions(tags []string) (string, []any) {
	if len(tags) == 0 {
		return "", nil
	}
	var b strings.Builder
	args := make([]any, 0, len(tags))
	for _, tag := range tags {
		b.WriteString(" AND JSON_CONTAINS(tags, ?, '$')")
		tagJSON, _ := json.Marshal(tag) // a Go string always marshals to a valid JSON scalar
		args = append(args, string(tagJSON))
	}
	return b.String(), args
}

// ListTemplates mirrors internal/adapter/postgres.Repository.ListTemplates'
// sort dispatch exactly.
func (r *Repository) ListTemplates(ctx context.Context, tenantID, scope, query string, tags []string, sort, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	switch sort {
	case "trending":
		return r.listTemplatesTrending(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	case "recent":
		return r.listTemplatesRecent(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	default:
		return r.listTemplatesByID(ctx, tenantID, scope, query, tags, pageToken, pageSize)
	}
}

// listTemplatesByID is the original (pre-authoring-fields) default sort —
// same id-keyset shape as the Postgres adapter, now also filterable by
// query (FULLTEXT, idx_workflow_templates_fts) and tags (AND-filter, see
// tagConditions).
func (r *Repository) listTemplatesByID(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	tagWhere, tagArgs := tagConditions(tags)
	sqlText := `
		SELECT ` + listTemplatesColumns + `
		FROM templates
		WHERE tenant_id = ? AND (? = '' OR scope = ?) AND id > ?
		  AND (? = '' OR MATCH(name, description) AGAINST(? IN NATURAL LANGUAGE MODE))
		  ` + tagWhere + `
		ORDER BY id
		LIMIT ?
	`
	args := []any{tenantID, scope, scope, pageToken, query, query}
	args = append(args, tagArgs...)
	args = append(args, pageSize)

	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query templates: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		tmpl, _, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan template row: %w", err)
		}
		out = append(out, tmpl)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// listTemplatesTrending mirrors internal/adapter/postgres's identical
// keyset shape (usage_count DESC, rating_sum DESC, id ASC tiebreak).
func (r *Repository) listTemplatesTrending(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	cursor, err := decodeListCursor(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: decode trending page_token: %w", err)
	}
	tagWhere, tagArgs := tagConditions(tags)

	sqlText := `
		SELECT ` + listTemplatesColumns + `
		FROM templates
		WHERE tenant_id = ? AND (? = '' OR scope = ?)
		  AND (? = '' OR MATCH(name, description) AGAINST(? IN NATURAL LANGUAGE MODE))
		  ` + tagWhere + `
		  AND (
		    ? = '' OR
		    usage_count < ? OR
		    (usage_count = ? AND rating_sum < ?) OR
		    (usage_count = ? AND rating_sum = ? AND id > ?)
		  )
		ORDER BY usage_count DESC, rating_sum DESC, id ASC
		LIMIT ?
	`
	args := []any{tenantID, scope, scope, query, query}
	args = append(args, tagArgs...)
	args = append(args, pageToken, cursor.usageCount, cursor.usageCount, cursor.ratingSum, cursor.usageCount, cursor.ratingSum, cursor.id, pageSize)

	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query templates (trending): %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	var last domain.WorkflowTemplate
	for rows.Next() {
		tmpl, _, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan template row: %w", err)
		}
		out = append(out, tmpl)
		last = tmpl
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = encodeListCursor(listCursor{usageCount: last.UsageCount, ratingSum: last.RatingSum, id: last.ID})
	}
	return out, next, nil
}

// listTemplatesRecent mirrors internal/adapter/postgres's identical
// keyset shape (updated_at DESC, id ASC tiebreak).
func (r *Repository) listTemplatesRecent(ctx context.Context, tenantID, scope, query string, tags []string, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error) {
	cursor, err := decodeListCursor(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: decode recent page_token: %w", err)
	}
	tagWhere, tagArgs := tagConditions(tags)

	sqlText := `
		SELECT ` + listTemplatesColumns + `
		FROM templates
		WHERE tenant_id = ? AND (? = '' OR scope = ?)
		  AND (? = '' OR MATCH(name, description) AGAINST(? IN NATURAL LANGUAGE MODE))
		  ` + tagWhere + `
		  AND (
		    ? = '' OR
		    updated_at < ? OR
		    (updated_at = ? AND id > ?)
		  )
		ORDER BY updated_at DESC, id ASC
		LIMIT ?
	`
	args := []any{tenantID, scope, scope, query, query}
	args = append(args, tagArgs...)
	args = append(args, pageToken, cursor.updatedAt, cursor.updatedAt, cursor.id, pageSize)

	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query templates (recent): %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	var lastUpdatedAt time.Time
	var lastID string
	for rows.Next() {
		tmpl, updatedAt, err := scanListedTemplate(rows)
		if err != nil {
			return nil, "", fmt.Errorf("mysql: scan template row: %w", err)
		}
		out = append(out, tmpl)
		lastUpdatedAt, lastID = updatedAt, tmpl.ID
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate template rows: %w", err)
	}

	next := ""
	if int32(len(out)) == pageSize && len(out) > 0 {
		next = encodeListCursor(listCursor{updatedAt: lastUpdatedAt, id: lastID})
	}
	return out, next, nil
}

// listCursor/encodeListCursor/decodeListCursor mirror
// internal/adapter/postgres's identical opaque-token shapes — pure Go,
// dialect-independent, duplicated here rather than shared because neither
// adapter package imports the other (see architecture/03's dependency
// rule: adapter packages don't import sibling adapter packages).
type listCursor struct {
	usageCount int32
	ratingSum  int32
	updatedAt  time.Time
	id         string
}

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
	case 2:
		ts, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return listCursor{}, fmt.Errorf("invalid page_token: %w", err)
		}
		return listCursor{updatedAt: ts, id: parts[1]}, nil
	case 3:
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

// ResolveChain backs usecase.ResolveTemplate — MySQL 8.0.14+/TiDB both
// support WITH RECURSIVE, so this is the same recursive-CTE shape as the
// Postgres adapter, no application-side chain-walking fallback needed.
func (r *Repository) ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, tenant_id, name, dag_json, scope, parent_template_id, 0 AS depth
			FROM templates
			WHERE tenant_id = ? AND id = ?

			UNION ALL

			SELECT t.id, t.tenant_id, t.name, t.dag_json, t.scope, t.parent_template_id, c.depth + 1
			FROM templates t
			JOIN chain c ON t.id = c.parent_template_id
			WHERE c.depth + 1 < ?
		)
		SELECT id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, '')
		FROM chain
		ORDER BY depth DESC
	`, tenantID, templateID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("mysql: query template chain: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowTemplate
	for rows.Next() {
		var tmpl domain.WorkflowTemplate
		var s string
		if err := rows.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &s, &tmpl.ParentTemplateID); err != nil {
			return nil, fmt.Errorf("mysql: scan template chain row: %w", err)
		}
		tmpl.Scope = domain.Scope(s)
		out = append(out, tmpl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate template chain rows: %w", err)
	}
	if len(out) == 0 {
		return nil, domain.ErrTemplateNotFound
	}
	return out, nil
}

// CreateExecution mirrors internal/adapter/postgres.Repository.CreateExecution.
func (r *Repository) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO executions (id, template_id, tenant_id, status, root_trace_id, paused_at, project_id, origin_task_id, inputs_json)
		VALUES (?,?,?,?,?,?,?,?,?)
	`, exec.ID, nullableString(exec.TemplateID), exec.TenantID, string(exec.Status), nullableString(exec.RootTraceID), exec.PausedAt, nullableString(exec.ProjectID), exec.OriginTaskID, nullableString(exec.InputsJSON))
	if err != nil {
		return fmt.Errorf("mysql: insert execution: %w", err)
	}
	return nil
}

func (r *Repository) GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(template_id, ''), tenant_id, status, COALESCE(root_trace_id, ''), paused_at, COALESCE(project_id, ''), origin_task_id, COALESCE(inputs_json, '')
		FROM executions
		WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	var exec domain.WorkflowExecution
	var status string
	var pausedAt sql.NullTime
	err := row.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt, &exec.ProjectID, &exec.OriginTaskID, &exec.InputsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowExecution{}, domain.ErrExecutionNotFound
	}
	if err != nil {
		return domain.WorkflowExecution{}, fmt.Errorf("mysql: query execution: %w", err)
	}
	exec.Status = domain.Status(status)
	if pausedAt.Valid {
		exec.PausedAt = &pausedAt.Time
	}
	return exec, nil
}

// HasActiveExecutions backs usecase.HasActiveExecutions.
func (r *Repository) HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM executions
			WHERE tenant_id = ? AND project_id = ? AND status IN ('pending','running','paused')
		)
	`, tenantID, projectID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("mysql: query has-active-executions: %w", err)
	}
	return exists, nil
}

// UpdateExecution persists an execution's mutable fields and, when event is
// non-nil, an outbox row, ALL in one transaction — see this file's package
// doc comment for why a SELECT ... FOR UPDATE existence check replaces the
// Postgres adapter's RowsAffected()==0 check.
func (r *Repository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution, event *domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM executions WHERE tenant_id = ? AND id = ? FOR UPDATE`, exec.TenantID, exec.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrExecutionNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: lock execution: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE executions
		SET status = ?, paused_at = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, string(exec.Status), exec.PausedAt, exec.TenantID, exec.ID)
	if err != nil {
		return fmt.Errorf("mysql: update execution: %w", err)
	}

	if event != nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, ?, ?)
		`, event.ID, exec.TenantID, event.Subject, event.OccurredAt, 1, []byte(event.PayloadJSON))
		if err != nil {
			return fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store —
// identical query/scan shape to the Postgres adapter, against
// outbox_events.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox event row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox event rows: %w", err)
	}
	return out, nil
}

// MarkPublished builds a dynamic IN (...) placeholder list — MySQL has no
// `= ANY($1)` array-parameter equivalent, mirrors
// usage-service/annotation-service's identical dynamic-IN pattern. Guards
// len(ids)==0 since MySQL's `IN ()` is a syntax error, not an empty match.
func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox_events SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}

// ListRunning backs usecase.RecoverExecutions' boot-time scan — see
// usecase.ExecutionRepository.ListRunning's doc comment for why this is
// deliberately unscoped by tenant_id.
func (r *Repository) ListRunning(ctx context.Context) ([]domain.WorkflowExecution, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(template_id, ''), tenant_id, status, COALESCE(root_trace_id, ''), paused_at, COALESCE(project_id, ''), origin_task_id, COALESCE(inputs_json, '')
		FROM executions
		WHERE status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query running executions: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowExecution
	for rows.Next() {
		var exec domain.WorkflowExecution
		var status string
		var pausedAt sql.NullTime
		if err := rows.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt, &exec.ProjectID, &exec.OriginTaskID, &exec.InputsJSON); err != nil {
			return nil, fmt.Errorf("mysql: scan running execution row: %w", err)
		}
		exec.Status = domain.Status(status)
		if pausedAt.Valid {
			exec.PausedAt = &pausedAt.Time
		}
		out = append(out, exec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate running execution rows: %w", err)
	}
	return out, nil
}

// ListExecutions backs usecase.ListExecutions — keyset pagination ordered
// newest-first by created_at. MySQL/TiDB both support row-value
// subquery comparison (`(a, b) < (SELECT a, b FROM ...)`), so this is the
// same shape as the Postgres adapter's compound-key WHERE clause, not an
// application-side rewrite.
func (r *Repository) ListExecutions(ctx context.Context, tenantID, projectID, cursor string, limit int32) ([]domain.WorkflowExecution, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(template_id, ''), tenant_id, status, COALESCE(root_trace_id, ''), paused_at, COALESCE(project_id, '')
		FROM executions
		WHERE tenant_id = ?
		  AND (? = '' OR project_id = ?)
		  AND (
		    ? = ''
		    OR (created_at, id) < (SELECT created_at, id FROM executions WHERE id = ? AND tenant_id = ?)
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, tenantID, projectID, projectID, cursor, cursor, tenantID, limit)
	if err != nil {
		return nil, "", fmt.Errorf("mysql: query executions: %w", err)
	}
	defer rows.Close()

	var out []domain.WorkflowExecution
	for rows.Next() {
		var exec domain.WorkflowExecution
		var status string
		var pausedAt sql.NullTime
		if err := rows.Scan(&exec.ID, &exec.TemplateID, &exec.TenantID, &status, &exec.RootTraceID, &pausedAt, &exec.ProjectID); err != nil {
			return nil, "", fmt.Errorf("mysql: scan execution row: %w", err)
		}
		exec.Status = domain.Status(status)
		if pausedAt.Valid {
			exec.PausedAt = &pausedAt.Time
		}
		out = append(out, exec)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("mysql: iterate execution rows: %w", err)
	}

	next := ""
	if int32(len(out)) == limit && len(out) > 0 {
		next = out[len(out)-1].ID
	}
	return out, next, nil
}

// CreateStepExecution backs usecase.StepExecutionRepository — see
// internal/adapter/postgres's identical method doc comment for why tenant
// scoping isn't a parameter here.
func (r *Repository) CreateStepExecution(ctx context.Context, se domain.StepExecution) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO step_executions (id, execution_id, step_id, wave, status, dispatch_token, output, error_message)
		VALUES (?,?,?,?,?,?,?,?)
	`, se.ID, se.ExecutionID, se.StepID, se.Wave, string(se.Status), se.DispatchToken, nullableString(se.OutputJSON), nullableString(se.Error))
	if err != nil {
		return fmt.Errorf("mysql: insert step execution: %w", err)
	}
	return nil
}

// UpdateStepExecution persists a step execution's mutable fields and,
// when event is non-zero, an outbox row — see this file's package doc
// comment for why a SELECT ... FOR UPDATE existence check replaces the
// Postgres adapter's RowsAffected()==0 check. tenant_id for the outbox row
// is resolved by joining to executions via se.ExecutionID, same as the
// Postgres adapter (this table carries no tenant_id column of its own).
func (r *Repository) UpdateStepExecution(ctx context.Context, se domain.StepExecution, event domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM step_executions WHERE id = ? FOR UPDATE`, se.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: update step execution: no row for id %s", se.ID)
	}
	if err != nil {
		return fmt.Errorf("mysql: lock step execution: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE step_executions
		SET status = ?, output = ?, error_message = ?, updated_at = NOW(6)
		WHERE id = ?
	`, string(se.Status), nullableString(se.OutputJSON), nullableString(se.Error), se.ID)
	if err != nil {
		return fmt.Errorf("mysql: update step execution: %w", err)
	}

	if event.ID != "" {
		var tenantID string
		if err := tx.QueryRowContext(ctx, `SELECT tenant_id FROM executions WHERE id = ?`, se.ExecutionID).Scan(&tenantID); err != nil {
			return fmt.Errorf("mysql: resolve step execution tenant: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES (?, ?, ?, ?, 1, ?)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, []byte(event.PayloadJSON)); err != nil {
			return fmt.Errorf("mysql: insert outbox event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

// ListStepExecutions returns every step_executions row for
// tenantID/executionID, ordered by wave then step_id — joins to executions
// for the tenant check, same as the Postgres adapter's RLS-mirroring join.
func (r *Repository) ListStepExecutions(ctx context.Context, tenantID, executionID string) ([]domain.StepExecution, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT se.id, se.execution_id, se.step_id, se.wave, se.status, se.dispatch_token,
		       COALESCE(se.output, ''), COALESCE(se.error_message, '')
		FROM step_executions se
		JOIN executions e ON e.id = se.execution_id
		WHERE e.tenant_id = ? AND se.execution_id = ?
		ORDER BY se.wave, se.step_id
	`, tenantID, executionID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query step executions: %w", err)
	}
	defer rows.Close()

	var out []domain.StepExecution
	for rows.Next() {
		var se domain.StepExecution
		var status string
		if err := rows.Scan(&se.ID, &se.ExecutionID, &se.StepID, &se.Wave, &status, &se.DispatchToken, &se.OutputJSON, &se.Error); err != nil {
			return nil, fmt.Errorf("mysql: scan step execution row: %w", err)
		}
		se.Status = domain.StepExecutionStatus(status)
		out = append(out, se)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate step execution rows: %w", err)
	}
	return out, nil
}
