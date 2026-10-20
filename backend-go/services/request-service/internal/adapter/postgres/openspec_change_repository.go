package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

const openSpecChangeColumns = `id::text, request_id::text, change_id, COALESCE(branch, ''), status, tasks_sync_state, COALESCE(commit_sha, ''),
	COALESCE(tasks_md_digest, ''), kg_available, COALESCE(kg_snapshot, ''), kg_read_ids, kg_gaps, kg_conflicts, COALESCE(skill_hash, ''),
	bundle_json, COALESCE(workspace_id, ''), redacted_secrets,
	COALESCE(tasks_sync_digest, ''), COALESCE(tasks_sync_error, ''), tasks_sync_attempts, tasks_sync_next_retry_at, version`

func scanOpenSpecChange(row pgx.Row) (domain.OpenSpecChange, error) {
	return scanOpenSpecChangeInto(row)
}

// scanOpenSpecChangeInto reads the change columns and then any extra destinations the query selected after them.
func scanOpenSpecChangeInto(row pgx.Row, extra ...any) (domain.OpenSpecChange, error) {
	var c domain.OpenSpecChange
	var status, syncState string
	var read, gaps, conflicts, bundle []byte
	dest := append([]any{&c.ID, &c.RequestID, &c.ChangeID, &c.Branch, &status, &syncState, &c.Commit, &c.TasksMDDigest, &c.KGAvailable,
		&c.KGSnapshot, &read, &gaps, &conflicts, &c.SkillHash, &bundle, &c.WorkspaceID, &c.RedactedSecrets,
		&c.SyncDigest, &c.SyncError, &c.SyncAttempts, &c.SyncNextRetryAt, &c.Version}, extra...)
	if err := row.Scan(dest...); err != nil {
		return domain.OpenSpecChange{}, err
	}
	c.Status, c.SyncState = domain.ChangeStatus(status), domain.SyncState(syncState)
	for _, f := range []struct {
		raw []byte
		to  *[]string
	}{{read, &c.KGReadIDs}, {gaps, &c.KGGaps}, {conflicts, &c.KGConflicts}} {
		if len(f.raw) > 0 {
			if err := json.Unmarshal(f.raw, f.to); err != nil {
				return domain.OpenSpecChange{}, fmt.Errorf("postgres: openspec change list column: %w", err)
			}
		}
	}
	if len(bundle) > 0 {
		if err := json.Unmarshal(bundle, &c.Bundle); err != nil {
			return domain.OpenSpecChange{}, fmt.Errorf("postgres: openspec change bundle: %w", err)
		}
	}
	return c, nil
}

func (r *OpenSpecChangeRepository) GetByRequest(ctx context.Context, requestID string) (domain.OpenSpecChange, bool, error) {
	var c domain.OpenSpecChange
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if notUUID(requestID) {
			return nil
		}
		var err error
		c, err = scanOpenSpecChange(db.QueryRow(ctx, `SELECT `+openSpecChangeColumns+` FROM request.openspec_changes WHERE tenant_id = $1 AND request_id = $2`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgres: get openspec change: %w", err)
		}
		found = true
		return nil
	})
	return c, found, err
}

