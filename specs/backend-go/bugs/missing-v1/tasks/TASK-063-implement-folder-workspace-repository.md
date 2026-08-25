# TASK-063: Implement `FolderWorkspaceRepository` (postgres adapter)

**From Solution:** SOL-010 (Design — Data model; `FolderWorkspaceRepository` port from TASK-062)
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go` (new)
**Depends on:** TASK-062
**Status:** `[ ]` TODO

---

## Context

Implements `usecase.FolderWorkspaceRepository` against Postgres, following
this service's existing `adapter/postgres/repository.go` conventions
(tenant-scoped queries, `UNIQUE` constraint violations mapped to a typed
domain error rather than passed through raw).

---

## Changes to make

**File:** `backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go`

```go
// Package postgres — folder_workspaces table adapter. Mirrors this
// package's existing repository.go conventions (tenant-scoped queries,
// pgconn error-code mapping for unique-constraint violations).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// pgUniqueViolationCode is Postgres's SQLSTATE for a unique_violation —
// used below to map folder_workspaces' UNIQUE(tenant_id, dev_server_id,
// path) constraint to domain.ErrPathAlreadyRegistered rather than a raw
// pgconn error. Follow this package's existing constant if one is already
// defined elsewhere in this file's sibling repository.go; do not
// redeclare if so.
const pgUniqueViolationCode = "23505"

type FolderWorkspaceRepository struct {
	pool PgxPool // reuse this package's existing pool interface/type alias
}

func NewFolderWorkspaceRepository(pool PgxPool) *FolderWorkspaceRepository {
	return &FolderWorkspaceRepository{pool: pool}
}

func (r *FolderWorkspaceRepository) Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO folder_workspaces (tenant_id, dev_server_id, path, name, added_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, fw.TenantID, fw.DevServerID, fw.Path, fw.Name, fw.AddedBy)

	if err := row.Scan(&fw.ID, &fw.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			return domain.FolderWorkspace{}, domain.ErrPathAlreadyRegistered
		}
		return domain.FolderWorkspace{}, fmt.Errorf("postgres: insert folder workspace: %w", err)
	}
	return fw, nil
}

func (r *FolderWorkspaceRepository) Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE folder_workspaces SET name = $2
		WHERE id = $1
		RETURNING id, tenant_id, dev_server_id, path, name, added_by, created_at
	`, id, name)

	var fw domain.FolderWorkspace
	if err := row.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
		}
		return domain.FolderWorkspace{}, fmt.Errorf("postgres: update folder workspace: %w", err)
	}
	return fw, nil
}

func (r *FolderWorkspaceRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM folder_workspaces WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgres: delete folder workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrFolderWorkspaceNotFound
	}
	return nil
}

func (r *FolderWorkspaceRepository) ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, dev_server_id, path, name, added_by, created_at
		FROM folder_workspaces WHERE tenant_id = $1 ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list folder workspaces: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderWorkspace
	for rows.Next() {
		var fw domain.FolderWorkspace
		if err := rows.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scan folder workspace: %w", err)
		}
		out = append(out, fw)
	}
	return out, rows.Err()
}

func (r *FolderWorkspaceRepository) FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, dev_server_id, path, name, added_by, created_at
		FROM folder_workspaces WHERE tenant_id = $1 AND dev_server_id = $2 AND path = $3
	`, tenantID, devServerID, path)

	var fw domain.FolderWorkspace
	if err := row.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: find folder workspace by path: %w", err)
	}
	return &fw, nil
}

func (r *FolderWorkspaceRepository) RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error) {
	// Cross-checks against this service's existing repos table — join
	// through projects to scope by dev_server_id, since repos itself has
	// no dev_server_id column of its own (it inherits one from its
	// owning project per project-service.md §4/§5).
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM repos r
			JOIN projects p ON p.id = r.project_id
			WHERE p.tenant_id = $1 AND p.dev_server_id = $2 AND r.path = $3
		)
	`, tenantID, devServerID, path).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: check repo path exists: %w", err)
	}
	return exists, nil
}

func (r *FolderWorkspaceRepository) Get(ctx context.Context, id string) (*domain.FolderWorkspace, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, dev_server_id, path, name, added_by, created_at
		FROM folder_workspaces WHERE id = $1
	`, id)

	var fw domain.FolderWorkspace
	if err := row.Scan(&fw.ID, &fw.TenantID, &fw.DevServerID, &fw.Path, &fw.Name, &fw.AddedBy, &fw.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres: get folder workspace: %w", err)
	}
	return &fw, nil
}
```

Adjust `PgxPool` to whatever this package's existing pool
interface/type-alias is actually named (check `repository.go`'s receiver
type and reuse it verbatim — do not introduce a second, differently-named
pool abstraction). Verify the actual column names on `repos`/`projects`
against this service's real migrations before finalizing
`RepoPathExists`'s join — the sketch above assumes `repos.project_id`,
`projects.dev_server_id`, `repos.path` per `project-service.md` §5's
documented shape.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./internal/adapter/postgres/...
go vet ./internal/adapter/postgres/...
```
