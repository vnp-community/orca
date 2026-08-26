# TASK-064: Implement `FolderWorkspace` usecases (create/update/delete/list/getPathStatus)

**From Solution:** SOL-010 (Design — `usecase/` layer)
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/create_folder_workspace.go`, `update_folder_workspace.go`, `delete_folder_workspace.go`, `list_folder_workspaces.go`, `get_folder_workspace_path_status.go` (all new)
**Depends on:** TASK-062, TASK-063
**Status:** `[x]` DONE — `FolderWorkspaceUseCase` (create/update/delete/list/getPathStatus, owner-only mutation guard) implemented. Worktree `agent-abbc42cb9786d6743`, commit `a329ce7d9`. Pending merge.

---

## Context

Standard CRUD shape, no relay/dispatch branching — per BUG-010's
dispatch-model finding, this namespace is Postgres-only. Authorization:
per `project-service.md` §9's posture ("`CreateProject` requires only
authentication"), `folder_workspaces` rows have no membership model of
their own — any authenticated tenant member can create/list. `Update`/
`Delete` additionally check `added_by == caller` OR global admin.

---

## Changes to make

**File:** `internal/usecase/create_folder_workspace.go`

```go
package usecase

import (
	"cmp"
	"context"
	"path/filepath"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type FolderWorkspaceUseCase struct {
	repo FolderWorkspaceRepository
}

func NewFolderWorkspaceUseCase(repo FolderWorkspaceRepository) *FolderWorkspaceUseCase {
	return &FolderWorkspaceUseCase{repo: repo}
}

type CreateFolderWorkspaceInput struct {
	DevServerID string
	Path        string
	Name        string
}

// Create validates Path is absolute, then delegates to the repository.
// The repository's UNIQUE(tenant_id, dev_server_id, path) constraint is
// the real conflict guard; GetPathStatus (below) is a pre-flight
// convenience the frontend calls separately — this usecase still
// surfaces the same ErrPathAlreadyRegistered on a constraint violation,
// not a generic error.
func (uc *FolderWorkspaceUseCase) Create(ctx context.Context, id Identity, in CreateFolderWorkspaceInput) (domain.FolderWorkspace, error) {
	if !filepath.IsAbs(in.Path) {
		return domain.FolderWorkspace{}, domain.ErrPathNotAbsolute
	}
	fw, err := domain.NewFolderWorkspace("", id.TenantID, in.DevServerID, filepath.Clean(in.Path), cmp.Or(in.Name, filepath.Base(in.Path)), id.UserID)
	if err != nil {
		return domain.FolderWorkspace{}, err
	}
	return uc.repo.Create(ctx, fw)
}
```

**File:** `internal/usecase/update_folder_workspace.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// Update renames a folder workspace — the only mutable field, per
// project.proto's UpdateFolderWorkspaceRequest doc comment. Rejects
// callers who are neither the original added_by nor a global admin.
func (uc *FolderWorkspaceUseCase) Update(ctx context.Context, id Identity, folderWorkspaceID, name string) (domain.FolderWorkspace, error) {
	existing, err := uc.repo.Get(ctx, folderWorkspaceID)
	if err != nil {
		return domain.FolderWorkspace{}, err
	}
	if existing == nil {
		return domain.FolderWorkspace{}, domain.ErrFolderWorkspaceNotFound
	}
	if existing.AddedBy != id.UserID && !id.IsGlobalAdmin {
		return domain.FolderWorkspace{}, ErrForbidden // reuse this package's existing sentinel if one already exists; else define here
	}
	return uc.repo.Update(ctx, folderWorkspaceID, name)
}
```

**File:** `internal/usecase/delete_folder_workspace.go`

```go
package usecase

import "context"

func (uc *FolderWorkspaceUseCase) Delete(ctx context.Context, id Identity, folderWorkspaceID string) error {
	existing, err := uc.repo.Get(ctx, folderWorkspaceID)
	if err != nil {
		return err
	}
	if existing == nil {
		return domain.ErrFolderWorkspaceNotFound
	}
	if existing.AddedBy != id.UserID && !id.IsGlobalAdmin {
		return ErrForbidden
	}
	return uc.repo.Delete(ctx, folderWorkspaceID)
}
```

**File:** `internal/usecase/list_folder_workspaces.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

func (uc *FolderWorkspaceUseCase) List(ctx context.Context, id Identity) ([]domain.FolderWorkspace, error) {
	return uc.repo.ListByTenant(ctx, id.TenantID)
}
```

**File:** `internal/usecase/get_folder_workspace_path_status.go`

```go
package usecase

import (
	"context"
	"path/filepath"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// GetPathStatus answers purely from this service's own tables — NOT a
// live filesystem probe. See domain.PathStatus* constants' doc comment
// and SOL-010's design note before changing that assumption.
func (uc *FolderWorkspaceUseCase) GetPathStatus(ctx context.Context, id Identity, devServerID, path string) (domain.PathStatus, error) {
	if !filepath.IsAbs(path) {
		return domain.PathStatus{Status: domain.PathStatusInvalid}, nil
	}
	clean := filepath.Clean(path)
	if existing, err := uc.repo.FindByPath(ctx, id.TenantID, devServerID, clean); err != nil {
		return domain.PathStatus{}, err
	} else if existing != nil {
		return domain.PathStatus{Status: domain.PathStatusAlreadyFolderWorkspace, ExistingID: existing.ID}, nil
	}
	if isRepo, err := uc.repo.RepoPathExists(ctx, id.TenantID, devServerID, clean); err != nil {
		return domain.PathStatus{}, err
	} else if isRepo {
		return domain.PathStatus{Status: domain.PathStatusAlreadyRepo}, nil
	}
	return domain.PathStatus{Status: domain.PathStatusAvailable}, nil
}
```

### Note on `Identity`/`ErrForbidden`/`IsGlobalAdmin`

Reuse this service's existing `Identity` type and forbidden-access
sentinel exactly as `CreateProject`/other usecases already use them (per
`project-service.md` §9) — do not introduce a second identity shape. If
`Identity` doesn't already carry an `IsGlobalAdmin` (or equivalently-named)
field, check how `project-service`'s existing admin-gated usecases (if
any) determine that, and match their convention rather than inventing a
new one here.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./internal/usecase/...
go vet ./internal/usecase/...
```