// Upsert writes the whole change for its Request: a first call inserts, later calls replace every field and bump the version.
// The tenant is the scoped one, so a change can never be written into another tenant's row.
func (r *OpenSpecChangeRepository) Upsert(ctx context.Context, c domain.OpenSpecChange) (domain.OpenSpecChange, error) {
	var out domain.OpenSpecChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		read, gaps, conflicts, bundle, err := encodeChangeJSON(c)
		if err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO request.openspec_changes (id, tenant_id, request_id, change_id, branch, status, tasks_sync_state, commit_sha,
				tasks_md_digest, kg_available, kg_snapshot, kg_read_ids, kg_gaps, kg_conflicts, skill_hash, bundle_json, workspace_id,
				tasks_sync_digest, tasks_sync_error, tasks_sync_attempts, tasks_sync_next_retry_at, redacted_secrets)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10, NULLIF($11, ''), $12, $13, $14, NULLIF($15, ''), $16, NULLIF($17, ''),
				NULLIF($18, ''), NULLIF($19, ''), $20, $21, $22)
			ON CONFLICT (tenant_id, request_id) DO UPDATE SET
				change_id = EXCLUDED.change_id, branch = EXCLUDED.branch, status = EXCLUDED.status, tasks_sync_state = EXCLUDED.tasks_sync_state,
				commit_sha = EXCLUDED.commit_sha, tasks_md_digest = EXCLUDED.tasks_md_digest, kg_available = EXCLUDED.kg_available,
				kg_snapshot = EXCLUDED.kg_snapshot, kg_read_ids = EXCLUDED.kg_read_ids, kg_gaps = EXCLUDED.kg_gaps, kg_conflicts = EXCLUDED.kg_conflicts,
				skill_hash = EXCLUDED.skill_hash, bundle_json = EXCLUDED.bundle_json, workspace_id = EXCLUDED.workspace_id,
				tasks_sync_digest = EXCLUDED.tasks_sync_digest, tasks_sync_error = EXCLUDED.tasks_sync_error,
				tasks_sync_attempts = EXCLUDED.tasks_sync_attempts, tasks_sync_next_retry_at = EXCLUDED.tasks_sync_next_retry_at,
				redacted_secrets = EXCLUDED.redacted_secrets,
				updated_at = now(), version = request.openspec_changes.version + 1`,
			c.ID, tenantID, c.RequestID, c.ChangeID, c.Branch, string(c.Status), string(c.SyncState), c.Commit, c.TasksMDDigest, c.KGAvailable,
			c.KGSnapshot, read, gaps, conflicts, c.SkillHash, bundle, c.WorkspaceID,
			c.SyncDigest, c.SyncError, c.SyncAttempts, c.SyncNextRetryAt, c.RedactedSecrets); err != nil {
			return fmt.Errorf("postgres: upsert openspec change: %w", err)
		}
		out, err = scanOpenSpecChange(db.QueryRow(ctx, `SELECT `+openSpecChangeColumns+` FROM request.openspec_changes WHERE tenant_id = $1 AND request_id = $2`, tenantID, c.RequestID))
		if err != nil {
			return fmt.Errorf("postgres: read back openspec change: %w", err)
		}
		return nil
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
		if notUUID(id) {
			return domain.ErrOpenSpecChangeNotFound
		}
		tag, err := db.Exec(ctx, `
			UPDATE request.openspec_changes
			SET tasks_sync_state = $4, tasks_sync_digest = NULLIF($5, ''), tasks_sync_error = NULLIF($6, ''), tasks_sync_attempts = $7,
				tasks_sync_next_retry_at = $8, updated_at = now(), version = version + 1
			WHERE id = $1 AND tenant_id = $2 AND version = $3`, id, tenantID, expectedVersion, string(u.State), u.Digest, u.Error, u.Attempts, u.NextRetryAt)
		if err != nil {
			return fmt.Errorf("postgres: update openspec change sync: %w", err)
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM request.openspec_changes WHERE id = $1 AND tenant_id = $2)`, id, tenantID).Scan(&exists); err != nil {
				return fmt.Errorf("postgres: classify openspec change update: %w", err)
			}
			if !exists {
				return domain.ErrOpenSpecChangeNotFound
			}
			return domain.ErrOpenSpecChangeVersionConflict
		}
		return nil
	})
}

// ListSyncWork lists ready changes that have something to do and whose retry time has come: tasks.md still has to catch up
// with task progress, or the Request is completed and the change is waiting to be archived. A failed change is left alone.
func (r *OpenSpecChangeRepository) ListSyncWork(ctx context.Context, limit int) ([]domain.OpenSpecChange, error) {
	var out []domain.OpenSpecChange
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT `+openSpecChangeColumns+` FROM request.openspec_changes
			WHERE tenant_id = $1 AND status = 'ready' AND tasks_sync_state <> 'failed'
				AND (tasks_sync_next_retry_at IS NULL OR tasks_sync_next_retry_at <= now())
				AND (tasks_sync_state = 'pending' OR request_id IN (SELECT id FROM request.requests WHERE tenant_id = $1 AND status = 'completed'))
			ORDER BY updated_at, id LIMIT $2`, tenantID, limit)
		if err != nil {
			return fmt.Errorf("postgres: list pending openspec changes: %w", err)
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
		rows, err := db.Query(ctx, `SELECT `+openSpecChangeColumns+`, updated_at FROM request.openspec_changes
			WHERE tenant_id = $1 AND COALESCE(workspace_id, '') <> '' ORDER BY updated_at, id LIMIT $2`, tenantID, limit)
		if err != nil {
			return fmt.Errorf("postgres: list openspec changes with a worktree: %w", err)
		}
		defer rows.Close()
		ids := []string{}
		for rows.Next() {
			var cand usecase.ProposalCleanupCandidate
			cand.Change, cand.UpdatedAt, err = scanOpenSpecChangeWithTime(rows)
			if err != nil {
				return err
			}
			out = append(out, cand)
			ids = append(ids, cand.Change.RequestID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		statuses := map[string]domain.RequestStatus{}
		srows, err := db.Query(ctx, `SELECT id::text, status FROM request.requests WHERE tenant_id = $1 AND id = ANY($2::uuid[])`, tenantID, ids)
		if err != nil {
			return fmt.Errorf("postgres: read request statuses: %w", err)
		}
		defer srows.Close()
		for srows.Next() {
			var id, st string
			if err := srows.Scan(&id, &st); err != nil {
				return err
			}
			statuses[id] = domain.RequestStatus(st)
		}
		for i := range out {
			out[i].RequestStatus = statuses[out[i].Change.RequestID]
		}
		return srows.Err()
	})
	return out, err
}

// scanOpenSpecChangeWithTime scans the change columns and the updated_at that follows them.
func scanOpenSpecChangeWithTime(row pgx.Rows) (domain.OpenSpecChange, time.Time, error) {
	var updated time.Time
	c, err := scanOpenSpecChangeInto(row, &updated)
	return c, updated, err
}
