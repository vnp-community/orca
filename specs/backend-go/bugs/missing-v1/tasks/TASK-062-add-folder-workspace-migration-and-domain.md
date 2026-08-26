# TASK-062: Add `folder_workspaces` migration, domain model, and repository port

**From Solution:** SOL-010 (Design — Data model, `usecase/ports.go` extension)
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/migrations/NNNN_create_folder_workspaces.sql` (new), `backend-go/services/project-service/internal/domain/folder_workspace.go` (new), `backend-go/services/project-service/internal/usecase/ports.go`
**Depends on:** TASK-061
**Status:** `[x]` DONE — migration `0006_folder_workspaces.{up,down}.sql` (`project.folder_workspaces` + RLS), domain model, and `FolderWorkspaceRepository` port added. Worktree `agent-abbc42cb9786d6743`, commit `a329ce7d9`. Pending merge.

---

## Context

Follows `project-service.md` §5's `repos` table shape, with `project_id`
dropped and `dev_server_id` added — a folder workspace needs to know
which host the path lives on since it has no owning `Project` to inherit
that from. RLS-enabled (`tenant_id`-scoped), matching every other table
in this service. The `UNIQUE (tenant_id, dev_server_id, path)` constraint
is the authoritative conflict guard `CreateFolderWorkspace` relies on;
`GetFolderWorkspacePathStatus` is a pre-flight UX convenience that queries
the same uniqueness, not the only enforcement point.

---

## Changes to make

### Step 1: Migration

**File:** `backend-go/services/project-service/migrations/NNNN_create_folder_workspaces.sql`
(replace `NNNN` with the next sequential migration number in this
service's `migrations/` directory)

```sql
CREATE TABLE folder_workspaces (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL,        -- logical FK -> infra-fleet-service.dev_servers
  path           TEXT NOT NULL,
  name           TEXT NOT NULL,
  added_by       UUID NOT NULL,        -- logical FK -> tenant-service.users
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, dev_server_id, path)
);
CREATE INDEX idx_folder_workspaces_tenant ON folder_workspaces (tenant_id);

ALTER TABLE folder_workspaces ENABLE ROW LEVEL SECURITY;
-- Follow this service's existing RLS policy convention for tenant_id-scoped
-- tables (see the policy this migration's neighbors use for `repos`/
-- `worktrees` — copy that exact policy shape, substituting
-- folder_workspaces as the table).
```

### Step 2: Domain model

**File:** `backend-go/services/project-service/internal/domain/folder_workspace.go`

```go
// Package domain — see project.go's package doc comment for this
// package's zero-external-imports convention.
package domain

import (
	"errors"
	"time"
)

var (
	ErrEmptyPath        = errors.New("domain: path is required")
	ErrPathNotAbsolute  = errors.New("domain: path must be absolute")
	ErrEmptyDevServerID = errors.New("domain: dev_server_id is required")
	// ErrPathAlreadyRegistered is what the postgres adapter maps a
	// UNIQUE(tenant_id, dev_server_id, path) violation to — the usecase
	// layer surfaces this as apperrors.KindAlreadyExists, not a generic
	// 500.
	ErrPathAlreadyRegistered = errors.New("domain: this path is already registered as a folder workspace")
	ErrFolderWorkspaceNotFound = errors.New("domain: folder workspace not found")
)

// PathStatus values for GetFolderWorkspacePathStatus — a DB-conflict
// check, not a live filesystem probe. See SOL-010's design note before
// changing that assumption.
const (
	PathStatusAvailable              = "PATH_STATUS_AVAILABLE"
	PathStatusAlreadyFolderWorkspace = "PATH_STATUS_ALREADY_FOLDER_WORKSPACE"
	PathStatusAlreadyRepo            = "PATH_STATUS_ALREADY_REPO"
	PathStatusInvalid                = "PATH_STATUS_INVALID"
)

// FolderWorkspace is a standalone, non-git filesystem path added directly
// to the workspace — see project.proto's FolderWorkspace message doc
// comment for how this differs from ProjectGroup/Repo.
type FolderWorkspace struct {
	ID          string
	TenantID    string
	DevServerID string
	Path        string
	Name        string
	AddedBy     string
	CreatedAt   time.Time
}

// PathStatus is GetFolderWorkspacePathStatus's result.
type PathStatus struct {
	Status     string
	ExistingID string // set when Status == PathStatusAlreadyFolderWorkspace
}

// NewFolderWorkspace enforces this entity's shape invariants.
func NewFolderWorkspace(id, tenantID, devServerID, path, name, addedBy string) (FolderWorkspace, error) {
	if devServerID == "" {
		return FolderWorkspace{}, ErrEmptyDevServerID
	}
	if path == "" {
		return FolderWorkspace{}, ErrEmptyPath
	}
	return FolderWorkspace{
		ID: id, TenantID: tenantID, DevServerID: devServerID, Path: path, Name: name, AddedBy: addedBy,
	}, nil
}
```

### Step 3: Extend `internal/usecase/ports.go`

Add alongside the existing `ProjectGroupRepository` port:

```go
// FolderWorkspaceRepository is the persistence port for FolderWorkspace —
// see domain.FolderWorkspace's doc comment for why this is a standalone
// entity, not a ProjectGroup extension.
type FolderWorkspaceRepository interface {
	Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error)
	Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error)
	Delete(ctx context.Context, id string) error
	ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error)
	FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error)
	// RepoPathExists cross-checks against the repos table so
	// GetFolderWorkspacePathStatus can distinguish
	// PathStatusAlreadyRepo from PathStatusAvailable.
	RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error)
	// Get is used by Update/Delete's ownership check (TASK-064) to load
	// the caller's added_by before mutating.
	Get(ctx context.Context, id string) (*domain.FolderWorkspace, error)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./internal/domain/... ./internal/usecase/...
go vet ./internal/domain/... ./internal/usecase/...

# Apply the migration against a local/test Postgres per this service's
# existing migration-runner convention, e.g.:
# go run ./cmd/migrate up   (or whatever this service's migration tool is)
```
