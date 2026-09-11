package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const worktreeColumns = `id, project_id, repo_id, path, branch, active, created_at,
	idempotency_key, linked_issue_provider, linked_issue_ref, status, base_ref,
	parent_worktree_id, origin, capture_source, capture_confidence, task_id,
	orchestration_run_id, coordinator_handle, created_by_terminal_handle, metadata`

// WorktreeRepository implements usecase.WorktreeRepository against
// `worktrees` — mirrors postgres.WorktreeRepository.
type WorktreeRepository struct {
	db *sql.DB
}

func NewWorktreeRepository(db *sql.DB) *WorktreeRepository {
	return &WorktreeRepository{db: db}
}

func (r *WorktreeRepository) RecordWorktreeCreated(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	return r.CreateWorktreeWithEvent(ctx, wt, event)
}

func (r *WorktreeRepository) FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+worktreeColumns+`
		FROM worktrees
		WHERE project_id = ? AND idempotency_key = ?
	`, projectID, idempotencyKey)

	out, err := scanWorktree(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Worktree{}, false, nil
	}
	if err != nil {
		return domain.Worktree{}, false, fmt.Errorf("mysql: find worktree by idempotency key: %w", err)
	}
	return out, true, nil
}

func (r *WorktreeRepository) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM worktrees WHERE id = ?`, worktreeID)
	if err != nil {
		return fmt.Errorf("mysql: delete worktree: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: rows affected: %w", err)
	}
	if n == 0 {
		return domain.ErrWorktreeNotFound
	}
	return nil
}

// ListWorktrees builds its WHERE clause dynamically — MySQL/database/sql
// has no array bind parameter equivalent to Postgres's `= ANY($2)`, so a
// non-empty statusIn becomes an `IN (?,...)` built to the right arity,
// mirroring the dynamic-IN-clause pattern this rollout already uses for
// MarkPublished below and outbox's MarkPublished precedent in
// usage-service/annotation-service.
func (r *WorktreeRepository) ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan *time.Time) ([]domain.Worktree, error) {
	query := `SELECT ` + worktreeColumns + ` FROM worktrees WHERE project_id = ?`
	args := []any{projectID}

	if len(statusIn) > 0 {
		placeholders := make([]string, len(statusIn))
		for i, s := range statusIn {
			placeholders[i] = "?"
			args = append(args, s)
		}
		query += ` AND status IN (` + strings.Join(placeholders, ",") + `)`
	}
	if olderThan != nil {
		query += ` AND created_at < ?`
		args = append(args, *olderThan)
	}
	query += ` ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query worktrees: %w", err)
	}
	defer rows.Close()

	var out []domain.Worktree
	for rows.Next() {
		wt, err := scanWorktree(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan worktree row: %w", err)
		}
		out = append(out, wt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate worktree rows: %w", err)
	}
	return out, nil
}

func (r *WorktreeRepository) GetWorktree(ctx context.Context, worktreeID string) (domain.Worktree, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+worktreeColumns+` FROM worktrees WHERE id = ?`, worktreeID)
	out, err := scanWorktree(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: get worktree: %w", err)
	}
	return out, nil
}

// SetWorktreeActivation checks existence separately — see repository.go's
// UpdateDevServerID doc comment for why (a no-op activation flip would
// otherwise look like "not found" via RowsAffected on MySQL).
func (r *WorktreeRepository) SetWorktreeActivation(ctx context.Context, worktreeID string, active bool) (domain.Worktree, error) {
	if _, err := r.GetWorktree(ctx, worktreeID); err != nil {
		return domain.Worktree{}, err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE worktrees SET active = ? WHERE id = ?`, active, worktreeID); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: update worktree activation: %w", err)
	}
	return r.GetWorktree(ctx, worktreeID)
}

func (r *WorktreeRepository) RenameWorktree(ctx context.Context, worktreeID, branch string) (domain.Worktree, error) {
	if _, err := r.GetWorktree(ctx, worktreeID); err != nil {
		return domain.Worktree{}, err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE worktrees SET branch = ? WHERE id = ?`, branch, worktreeID); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: rename worktree: %w", err)
	}
	return r.GetWorktree(ctx, worktreeID)
}

