package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

var _ usecase.OpenSpecChangeRepository = (*OpenSpecChangeRepository)(nil)

type OpenSpecChangeRepository struct {
	*Repository
}

func NewOpenSpecChangeRepository(r *Repository) *OpenSpecChangeRepository {
	return &OpenSpecChangeRepository{Repository: r}
}

const openSpecChangeColumns = `id, request_id, change_id, COALESCE(branch, ''), status, tasks_sync_state, COALESCE(commit_sha, ''),
	COALESCE(tasks_md_digest, ''), kg_available, COALESCE(kg_snapshot, ''), kg_read_ids, kg_gaps, kg_conflicts, COALESCE(skill_hash, ''),
	bundle_json, COALESCE(workspace_id, ''), redacted_secrets,
	COALESCE(tasks_sync_digest, ''), COALESCE(tasks_sync_error, ''), tasks_sync_attempts, tasks_sync_next_retry_at, version`

func scanOpenSpecChange(row rowScanner) (domain.OpenSpecChange, error) {
	return scanOpenSpecChangeInto(row)
}

// scanOpenSpecChangeInto reads the change columns and then any extra destinations the query selected after them.
func scanOpenSpecChangeInto(row rowScanner, extra ...any) (domain.OpenSpecChange, error) {
	var c domain.OpenSpecChange
	var status, syncState string
	var read, gaps, conflicts, bundle []byte
	var retry sql.NullTime
	dest := append([]any{&c.ID, &c.RequestID, &c.ChangeID, &c.Branch, &status, &syncState, &c.Commit, &c.TasksMDDigest, &c.KGAvailable,
		&c.KGSnapshot, &read, &gaps, &conflicts, &c.SkillHash, &bundle, &c.WorkspaceID, &c.RedactedSecrets,
		&c.SyncDigest, &c.SyncError, &c.SyncAttempts, &retry, &c.Version}, extra...)
	if err := row.Scan(dest...); err != nil {
		return domain.OpenSpecChange{}, err
	}
	c.Status, c.SyncState = domain.ChangeStatus(status), domain.SyncState(syncState)
	if retry.Valid {
		at := retry.Time.UTC()
		c.SyncNextRetryAt = &at
	}
	for _, f := range []struct {
		raw []byte
		to  *[]string
	}{{read, &c.KGReadIDs}, {gaps, &c.KGGaps}, {conflicts, &c.KGConflicts}} {
		if len(f.raw) > 0 {
			if err := json.Unmarshal(f.raw, f.to); err != nil {
				return domain.OpenSpecChange{}, fmt.Errorf("mysql: openspec change list column: %w", err)
			}
		}
	}
	if len(bundle) > 0 {
		if err := json.Unmarshal(bundle, &c.Bundle); err != nil {
			return domain.OpenSpecChange{}, fmt.Errorf("mysql: openspec change bundle: %w", err)
		}
	}
	return c, nil
}

func (r *OpenSpecChangeRepository) GetByRequest(ctx context.Context, requestID string) (domain.OpenSpecChange, bool, error) {
	var c domain.OpenSpecChange
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		var err error
		c, found, err = getOpenSpecChange(ctx, db, tenantID, requestID)
		return err
	})
	return c, found, err
}

// The tenant comes from the scoped context, never a constant: MySQL has no RLS, so without it another tenant's change would be readable.
func getOpenSpecChange(ctx context.Context, db dbExecer, tenantID, requestID string) (domain.OpenSpecChange, bool, error) {
	c, err := scanOpenSpecChange(db.QueryRowContext(ctx, `SELECT `+openSpecChangeColumns+` FROM openspec_changes WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OpenSpecChange{}, false, nil
	}
	if err != nil {
		return domain.OpenSpecChange{}, false, fmt.Errorf("mysql get openspec change: %w", err)
	}
	return c, true, nil
}

// Upsert writes the whole change for its Request: a first call inserts, later calls replace every field and bump the version.
func (r *OpenSpecChangeRepository) Upsert(ctx context.Context, c domain.OpenSpecChange) (domain.OpenSpecChange, error) {
	var out domain.OpenSpecChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		read, gaps, conflicts, bundle, err := encodeChangeJSON(c)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO openspec_changes (id, tenant_id, request_id, change_id, branch, status, tasks_sync_state, commit_sha,
				tasks_md_digest, kg_available, kg_snapshot, kg_read_ids, kg_gaps, kg_conflicts, skill_hash, bundle_json, workspace_id,
				tasks_sync_digest, tasks_sync_error, tasks_sync_attempts, tasks_sync_next_retry_at, redacted_secrets)
			VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''),
				NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				change_id = VALUES(change_id), branch = VALUES(branch), status = VALUES(status), tasks_sync_state = VALUES(tasks_sync_state),
				commit_sha = VALUES(commit_sha), tasks_md_digest = VALUES(tasks_md_digest), kg_available = VALUES(kg_available),
				kg_snapshot = VALUES(kg_snapshot), kg_read_ids = VALUES(kg_read_ids), kg_gaps = VALUES(kg_gaps), kg_conflicts = VALUES(kg_conflicts),
				skill_hash = VALUES(skill_hash), bundle_json = VALUES(bundle_json), workspace_id = VALUES(workspace_id),
				tasks_sync_digest = VALUES(tasks_sync_digest), tasks_sync_error = VALUES(tasks_sync_error),
				tasks_sync_attempts = VALUES(tasks_sync_attempts), tasks_sync_next_retry_at = VALUES(tasks_sync_next_retry_at),
				redacted_secrets = VALUES(redacted_secrets),
				updated_at = CURRENT_TIMESTAMP(6), version = version + 1`,
			c.ID, tenantID, c.RequestID, c.ChangeID, c.Branch, string(c.Status), string(c.SyncState), c.Commit, c.TasksMDDigest, c.KGAvailable,
			c.KGSnapshot, read, gaps, conflicts, c.SkillHash, bundle, c.WorkspaceID,
			c.SyncDigest, c.SyncError, c.SyncAttempts, c.SyncNextRetryAt, c.RedactedSecrets); err != nil {
			return fmt.Errorf("mysql upsert openspec change: %w", err)
		}
		var found bool
		out, found, err = getOpenSpecChange(ctx, db, tenantID, c.RequestID)
		if err == nil && !found {
			err = fmt.Errorf("mysql: openspec change vanished after upsert")
		}
		return err
	})
	return out, err
}

