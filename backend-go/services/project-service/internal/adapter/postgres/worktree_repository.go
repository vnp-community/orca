package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const worktreeColumns = `id, project_id, repo_id, path, branch, active, created_at,
	parent_worktree_id, origin, capture_source, capture_confidence, task_id,
	orchestration_run_id, coordinator_handle, created_by_terminal_handle, metadata`

// WorktreeRepository implements usecase.WorktreeRepository against
// project.worktrees.
type WorktreeRepository struct {
	pool *pgxpool.Pool
}

func NewWorktreeRepository(pool *pgxpool.Pool) *WorktreeRepository {
	return &WorktreeRepository{pool: pool}
}

func (r *WorktreeRepository) RecordWorktreeCreated(ctx context.Context, wt domain.Worktree) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.worktrees (
			id, project_id, repo_id, path, branch, active,
			parent_worktree_id, origin, capture_source, capture_confidence, task_id,
			orchestration_run_id, coordinator_handle, created_by_terminal_handle
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING `+worktreeColumns,
		wt.ID, wt.ProjectID, wt.RepoID, wt.Path, wt.Branch, wt.Active,
		wt.ParentWorktreeID, wt.Origin, wt.CaptureSource, wt.CaptureConfidence, wt.TaskID,
		wt.OrchestrationRunID, wt.CoordinatorHandle, wt.CreatedByTerminalHandle,
	)

	out, err := scanWorktree(row)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert worktree: %w", err)
	}
	return out, nil
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

func (r *WorktreeRepository) ListWorktrees(ctx context.Context, projectID string) ([]domain.Worktree, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+worktreeColumns+`
		FROM project.worktrees
		WHERE project_id = $1
		ORDER BY id
	`, projectID)
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

// UpdateWorktreeMeta shallow-merges patch (a JSON object) into the stored
// metadata blob via Postgres jsonb's `||` operator — an explicit JSON null
// in patch overwrites the corresponding key with null (the frontend's own
// "clear this field" wire convention, see encodePushTargetClearForRuntimeRpc),
// an omitted key leaves the previously-stored value untouched. patch must be
// a JSON object, never an array/scalar — `||` on two jsonb objects merges;
// on anything else it behaves unpredictably, so ports.go's caller contract
// requires object shape.
func (r *WorktreeRepository) UpdateWorktreeMeta(ctx context.Context, worktreeID string, patch json.RawMessage) (domain.Worktree, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.worktrees SET metadata = metadata || $1::jsonb WHERE id = $2
		RETURNING `+worktreeColumns,
		[]byte(patch), worktreeID,
	)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: update worktree metadata: %w", err)
	}
	return out, nil
}

// SetWorktreeLineage re-parents (or, when parentWorktreeID is nil, clears
// the parent of) an already-created worktree. capture_confidence tracks
// parent_worktree_id's presence the same way NewWorktree's creation-time
// capture does (see domain.NewWorktree's doc comment) — "explicit" when a
// parent is set, cleared alongside it when removed.
func (r *WorktreeRepository) SetWorktreeLineage(ctx context.Context, worktreeID string, parentWorktreeID *string) (domain.Worktree, error) {
	var captureConfidence *string
	if parentWorktreeID != nil {
		explicit := "explicit"
		captureConfidence = &explicit
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE project.worktrees SET parent_worktree_id = $1, capture_confidence = $2 WHERE id = $3
		RETURNING `+worktreeColumns,
		parentWorktreeID, captureConfidence, worktreeID,
	)

	out, err := scanWorktree(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Worktree{}, domain.ErrWorktreeNotFound
	}
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: set worktree lineage: %w", err)
	}
	return out, nil
}

// scanWorktree does NOT convert pgx.ErrNoRows itself — see
// scanProjectGroup's identical doc comment for why (every caller already
// checks errors.Is(err, pgx.ErrNoRows) against the raw scan error).
func scanWorktree(row rowScanner) (domain.Worktree, error) {
	var wt domain.Worktree
	if err := row.Scan(
		&wt.ID, &wt.ProjectID, &wt.RepoID, &wt.Path, &wt.Branch, &wt.Active, &wt.CreatedAt,
		&wt.ParentWorktreeID, &wt.Origin, &wt.CaptureSource, &wt.CaptureConfidence, &wt.TaskID,
		&wt.OrchestrationRunID, &wt.CoordinatorHandle, &wt.CreatedByTerminalHandle, &wt.Metadata,
	); err != nil {
		return domain.Worktree{}, err
	}
	return wt, nil
}

// ListLineage returns every worktree with an explicitly-captured parent,
// tenant-scoped implicitly via the same RLS policy every other query
// against this table relies on (see 0004_worktrees.up.sql's tenant_isolation
// policy) — no explicit tenant filter needed here.
func (r *WorktreeRepository) ListLineage(ctx context.Context) ([]domain.Worktree, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+worktreeColumns+`
		FROM project.worktrees
		WHERE parent_worktree_id IS NOT NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("postgres: query worktree lineage: %w", err)
	}
	defer rows.Close()

	var out []domain.Worktree
	for rows.Next() {
		wt, err := scanWorktree(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan worktree lineage row: %w", err)
		}
		out = append(out, wt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate worktree lineage rows: %w", err)
	}
	return out, nil
}