// UpdateWorktreeMeta shallow-merges patch into the stored metadata blob.
// MySQL's JSON_MERGE_PATCH (RFC 7396 merge-patch semantics — a null value
// in patch REMOVES the key, matching Postgres jsonb `||`'s explicit-null-
// overwrite behavior closely enough for this callsite: postgres.
// WorktreeRepository.UpdateWorktreeMeta's own doc comment already documents
// "an explicit JSON null in patch overwrites the corresponding key with
// null" as the frontend's "clear this field" convention — JSON_MERGE_PATCH
// deletes the key entirely on null instead of storing a literal null, which
// is the same observable "field is gone/cleared" outcome for every reader
// in this codebase (none query metadata's raw JSON shape server-side). Same
// object-only-shape contract as the Postgres version — ports.go's caller
// contract already requires patch to be a JSON object.
func (r *WorktreeRepository) UpdateWorktreeMeta(ctx context.Context, worktreeID string, patch json.RawMessage) (domain.Worktree, error) {
	if _, err := r.GetWorktree(ctx, worktreeID); err != nil {
		return domain.Worktree{}, err
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE worktrees SET metadata = JSON_MERGE_PATCH(metadata, ?) WHERE id = ?
	`, []byte(patch), worktreeID); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: update worktree metadata: %w", err)
	}
	return r.GetWorktree(ctx, worktreeID)
}

func (r *WorktreeRepository) SetWorktreeLineage(ctx context.Context, worktreeID string, parentWorktreeID *string) (domain.Worktree, error) {
	if _, err := r.GetWorktree(ctx, worktreeID); err != nil {
		return domain.Worktree{}, err
	}
	var captureConfidence *string
	if parentWorktreeID != nil {
		explicit := "explicit"
		captureConfidence = &explicit
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE worktrees SET parent_worktree_id = ?, capture_confidence = ? WHERE id = ?
	`, ptrToAny(parentWorktreeID), ptrToAny(captureConfidence), worktreeID); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: set worktree lineage: %w", err)
	}
	return r.GetWorktree(ctx, worktreeID)
}

func scanWorktree(row rowScanner) (domain.Worktree, error) {
	var wt domain.Worktree
	var status string
	var idempotencyKey, linkedIssueProvider, linkedIssueRef, baseRef sql.NullString
	var parentWorktreeID, origin, captureSource, captureConfidence, taskID, orchestrationRunID, coordinatorHandle, createdByTerminalHandle sql.NullString
	var metadata []byte
	if err := row.Scan(
		&wt.ID, &wt.ProjectID, &wt.RepoID, &wt.Path, &wt.Branch, &wt.Active, &wt.CreatedAt,
		&idempotencyKey, &linkedIssueProvider, &linkedIssueRef, &status, &baseRef,
		&parentWorktreeID, &origin, &captureSource, &captureConfidence, &taskID,
		&orchestrationRunID, &coordinatorHandle, &createdByTerminalHandle, &metadata,
	); err != nil {
		return domain.Worktree{}, err
	}
	wt.Status = domain.WorktreeStatus(status)
	wt.IdempotencyKey = nullStringPtr(idempotencyKey)
	wt.LinkedIssueProvider = linkedIssueProvider.String
	wt.LinkedIssueRef = linkedIssueRef.String
	wt.BaseRef = nullStringPtr(baseRef)
	wt.ParentWorktreeID = nullStringPtr(parentWorktreeID)
	wt.Origin = nullStringPtr(origin)
	wt.CaptureSource = nullStringPtr(captureSource)
	wt.CaptureConfidence = nullStringPtr(captureConfidence)
	wt.TaskID = nullStringPtr(taskID)
	wt.OrchestrationRunID = nullStringPtr(orchestrationRunID)
	wt.CoordinatorHandle = nullStringPtr(coordinatorHandle)
	wt.CreatedByTerminalHandle = nullStringPtr(createdByTerminalHandle)
	if len(metadata) > 0 {
		wt.Metadata = json.RawMessage(metadata)
	}
	return wt, nil
}

