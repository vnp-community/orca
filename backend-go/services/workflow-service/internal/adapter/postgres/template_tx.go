package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// WithTx implements usecase.TemplateRepository.WithTx — begins a Postgres
// transaction, hands fn a templateTx scoped to it, commits on nil and
// rolls back on any error fn returns (including a panic recovered nowhere
// here — Rollback's own no-op-after-Commit semantics make the deferred
// Rollback safe to call unconditionally after a successful Commit).
func (r *Repository) WithTx(ctx context.Context, fn func(tx usecase.TemplateRepositoryTx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(&templateTx{tx: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit tx: %w", err)
	}
	return nil
}

// GetByShareToken looks up the (at most one) template whose share_token
// matches — backs PreviewSharedTemplate/ImportSharedTemplate.
func (r *Repository) GetByShareToken(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version,
		       visibility, COALESCE(share_token, ''), rating_sum, rating_count
		FROM workflow.templates
		WHERE share_token = $1
	`, shareToken)

	var tmpl domain.WorkflowTemplate
	var scope, visibility string
	err := row.Scan(&tmpl.ID, &tmpl.TenantID, &tmpl.Name, &tmpl.DAGJSON, &scope, &tmpl.ParentTemplateID, &tmpl.Version,
		&visibility, &tmpl.ShareToken, &tmpl.RatingSum, &tmpl.RatingCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: query template by share token: %w", err)
	}
	tmpl.Scope = domain.Scope(scope)
	tmpl.Visibility = domain.Visibility(visibility)
	return tmpl, nil
}

// SetShareToken mints (or overwrites) templateID's share_token — backs
// usecase.GenerateShareLink. Not tenant-scoped in its WHERE clause the
// caller (usecase.GenerateShareLink) already confirmed ownership via
// GetTemplate before calling this, same convention as Update's
// already-confirmed-exists precondition.
func (r *Repository) SetShareToken(ctx context.Context, templateID, token string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE workflow.templates SET share_token = $1, updated_at = now() WHERE id = $2`, token, templateID)
	if err != nil {
		return fmt.Errorf("postgres: set share token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// templateTx implements usecase.TemplateRepositoryTx — every method
// participates in the transaction tx belongs to (see
// Repository.WithTx/ApprovalRepositoryTx.Templates).
type templateTx struct {
	tx pgx.Tx
}

// UpdateVisibility persists tmpl's Visibility and returns the updated row —
// the direct-apply path (see usecase.TemplateRepositoryTx.UpdateVisibility's
// doc comment).
func (t *templateTx) UpdateVisibility(ctx context.Context, tmpl domain.WorkflowTemplate) (domain.WorkflowTemplate, error) {
	row := t.tx.QueryRow(ctx, `
		UPDATE workflow.templates
		SET visibility = $1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3
		RETURNING id, tenant_id, name, dag_json::text, scope, COALESCE(parent_template_id::text, ''), version,
		          visibility, COALESCE(share_token, ''), rating_sum, rating_count
	`, string(tmpl.Visibility), tmpl.ID, tmpl.TenantID)

	var updated domain.WorkflowTemplate
	var scope, visibility string
	err := row.Scan(&updated.ID, &updated.TenantID, &updated.Name, &updated.DAGJSON, &scope, &updated.ParentTemplateID, &updated.Version,
		&visibility, &updated.ShareToken, &updated.RatingSum, &updated.RatingCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.WorkflowTemplate{}, domain.ErrTemplateNotFound
	}
	if err != nil {
		return domain.WorkflowTemplate{}, fmt.Errorf("postgres: update template visibility: %w", err)
	}
	updated.Scope = domain.Scope(scope)
	updated.Visibility = domain.Visibility(visibility)
	return updated, nil
}

// SetVisibility sets only templateID's visibility column — see
// usecase.TemplateRepositoryTx.SetVisibility's doc comment.
func (t *templateTx) SetVisibility(ctx context.Context, templateID string, v domain.Visibility) error {
	tag, err := t.tx.Exec(ctx, `UPDATE workflow.templates SET visibility = $1, updated_at = now() WHERE id = $2`, string(v), templateID)
	if err != nil {
		return fmt.Errorf("postgres: set template visibility: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// CreateExecution mirrors Repository.CreateExecution but inside this
// transaction — see usecase.TemplateRepositoryTx.CreateExecution's doc
// comment for why this exists alongside ExecutionRepository.CreateExecution.
func (t *templateTx) CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO workflow.executions (id, template_id, tenant_id, status, root_trace_id, paused_at, project_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, exec.ID, nullableString(exec.TemplateID), exec.TenantID, string(exec.Status), nullableString(exec.RootTraceID), exec.PausedAt, nullableString(exec.ProjectID))
	if err != nil {
		return fmt.Errorf("postgres: insert execution (tx): %w", err)
	}
	return nil
}

// IncrementUsageCount bumps templateID's usage_count by 1 — see
// usecase.TemplateRepositoryTx.IncrementUsageCount's doc comment.
func (t *templateTx) IncrementUsageCount(ctx context.Context, templateID string) error {
	tag, err := t.tx.Exec(ctx, `UPDATE workflow.templates SET usage_count = usage_count + 1, updated_at = now() WHERE id = $1`, templateID)
	if err != nil {
		return fmt.Errorf("postgres: increment usage count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

// UpsertRating implements usecase.TemplateRepositoryTx.UpsertRating — see
// that method's doc comment. Three steps, one transaction: (1) lock and
// read the caller's PRIOR rating for this template, if any (FOR UPDATE —
// this read-then-write needs the row locked against a concurrent rating
// from the same user landing between the read and the aggregate update
// below); (2) upsert the ratings row; (3) apply the delta (new-old stars,
// and +1 to rating_count only for a genuinely new rating) to
// templates.rating_sum/rating_count in the same statement, RETURNING the
// post-write aggregate.
func (t *templateTx) UpsertRating(ctx context.Context, templateID, userID string, stars int32) (usecase.RateTemplateResult, error) {
	var oldStars int32
	err := t.tx.QueryRow(ctx, `
		SELECT stars FROM workflow.ratings WHERE template_id = $1 AND user_id = $2 FOR UPDATE
	`, templateID, userID).Scan(&oldStars)
	isNew := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !isNew {
		return usecase.RateTemplateResult{}, fmt.Errorf("postgres: query existing rating: %w", err)
	}

	_, err = t.tx.Exec(ctx, `
		INSERT INTO workflow.ratings (template_id, user_id, stars)
		VALUES ($1, $2, $3)
		ON CONFLICT (template_id, user_id) DO UPDATE SET stars = EXCLUDED.stars, updated_at = now()
	`, templateID, userID, stars)
	if err != nil {
		return usecase.RateTemplateResult{}, fmt.Errorf("postgres: upsert rating: %w", err)
	}

	sumDelta := stars
	var countDelta int32
	if isNew {
		countDelta = 1
	} else {
		sumDelta = stars - oldStars
	}

	row := t.tx.QueryRow(ctx, `
		UPDATE workflow.templates
		SET rating_sum = rating_sum + $1, rating_count = rating_count + $2, updated_at = now()
		WHERE id = $3
		RETURNING rating_sum, rating_count
	`, sumDelta, countDelta, templateID)

	var result usecase.RateTemplateResult
	if err := row.Scan(&result.RatingSum, &result.RatingCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.RateTemplateResult{}, domain.ErrTemplateNotFound
		}
		return usecase.RateTemplateResult{}, fmt.Errorf("postgres: update template rating aggregate: %w", err)
	}
	return result, nil
}
