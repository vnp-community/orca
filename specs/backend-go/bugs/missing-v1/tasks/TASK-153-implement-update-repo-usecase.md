# TASK-153: Implement `UpdateRepo` usecase and wire it into `project-service`'s gRPC server (Bucket 2)

**From Solution:** SOL-023 (Bucket 2)
**Priority:** P1
**Service:** `project-service`
**File:** `services/project-service/internal/usecase/update_repo.go` (new), `services/project-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-152 (needs generated `UpdateRepoRequest`/`UpdateRepoResponse` stubs)
**Status:** `[x]` DONE — implemented in worktree `agent-a5714e047dcaed0fc` (branch `worktree-agent-a5714e047dcaed0fc`), **committed** as `56c5fbeff`. Build/vet/test clean. Pending merge.

---

## Context

Field-masked update, mirroring `UpdateProject`'s own convention: an empty
string on the request means "leave this field unchanged."

## Changes to make

### New file `internal/usecase/update_repo.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// UpdateRepoInput mirrors the gRPC request 1:1 — empty URL/DisplayName
// means "no change," same field-mask convention UpdateProject already
// uses (project.proto:22).
type UpdateRepoInput struct {
	RepoID      string
	URL         string
	DisplayName string
}

type UpdateRepo struct {
	repos RepoRepository
}

func NewUpdateRepo(repos RepoRepository) *UpdateRepo {
	return &UpdateRepo{repos: repos}
}

func (uc *UpdateRepo) Execute(ctx context.Context, in UpdateRepoInput) (domain.Repo, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	repo, err := uc.repos.Get(ctx, tenantID, in.RepoID)
	if err != nil {
		return domain.Repo{}, err
	}
	if in.URL != "" {
		repo.URL = in.URL
	}
	if in.DisplayName != "" {
		repo.DisplayName = in.DisplayName
	}
	return uc.repos.Update(ctx, repo)
}
```

**Before writing this file**, check `internal/usecase/ports.go` for the
exact name/shape of the existing repo persistence port (`RepoRepository`
above is the expected name based on `AddRepo`/`RemoveRepo`'s existing
usecases — confirm against the real port name and adjust the field/method
names in this file, plus add an `Update(ctx, domain.Repo) (domain.Repo,
error)` method to that port interface and its Postgres implementation in
`internal/adapter/postgres/` if one doesn't already exist, following the
exact SQL/scan pattern the port's existing `Get`/`Create` methods use).

### `internal/adapter/grpc/server.go`

1. Add `updateRepo *usecase.UpdateRepo` to the `Server` struct and to
   `New(...)`'s parameter list and body, following the exact pattern the
   existing `addRepo`/`removeRepo` fields already use.
2. Add the RPC method, next to the existing `AddRepo`/`RemoveRepo` gRPC
   methods:

```go
func (s *Server) UpdateRepo(ctx context.Context, req *projectv1.UpdateRepoRequest) (*projectv1.UpdateRepoResponse, error) {
	repo, err := s.updateRepo.Execute(ctx, usecase.UpdateRepoInput{
		RepoID:      req.GetRepoId(),
		URL:         req.GetUrl(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateRepoResponse{Repo: toRepoProto(repo)}, nil
}
```

Reuse whatever helper the existing `AddRepo` method already uses to
convert `domain.Repo` → `*projectv1.Repo` (`toRepoProto` above is a
placeholder name — match the real one).

3. In `cmd/server/main.go`, construct `usecase.NewUpdateRepo(repoRepository)`
   next to the other repo usecase constructors and pass it into
   `grpc.New(...)`'s call, matching the existing constructor-wiring order.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./... && go vet ./...
```
