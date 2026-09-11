package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// WithTx implements usecase.TemplateRepository.WithTx — begins a
// transaction, hands fn a templateTx scoped to it, commits on nil and
// rolls back on any error fn returns (Rollback after a successful Commit
// is a safe no-op, same as internal/adapter/postgres's identical method).
func (r *Repository) WithTx(ctx context.Context, fn func(tx usecase.TemplateRepositoryTx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&templateTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

// GetByShareToken looks up the (at most one) template whose share_token
// matches — backs PreviewSharedTemplate/ImportSharedTemplate.
func (r *Repository) GetByShareToken(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, ''), version,
		       visibility, COALESCE(share_token, ''), rating_sum, rating_count
		FROM templates
		WHERE share_token = ?
	`, shareToken)

	var tmpl domain.WorkflowTemplate
	var scope, visibility string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version,
		&visibility, &tmpl.ShareToken, &tmpl.RatingSum, &tmpl.RatingCount)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: query template by share token: %w", err)
	}
	tmpl.Scope = domain.Scope(scope)
	tmpl.Visibility = domain.Visibility(visibility)
	return tmpl, nil
}

// SetShareToken mints (or overwrites) templateID's share_token — backs
// usecase.GenerateShareLink. Not tenant-scoped in its WHERE clause, same
// already-confirmed-ownership precondition as the Postgres adapter's
// identical method. Wrapped in its own transaction (unlike the Postgres
// adapter's single Exec) purely so the existence check can use
// SELECT ... FOR UPDATE instead of trusting RowsAffected() — see this
// package's repository.go doc comment.
func (r *Repository) SetShareToken(ctx context.Context, templateID, token string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM templates WHERE id = ? FOR UPDATE`, templateID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrTemplateNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: lock template: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE templates SET share_token = ?, updated_at = NOW(6) WHERE id = ?`, token, templateID); err != nil {
		return fmt.Errorf("mysql: set share token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

// templateTx implements usecase.TemplateRepositoryTx — every method
// participates in the transaction tx belongs to (see
// Repository.WithTx/approvalTx.Templates).
type templateTx struct {
	tx *sql.Tx
}

// UpdateVisibility persists tmpl's Visibility and returns the updated row.
// SELECT ... FOR UPDATE existence check + UPDATE + re-SELECT replaces the
// Postgres adapter's single atomic UPDATE ... RETURNING (MySQL has no
// RETURNING) — see repository.go's package doc comment.
func (t *templateTx) UpdateVisibility(ctx context.Context, tmpl domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	var exists int
	err := t.tx.QueryRowContext(ctx, `SELECT 1 FROM templates WHERE id = ? AND tenant_id = ? FOR UPDATE`, tmpl.ID, tmpl.TenantID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: lock template: %w", err)
	}

	if _, err := t.tx.ExecContext(ctx, `
		UPDATE templates SET visibility = ?, updated_at = NOW(6) WHERE id = ? AND tenant_id = ?
	`, string(tmpl.Visibility), tmpl.ID, tmpl.TenantID); err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: update template visibility: %w", err)
	}

	var updated domain.WorkflowTemplate
	var scope, visibility string
	err = t.tx.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, dag_json, scope, COALESCE(parent_template_id, ''), version,
		       visibility, COALESCE(share_token, ''), rating_sum, rating_count
		FROM templates WHERE id = ? AND tenant_id = ?
	`, tmpl.ID, tmpl.TenantID).Scan(&updated.ID, &updated.TenantID, &updated.Name, &updated.DAGJSON, &scope, &updated.ParentTemplateID, &updated.Version,
		&visibility, &updated.ShareToken, &updated.RatingSum, &updated.RatingCount)
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("mysql: re-read updated template: %w", err)
	}
	updated.Scope = domain.Scope(scope)
	updated.Visibility = domain.Visibility(visibility)
	return updated, nil
}

// SetVisibility sets only templateID's visibility column — see
// usecase.TemplateRepositoryTx.SetVisibility's doc comment.
func (t *templateTx) SetVisibility(ctx context.Context, templateID string, v domain.Visibility) error {
	var exists int
	err := t.tx.QueryRowContext(ctx, `SELECT 1 FROM templates WHERE id = ? FOR UPDATE`, templateID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrTemplateNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: lock template: %w", err)
	}
	if _, err := t.tx.ExecContext(ctx, `UPDATE templates SET visibility = ?, updated_at = NOW(6) WHERE id = ?`, string(v), templateID); err != nil {
		return fmt.Errorf("mysql: set template visibility: %w", err)
	}
	return nil
}