func encodeChangeJSON(c domain.OpenSpecChange) (read, gaps, conflicts, bundle any, err error) {
	enc := func(v any) (any, error) {
		b, err := json.Marshal(v)
		return string(b), err
	}
	if read, err = enc(nonNilStrings(c.KGReadIDs)); err != nil {
		return
	}
	if gaps, err = enc(nonNilStrings(c.KGGaps)); err != nil {
		return
	}
	if conflicts, err = enc(nonNilStrings(c.KGConflicts)); err != nil {
		return
	}
	if c.Bundle == nil {
		return read, gaps, conflicts, nil, nil
	}
	bundle, err = enc(c.Bundle)
	return
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// UpdateSync moves the sync state of the change at expectedVersion; a stale version is a conflict, never a silent overwrite.
func (r *OpenSpecChangeRepository) UpdateSync(ctx context.Context, id string, u domain.SyncUpdate, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `
			UPDATE openspec_changes
			SET tasks_sync_state = ?, tasks_sync_digest = NULLIF(?, ''), tasks_sync_error = NULLIF(?, ''), tasks_sync_attempts = ?,
				tasks_sync_next_retry_at = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
			WHERE id = ? AND tenant_id = ? AND version = ?`, string(u.State), u.Digest, u.Error, u.Attempts, u.NextRetryAt, id, tenantID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: update openspec change sync: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return nil
		}
		var exists int
		err = db.QueryRowContext(ctx, `SELECT 1 FROM openspec_changes WHERE id = ? AND tenant_id = ?`, id, tenantID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrOpenSpecChangeNotFound
		}
		if err != nil {
			return fmt.Errorf("mysql: classify openspec change update: %w", err)
		}
		return domain.ErrOpenSpecChangeVersionConflict
	})
}

// ListSyncWork lists ready changes that have something to do and whose retry time has come: tasks.md still has to catch up
// with task progress, or the Request is completed and the change is waiting to be archived. A failed change is left alone.
func (r *OpenSpecChangeRepository) ListSyncWork(ctx context.Context, limit int) ([]domain.OpenSpecChange, error) {
	var out []domain.OpenSpecChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+openSpecChangeColumns+` FROM openspec_changes
			WHERE tenant_id = ? AND status = 'ready' AND tasks_sync_state <> 'failed'
				AND (tasks_sync_next_retry_at IS NULL OR tasks_sync_next_retry_at <= CURRENT_TIMESTAMP(6))
				AND (tasks_sync_state = 'pending' OR request_id IN (SELECT id FROM requests WHERE tenant_id = ? AND status = 'completed'))
			ORDER BY updated_at, id LIMIT ?`, tenantID, tenantID, limit)
		if err != nil {
			return fmt.Errorf("mysql: list pending openspec changes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanOpenSpecChange(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

// ListWithWorkspace lists the tenant's changes that still hold a worktree. The Request status comes from a second query so
// the change columns stay unambiguous.
func (r *OpenSpecChangeRepository) ListWithWorkspace(ctx context.Context, limit int) ([]usecase.ProposalCleanupCandidate, error) {
	var out []usecase.ProposalCleanupCandidate
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+openSpecChangeColumns+`, updated_at FROM openspec_changes
			WHERE tenant_id = ? AND COALESCE(workspace_id, '') <> '' ORDER BY updated_at, id LIMIT ?`, tenantID, limit)
		if err != nil {
			return fmt.Errorf("mysql: list openspec changes with a worktree: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var cand usecase.ProposalCleanupCandidate
			if cand.Change, err = scanOpenSpecChangeInto(rows, &cand.UpdatedAt); err != nil {
				return err
			}
			out = append(out, cand)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for i := range out {
			var st string
			err := db.QueryRowContext(ctx, `SELECT status FROM requests WHERE tenant_id = ? AND id = ?`, tenantID, out[i].Change.RequestID).Scan(&st)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("mysql: read request status: %w", err)
			}
			out[i].RequestStatus = domain.RequestStatus(st)
		}
		return nil
	})
	return out, err
}
