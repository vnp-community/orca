# TASK-WT-04-03: `RecordWorktreeCreated` accepts `baseRef`; `GetWorktree` RPC handler

**From Solution:** SOL-WT-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/record_worktree_created.go`
**Depends on:** TASK-WT-04-02
**Status:** `[ ]` TODO

---

## Context

Completes `project-service`'s side of the `base_ref` backfill: the usecase input gains `BaseRef`, and a new `GetWorktree` usecase + gRPC handler exposes the RPC [TASK-WT-04-01](./TASK-WT-04-01-schema-base-ref.md) added to the proto.

## Changes to make

`backend-go/services/project-service/internal/usecase/record_worktree_created.go` — add `BaseRef` to the input and thread it into `NewWorktree`:

```go
type RecordWorktreeCreatedInput struct {
	ProjectID string
	RepoID    string
	Path      string
	Branch    string
	BaseRef   string // NEW
	Lineage   domain.WorktreeLineageCapture
}
```

```go
	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch, in.BaseRef, in.Lineage)
```

(Combine with [TASK-WT-01-06](./TASK-WT-01-06-project-service-outbox-event.md)'s event-building change to this same function if that task already landed — both add to the same `Execute` body without conflicting.)

Create `backend-go/services/project-service/internal/usecase/get_worktree.go` (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type GetWorktree struct {
	repo WorktreeRepository
}

func NewGetWorktree(repo WorktreeRepository) *GetWorktree {
	return &GetWorktree{repo: repo}
}

func (uc *GetWorktree) Execute(ctx context.Context, worktreeID string) (domain.Worktree, error) {
	wt, err := uc.repo.GetWorktree(ctx, worktreeID)
	if err != nil {
		if err == domain.ErrWorktreeNotFound {
			return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
		}
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_GET_WORKTREE_FAILED", "failed to fetch worktree", err)
	}
	return wt, nil
}
```

Add the gRPC handler to `backend-go/services/project-service/internal/adapter/grpc/server.go` (near `ListWorktrees`, `server.go:360-370`), and update `toProtoWorktree` (`server.go:603-622`) to carry `BaseRef`:

```go
func toProtoWorktree(wt domain.Worktree) *projectv1.Worktree {
	return &projectv1.Worktree{
		Id:        wt.ID,
		ProjectId: wt.ProjectID,
		RepoId:    wt.RepoID,
		Path:      wt.Path,
		Branch:    wt.Branch,
		Active:    wt.Active,

		ParentWorktreeId:        wt.ParentWorktreeID,
		Origin:                  wt.Origin,
		CaptureSource:           wt.CaptureSource,
		CaptureConfidence:       wt.CaptureConfidence,
		TaskId:                  wt.TaskID,
		OrchestrationRunId:      wt.OrchestrationRunID,
		CoordinatorHandle:       wt.CoordinatorHandle,
		CreatedByTerminalHandle: wt.CreatedByTerminalHandle,
		CreatedAtUnixMs:         wt.CreatedAt.UnixMilli(),
		BaseRef:                 wt.BaseRef, // NEW
	}
}

func (s *Server) GetWorktree(ctx context.Context, req *projectv1.GetWorktreeRequest) (*projectv1.Worktree, error) {
	wt, err := s.getWorktree.Execute(ctx, req.GetWorktreeId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoWorktree(wt), nil
}
```

Also update `RecordWorktreeCreated`'s handler (`server.go:331-351`) to forward `req.GetBaseRef()` into `usecase.RecordWorktreeCreatedInput.BaseRef`, and wire `getWorktree *usecase.GetWorktree` through the `Server` struct + `New(...)` constructor (same pattern as every other usecase in this file) plus `cmd/server/main.go`'s composition root.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
```

Expected: clean build.
