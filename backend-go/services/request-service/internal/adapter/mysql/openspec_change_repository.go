package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	bundle_json, COALESCE(workspace_id, ''), version`

func scanOpenSpecChange(row rowScanner) (domain.OpenSpecChange, error) {
	var c domain.OpenSpecChange
	var status, syncState string
	var read, gaps, conflicts, bundle []byte
	if err := row.Scan(&c.ID, &c.RequestID, &c.ChangeID, &c.Branch, &status, &syncState, &c.Commit, &c.TasksMDDigest, &c.KGAvailable,
		&c.KGSnapshot, &read, &gaps, &conflicts, &c.SkillHash, &bundle, &c.WorkspaceID, &c.Version); err != nil {
		return domain.OpenSpecChange{}, err
	}
	c.Status, c.SyncState = domain.ChangeStatus(status), domain.SyncState(syncState)
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
				tasks_md_digest, kg_available, kg_snapshot, kg_read_ids, kg_gaps, kg_conflicts, skill_hash, bundle_json, workspace_id)
			VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''))
			ON DUPLICATE KEY UPDATE
				change_id = VALUES(change_id), branch = VALUES(branch), status = VALUES(status), tasks_sync_state = VALUES(tasks_sync_state),
				commit_sha = VALUES(commit_sha), tasks_md_digest = VALUES(tasks_md_digest), kg_available = VALUES(kg_available),
				kg_snapshot = VALUES(kg_snapshot), kg_read_ids = VALUES(kg_read_ids), kg_gaps = VALUES(kg_gaps), kg_conflicts = VALUES(kg_conflicts),
				skill_hash = VALUES(skill_hash), bundle_json = VALUES(bundle_json), workspace_id = VALUES(workspace_id),
				updated_at = CURRENT_TIMESTAMP(6), version = version + 1`,
			c.ID, tenantID, c.RequestID, c.ChangeID, c.Branch, string(c.Status), string(c.SyncState), c.Commit, c.TasksMDDigest, c.KGAvailable,
			c.KGSnapshot, read, gaps, conflicts, c.SkillHash, bundle, c.WorkspaceID); err != nil {
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
func (r *OpenSpecChangeRepository) UpdateSync(ctx context.Context, id string, state domain.SyncState, digest string, at *time.Time, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `
			UPDATE openspec_changes
			SET tasks_sync_state = ?, tasks_sync_digest = NULLIF(?, ''), tasks_sync_next_retry_at = ?, updated_at = CURRENT_TIMESTAMP(6), version = version + 1
			WHERE id = ? AND tenant_id = ? AND version = ?`, string(state), digest, at, id, tenantID, expectedVersion)
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

// ListPendingSync lists ready changes whose tasks.md still has to catch up with task progress and whose retry time has come.
func (r *OpenSpecChangeRepository) ListPendingSync(ctx context.Context, limit int) ([]domain.OpenSpecChange, error) {
	var out []domain.OpenSpecChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.QueryContext(ctx, `SELECT `+openSpecChangeColumns+` FROM openspec_changes
			WHERE tenant_id = ? AND status = 'ready' AND tasks_sync_state IN ('pending', 'failed')
				AND (tasks_sync_next_retry_at IS NULL OR tasks_sync_next_retry_at <= CURRENT_TIMESTAMP(6))
			ORDER BY updated_at, id LIMIT ?`, tenantID, limit)
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