// CreateExecution mirrors Repository.CreateExecution but inside this
// transaction — see usecase.TemplateRepositoryTx.CreateExecution's doc
// comment for why this exists alongside ExecutionRepository.CreateExecution.
func (t *templateTx) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO executions (id, template_id, tenant_id, status, root_trace_id, paused_at, project_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, exec.ID, nullableString(exec.TemplateID), exec.TenantID, string(exec.Status), nullableString(exec.RootTraceID), exec.PausedAt, nullableString(exec.ProjectID))
	if err != nil {
		return fmt.Errorf("mysql: insert execution (tx): %w", err)
	}
	return nil
}

// IncrementUsageCount bumps templateID's usage_count by 1 — see
// usecase.TemplateRepositoryTx.IncrementUsageCount's doc comment.
func (t *templateTx) IncrementUsageCount(ctx context.Context, templateID string) error {
	var exists int
	err := t.tx.QueryRowContext(ctx, `SELECT 1 FROM templates WHERE id = ? FOR UPDATE`, templateID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrTemplateNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: lock template: %w", err)
	}
	if _, err := t.tx.ExecContext(ctx, `UPDATE templates SET usage_count = usage_count + 1, updated_at = NOW(6) WHERE id = ?`, templateID); err != nil {
		return fmt.Errorf("mysql: increment usage count: %w", err)
	}
	return nil
}

// UpsertRating implements usecase.TemplateRepositoryTx.UpsertRating — see
// that method's doc comment. Same three-step shape as the Postgres
// adapter (lock+read prior rating, upsert the ratings row, apply the
// delta to templates' aggregate), translated to MySQL: `FOR UPDATE`
// (supported identically), `INSERT ... ON DUPLICATE KEY UPDATE` in place
// of `ON CONFLICT DO UPDATE`, and a SELECT ... FOR UPDATE existence check
// + UPDATE + re-SELECT in place of the single UPDATE ... RETURNING.
func (t *templateTx) UpsertRating(ctx context.Context, templateID, userID string, stars int32) (usecase.RateTemplateResult, error) {
	var oldStars int32
	err := t.tx.QueryRowContext(ctx, `
		SELECT stars FROM ratings WHERE template_id = ? AND user_id = ? FOR UPDATE
	`, templateID, userID).Scan(&oldStars)
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return usecase.RateTemplateResult{}, fmt.Errorf("mysql: query existing rating: %w", err)
	}

	_, err = t.tx.ExecContext(ctx, `
		INSERT INTO ratings (template_id, user_id, stars) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE stars = VALUES(stars), updated_at = NOW(6)
	`, templateID, userID, stars)
	if err != nil {
		return usecase.RateTemplateResult{}, fmt.Errorf("mysql: upsert rating: %w", err)
	}

	sumDelta := stars
	var countDelta int32
	if isNew {
		countDelta = 1
	} else {
		sumDelta = stars - oldStars
	}

	var exists int
	err = t.tx.QueryRowContext(ctx, `SELECT 1 FROM templates WHERE id = ? FOR UPDATE`, templateID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return usecase.RateTemplateResult{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return usecase.RateTemplateResult{}, fmt.Errorf("mysql: lock template: %w", err)
	}

	if _, err := t.tx.ExecContext(ctx, `
		UPDATE templates SET rating_sum = rating_sum + ?, rating_count = rating_count + ?, updated_at = NOW(6) WHERE id = ?
	`, sumDelta, countDelta, templateID); err != nil {
		return usecase.RateTemplateResult{}, fmt.Errorf("mysql: update template rating aggregate: %w", err)
	}

	var result usecase.RateTemplateResult
	if err := t.tx.QueryRowContext(ctx, `SELECT rating_sum, rating_count FROM templates WHERE id = ?`, templateID).Scan(&result.RatingSum, &result.RatingCount); err != nil {
		return usecase.RateTemplateResult{}, fmt.Errorf("mysql: re-read rating aggregate: %w", err)
	}
	return result, nil
}