// CreateWorktreeWithEvent inserts worktree and its worktree.created outbox
// event in ONE transaction — mirrors postgres.WorktreeRepository's identical
// method. metadata is deliberately left out of the INSERT column list, same
// as the Postgres version omitting it from its own INSERT — both rely on
// the column DEFAULT ('{}'::jsonb / (JSON_OBJECT())) to seed it.
func (r *WorktreeRepository) CreateWorktreeWithEvent(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	status := wt.Status
	if status == "" {
		status = domain.WorktreeStatusActive
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO worktrees (
			id, project_id, repo_id, path, branch, active, created_at, idempotency_key, linked_issue_provider, linked_issue_ref, status, base_ref,
			parent_worktree_id, origin, capture_source, capture_confidence, task_id,
			orchestration_run_id, coordinator_handle, created_by_terminal_handle
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, wt.ID, wt.ProjectID, wt.RepoID, wt.Path, wt.Branch, wt.Active, time.Now().UTC(), ptrToAny(wt.IdempotencyKey), nullableString(wt.LinkedIssueProvider), nullableString(wt.LinkedIssueRef), string(status), ptrToAny(wt.BaseRef),
		ptrToAny(wt.ParentWorktreeID), ptrToAny(wt.Origin), ptrToAny(wt.CaptureSource), ptrToAny(wt.CaptureConfidence), ptrToAny(wt.TaskID),
		ptrToAny(wt.OrchestrationRunID), ptrToAny(wt.CoordinatorHandle), ptrToAny(wt.CreatedByTerminalHandle),
	); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: insert worktree: %w", err)
	}

	row := tx.QueryRowContext(ctx, `SELECT `+worktreeColumns+` FROM worktrees WHERE id = ?`, wt.ID)
	out, err := scanWorktree(row)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: read back inserted worktree: %w", err)
	}

	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return domain.Worktree{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Worktree{}, fmt.Errorf("mysql: commit worktree+event tx: %w", err)
	}
	return out, nil
}

// insertOutboxEvent is the shared INSERT both CreateWorktreeWithEvent and
// RemoveWorktreeWithEvent use — factored out since both need the identical
// statement inside their own transaction.
func insertOutboxEvent(ctx context.Context, tx *sql.Tx, event domain.OutboxEvent) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?, ?, ?, ?, 1, ?)
	`, event.ID, event.TenantID, event.Subject, event.OccurredAt, []byte(event.PayloadJSON)); err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

func (r *WorktreeRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	return fetchUnpublishedOutbox(ctx, r.db, limit)
}

// RemoveWorktreeWithEvent deletes the row and enqueues worktree.deleted in
// the same transaction — no DELETE...RETURNING in MySQL, so this SELECTs
// the row first (inside the same transaction, so it's consistent with the
// DELETE that follows), then deletes it.
func (r *WorktreeRepository) RemoveWorktreeWithEvent(ctx context.Context, worktreeID string, buildEvent func(removed domain.Worktree) domain.OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `SELECT `+worktreeColumns+` FROM worktrees WHERE id = ?`, worktreeID)
	removed, err := scanWorktree(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrWorktreeNotFound
	}
	if err != nil {
		return fmt.Errorf("mysql: read worktree before delete: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM worktrees WHERE id = ?`, worktreeID); err != nil {
		return fmt.Errorf("mysql: delete worktree: %w", err)
	}

	event := buildEvent(removed)
	if err := insertOutboxEvent(ctx, tx, event); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit worktree+event tx: %w", err)
	}
	return nil
}

func (r *WorktreeRepository) MarkPublished(ctx context.Context, ids []string) error {
	return markOutboxPublished(ctx, r.db, ids)
}

// ptrToAny binds a *string as either SQL NULL (nil pointer) or its
// dereferenced value — the MySQL-side counterpart to nullableString for
// fields that are already *string in the domain layer (lineage fields,
// IdempotencyKey, BaseRef) rather than ""-means-unset plain strings.
func ptrToAny(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// nullStringPtr converts a scanned sql.NullString back into the *string
// shape domain.Worktree's nullable fields use — nil when the column was
// SQL NULL, matching pgx's native *string scan behavior that
// postgres.scanWorktree relies on implicitly.
func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

// ListLineage returns every worktree with an explicitly-captured parent —
// tenant-scoping note: see ListReposForTenant's identical doc comment (no
// RLS on MySQL, but this query was never RLS-scoped in any way other
// application code already relies on — see usecase.ListWorktreeLineage's
// doc comment referenced by ports.go).
func (r *WorktreeRepository) ListLineage(ctx context.Context) ([]domain.Worktree, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+worktreeColumns+`
		FROM worktrees
		WHERE parent_worktree_id IS NOT NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("mysql: query worktree lineage: %w", err)
	}
	defer rows.Close()

	var out []domain.Worktree
	for rows.Next() {
		wt, err := scanWorktree(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan worktree lineage row: %w", err)
		}
		out = append(out, wt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate worktree lineage rows: %w", err)
	}
	return out, nil
}
