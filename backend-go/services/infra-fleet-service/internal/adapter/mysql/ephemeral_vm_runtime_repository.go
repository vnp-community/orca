package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// EphemeralVmRuntimeStore implements usecase.EphemeralVmRuntimeRepository
// against ephemeral_vm_runtimes (migrations/mysql/0013).
type EphemeralVmRuntimeStore struct {
	db *sql.DB
}

func NewEphemeralVmRuntimeStore(db *sql.DB) *EphemeralVmRuntimeStore {
	return &EphemeralVmRuntimeStore{db: db}
}

const ephemeralVmRuntimeColumns = `id, repo_id, recipe_id, connection_type, status, COALESCE(environment_id, ''),
	       COALESCE(workspace_id, ''), COALESCE(last_error, ''), created_at, updated_at`

func scanEphemeralVmRuntime(row rowScanner) (domain.EphemeralVmRuntime, error) {
	var r domain.EphemeralVmRuntime
	err := row.Scan(&r.ID, &r.RepoID, &r.RecipeID, &r.ConnectionType,
		&r.Status, &r.EnvironmentID, &r.WorkspaceID, &r.LastError, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

// List returns every non-destroyed runtime for tenantID — backs
// ListEphemeralVmRuntimes (TASK-002); Create/UpdateStatus (TASK-004) write
// to the same table.
func (s *EphemeralVmRuntimeStore) List(ctx context.Context, tenantID string) ([]domain.EphemeralVmRuntime, error) {
	q := `SELECT ` + ephemeralVmRuntimeColumns + `
		FROM ephemeral_vm_runtimes
		WHERE tenant_id = ? AND status <> 'destroyed'
		ORDER BY created_at`
	rows, err := s.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql: listing ephemeral vm runtimes: %w", err)
	}
	defer rows.Close()

	var out []domain.EphemeralVmRuntime
	for rows.Next() {
		r, err := scanEphemeralVmRuntime(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scanning ephemeral vm runtime row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get backs EphemeralVmRelay.CleanupWorkspace (TASK-BE-EVM-001), which only
// receives a runtimeID (not a workspaceID) and needs the runtime's
// RecipeID before relaying vm.exec.
func (s *EphemeralVmRuntimeStore) Get(ctx context.Context, tenantID, id string) (domain.EphemeralVmRuntime, error) {
	q := `SELECT ` + ephemeralVmRuntimeColumns + `
		FROM ephemeral_vm_runtimes
		WHERE tenant_id = ? AND id = ? AND status <> 'destroyed'`
	r, err := scanEphemeralVmRuntime(s.db.QueryRowContext(ctx, q, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EphemeralVmRuntime{}, domain.ErrEphemeralVmRuntimeNotFound
	}
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: get ephemeral vm runtime: %w", err)
	}
	return r, nil
}

// GetByWorkspaceID backs EphemeralVmRelay.SuspendWorkspace/ResumeWorkspace
// (TASK-004), which look up the runtime attached to a workspace/worktree id.
func (s *EphemeralVmRuntimeStore) GetByWorkspaceID(ctx context.Context, tenantID, workspaceID string) (domain.EphemeralVmRuntime, error) {
	q := `SELECT ` + ephemeralVmRuntimeColumns + `
		FROM ephemeral_vm_runtimes
		WHERE tenant_id = ? AND workspace_id = ? AND status <> 'destroyed'`
	r, err := scanEphemeralVmRuntime(s.db.QueryRowContext(ctx, q, tenantID, workspaceID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EphemeralVmRuntime{}, domain.ErrEphemeralVmRuntimeNotFound
	}
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: get ephemeral vm runtime by workspace: %w", err)
	}
	return r, nil
}

// selectByTenantAndID re-reads one runtime row after a write — the shared
// "UPDATE then SELECT" half of every RETURNING translation below (per
// BE-DB-SOL-005 §3's pattern): a query finding nothing means either a
// genuine not-found id/tenant OR the row was concurrently deleted between
// the UPDATE and this SELECT, both mapped to
// domain.ErrEphemeralVmRuntimeNotFound, matching what the Postgres
// RETURNING clause finding no row also meant.
func (s *EphemeralVmRuntimeStore) selectByTenantAndID(ctx context.Context, tenantID, id string) (domain.EphemeralVmRuntime, error) {
	q := `SELECT ` + ephemeralVmRuntimeColumns + ` FROM ephemeral_vm_runtimes WHERE tenant_id = ? AND id = ?`
	r, err := scanEphemeralVmRuntime(s.db.QueryRowContext(ctx, q, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EphemeralVmRuntime{}, domain.ErrEphemeralVmRuntimeNotFound
	}
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: reading back ephemeral vm runtime: %w", err)
	}
	return r, nil
}

// UpdateStatus applies a partial update — status is always set; the other
// fields are only touched when non-empty (workspaceID)/explicitly passed
// (lastError, which callers pass "" to clear on a successful transition).
// No RETURNING on MySQL — UPDATE (ignoring RowsAffected: a 0-row UPDATE on
// a genuinely missing id and a successful UPDATE are told apart by the
// follow-up SELECT instead, not by counting), then read back.
func (s *EphemeralVmRuntimeStore) UpdateStatus(ctx context.Context, tenantID, id, status, workspaceID, lastError string) (domain.EphemeralVmRuntime, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ephemeral_vm_runtimes
		SET status = ?,
		    workspace_id = COALESCE(NULLIF(?, ''), workspace_id),
		    last_error = NULLIF(?, ''),
		    updated_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND id = ?`, status, workspaceID, lastError, tenantID, id)
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: update ephemeral vm runtime status: %w", err)
	}
	return s.selectByTenantAndID(ctx, tenantID, id)
}

// UpdateProvisionResult applies a partial update like UpdateStatus, plus
// connection_type — backs EphemeralVmRelay.Provision (TASK-BE-EVM-004),
// which must persist which branch ("orca-server" | "ssh") vm.provision's
// terminal result resolved to.
func (s *EphemeralVmRuntimeStore) UpdateProvisionResult(ctx context.Context, tenantID, id, status, connectionType, lastError string) (domain.EphemeralVmRuntime, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ephemeral_vm_runtimes
		SET status = ?,
		    connection_type = COALESCE(NULLIF(?, ''), connection_type),
		    last_error = NULLIF(?, ''),
		    updated_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND id = ?`, status, connectionType, lastError, tenantID, id)
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: update ephemeral vm runtime provision result: %w", err)
	}
	return s.selectByTenantAndID(ctx, tenantID, id)
}

// SetEnvironmentID writes the real dev_servers.id PK onto runtimeID's
// environment_id column — see the interface method's doc comment
// (list_ephemeral_vm_runtimes.go) for why the caller lives outside this
// package. Not-found (the overwhelmingly common case, since most agents
// minting a token are not ephemeral VMs) is surfaced as
// domain.ErrEphemeralVmRuntimeNotFound so callers can treat it as a silent
// no-op, matching Get/GetByWorkspaceID/UpdateStatus's own not-found
// convention in this file.
func (s *EphemeralVmRuntimeStore) SetEnvironmentID(ctx context.Context, tenantID, runtimeID, environmentID string) (domain.EphemeralVmRuntime, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ephemeral_vm_runtimes
		SET environment_id = ?, updated_at = CURRENT_TIMESTAMP(6)
		WHERE tenant_id = ? AND id = ?`, environmentID, tenantID, runtimeID)
	if err != nil {
		return domain.EphemeralVmRuntime{}, fmt.Errorf("mysql: set ephemeral vm runtime environment id: %w", err)
	}
	return s.selectByTenantAndID(ctx, tenantID, runtimeID)
}

// FindDevServerByEnvironmentID backs SpawnTerminalSession's environmentId
// resolution fallback (TASK-BE-EVM-007) — see the interface method's doc
// comment for why this returns environmentID itself as devServerID rather
// than a distinct column value.
func (s *EphemeralVmRuntimeStore) FindDevServerByEnvironmentID(ctx context.Context, tenantID, environmentID string) (string, bool, error) {
	const q = `
		SELECT 1 FROM ephemeral_vm_runtimes
		WHERE tenant_id = ? AND environment_id = ? AND status <> 'destroyed'
		LIMIT 1`
	var exists int
	err := s.db.QueryRowContext(ctx, q, tenantID, environmentID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("mysql: find dev server by environment id: %w", err)
	}
	return environmentID, true, nil
}
