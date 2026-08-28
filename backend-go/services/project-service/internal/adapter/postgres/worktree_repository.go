package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const worktreeColumns = `id, project_id, repo_id, path, branch, active, idempotency_key, COALESCE(linked_issue_provider, ''), COALESCE(linked_issue_ref, ''), status, base_ref`

// WorktreeRepository implements usecase.WorktreeRepository against
// project.worktrees.
type WorktreeRepository struct {
	pool *pgxpool.Pool
}

func NewWorktreeRepository(pool *pgxpool.Pool) *WorktreeRepository {
	return &WorktreeRepository{pool: pool}
}

// RecordWorktreeCreated inserts the worktree row and enqueues event as an
// outbox row — both in ONE transaction (the transactional-outbox pattern,
// specs/backend-go/architecture/05-data-architecture.md), mirroring the
// real, working pattern usage-service.Repository.SaveSession already ships.
// Identical in contract to CreateWorktreeWithEvent — both interface methods
// share the same transactional-outbox shape; see usecase.WorktreeRepository's
// doc comment.
func (r *WorktreeRepository) RecordWorktreeCreated(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	return r.CreateWorktreeWithEvent(ctx, wt, event)
}

// FindWorktreeByIdempotencyKey backs BR-CLI-01 — a second CreateWorktree
// call with the same (project_id, idempotency_key) returns the existing row
// instead of git-gateway-service re-running `git worktree add`.
// found=false, err=nil means "no match yet", not an error.
func (r *WorktreeRepository) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+worktreeColumns+`
		FROM project.worktrees
		WHERE project_id = $1 AND idempotency_key = $2
	`, projectID, idempotencyKey)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, false, nil
	}
	if err != nil {
		return domain.Worktree{}, false, fmt.Errorf("postgres: find worktree by idempotency key: %w", err)
	}
	return out, true, nil
}

// RecordWorktreeRemoved hard-deletes the worktree row — see
// usecase.WorktreeRepository.RecordWorktreeRemoved's doc comment for the
// decision.
func (r *WorktreeRepository) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM project.worktrees WHERE id = $1`, worktreeID)
	if err != nil {
		return fmt.Errorf("postgres: delete worktree: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrWorktreeNotFound
	}
	return nil
}

// ListWorktrees returns projectID's worktrees, optionally filtered by
// statusIn (nil/empty = no filter) and olderThan (nil = no filter, else
// created_at < *olderThan) — BL-AT-04's cleanup_worktrees step candidate
// query. Every existing caller (passing nil, nil) sees unchanged behavior.
func (r *WorktreeRepository) ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan *time.Time) ([]domain.Worktree, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+worktreeColumns+`
		FROM project.worktrees
		WHERE project_id = $1
		  AND ($2::text[] IS NULL OR status = ANY($2))
		  AND ($3::timestamptz IS NULL OR created_at < $3)
		ORDER BY id
	`, projectID, nullableStringSlice(statusIn), olderThan)
	if err != nil {
		return nil, fmt.Errorf("postgres: query worktrees: %w", err)
	}
	defer rows.Close()

	var out []domain.Worktree
	for rows.Next() {
		wt, err := scanWorktree(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan worktree row: %w", err)
		}
		out = append(out, wt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate worktree rows: %w", err)
	}
	return out, nil
}

// GetWorktree looks up a single worktree by id — backs the new GetWorktree
// RPC (SOL-WT-04), used by git-gateway-service's CompareWorktrees to fetch
// each compared worktree's repo_id/branch/base_ref.
func (r *WorktreeRepository) GetWorktree(ctx context.Context, worktreeID string) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+worktreeColumns+` FROM project.worktrees WHERE id = $1`, worktreeID)
	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: get worktree: %w", err)
	}
	return out, nil
}

func (r *WorktreeRepository) SetWorktreeActivation(ctx context.Context, worktreeID string, active bool) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.worktrees SET active = $1 WHERE id = $2
		RETURNING `+worktreeColumns,
		active, worktreeID,
	)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: update worktree activation: %w", err)
	}
	return out, nil
}

func (r *WorktreeRepository) RenameWorktree(ctx context.Context, worktreeID, branch string) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.worktrees SET branch = $1 WHERE id = $2
		RETURNING `+worktreeColumns,
		branch, worktreeID,
	)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: rename worktree: %w", err)
	}
	return out, nil
}

// scanWorktree does NOT convert pgx.ErrNoRows itself — see
// scanProjectGroup's identical doc comment for why (every caller already
// checks errors.Is(err, pgx.ErrNoRows) against the raw scan error).
func scanWorktree(row rowScanner) (domain.Worktree, error) {
	var wt domain.Worktree
	var status string
	if err := row.Scan(&wt.ID, &wt.ProjectID, &wt.RepoID, &wt.Path, &wt.Branch, &wt.Active, &wt.IdempotencyKey, &wt.LinkedIssueProvider, &wt.LinkedIssueRef, &status, &wt.BaseRef); err != nil {
		return domain.Worktree{}, err
	}
	wt.Status = domain.WorktreeStatus(status)
	return wt, nil
}

// CreateWorktreeWithEvent inserts worktree and its worktree.created outbox
// event in ONE transaction (SOL-PI-03) — both-or-neither persists. See
// usage-service's SaveSession(ctx, session, event) for the precedent this
// follows.
func (r *WorktreeRepository) CreateWorktreeWithEvent(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	status := wt.Status
	if status == "" {
		status = domain.WorktreeStatusActive
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	row := tx.QueryRow(ctx, `
		INSERT INTO project.worktrees (id, project_id, repo_id, path, branch, active, idempotency_key, linked_issue_provider, linked_issue_ref, status, base_ref)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10, $11)
		RETURNING `+worktreeColumns,
		wt.ID, wt.ProjectID, wt.RepoID, wt.Path, wt.Branch, wt.Active, wt.IdempotencyKey, wt.LinkedIssueProvider, wt.LinkedIssueRef, string(status), wt.BaseRef,
	)
	out, err := scanWorktree(row)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert worktree: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO project.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: commit worktree+event tx: %w", err)
	}
	return out, nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired. Kept on the same
// WorktreeRepository as RecordWorktreeCreated since both operate on this
// service's own database; no domain reason to split them into a separate
// type.
func (r *WorktreeRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM project.outbox_events
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

// RemoveWorktreeWithEvent deletes the row and enqueues worktree.deleted in
// the same transaction — buildEvent runs against the just-deleted row's
// linked-issue fields (DELETE ... RETURNING), so the caller never needs a
// separate pre-delete read.
func (r *WorktreeRepository) RemoveWorktreeWithEvent(ctx context.Context, worktreeID string, buildEvent func(removed domain.Worktree) domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	row := tx.QueryRow(ctx, `DELETE FROM project.worktrees WHERE id = $1 RETURNING `+worktreeColumns, worktreeID)
	removed, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrWorktreeNotFound
	}
	if err != nil {
		return fmt.Errorf("postgres: delete worktree: %w", err)
	}

	event := buildEvent(removed)
	if _, err := tx.Exec(ctx, `
		INSERT INTO project.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, event.TenantID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit worktree+event tx: %w", err)
	}
	return nil
}

func (r *WorktreeRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE project.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

// nullableStringSlice returns nil for an empty slice so the SQL's
// `$2::text[] IS NULL` branch is taken — a Go nil slice already encodes as
// SQL NULL via pgx, but an explicit empty (non-nil) slice from a caller
// would otherwise encode as an empty array, which `= ANY('{}')` never
// matches (not "no filter").
func nullableStringSlice(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
