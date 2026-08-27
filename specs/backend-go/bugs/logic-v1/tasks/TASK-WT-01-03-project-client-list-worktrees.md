# TASK-WT-01-03: Add `ProjectClient.ListWorktrees` for the BR-WT-04 count cap

**From Solution:** SOL-WT-01
**Priority:** P0 — the usecase task depends on this port
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go`
**Depends on:** none — `project-service.ListWorktrees(project_id)` already exists in `project.proto` (verified: `service ProjectService` line 40), so no proto change is needed
**Status:** `[x]` DONE — Added ProjectClient.ListWorktrees, WorktreeRecord.RepoID/Active; go build + go test ./services/git-gateway-service/... clean.

---

## Context

BR-WT-04 (max 20 worktrees per repo) needs a read git-gateway-service doesn't currently have a port for: the caller-project's existing worktree count. `project-service.ListWorktrees` is already real; this task only adds a new method to the already-existing `ProjectClient` port (`ports.go:341-345`) and implements it against the RPC that already exists — per [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md)'s "no proto change needed" finding.

## Changes to make

In `backend-go/services/git-gateway-service/internal/usecase/ports.go`, extend the `ProjectClient` interface:

```go
type ProjectClient interface {
	GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error)
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error)
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
	// ListWorktrees backs BR-WT-04's count cap (max 20 active worktrees per
	// repo) — project-service.ListWorktrees(project_id) already exists
	// (proto/orca/project/v1/project.proto); this is a new call on an
	// existing RPC, not a new proto surface.
	ListWorktrees(ctx context.Context, projectID string) ([]domain.WorktreeRecord, error)
}
```

`domain.WorktreeRecord` (`domain.go:248-252`) currently has no `RepoID`/`Active` fields — the count cap needs both to filter correctly. Extend it:

```go
// WorktreeRecord mirrors project-service's Worktree message — the
// bookkeeping row RecordWorktreeCreated/SetWorktreeActivation/ListWorktrees
// return. RepoID/Active added for BR-WT-04's per-repo active-count cap.
type WorktreeRecord struct {
	ID     string
	RepoID string
	Path   string
	Branch string
	Active bool
}
```

Implement in `backend-go/services/git-gateway-service/internal/adapter/grpcclient/project_client.go`:

```go
func (p *ProjectClient) ListWorktrees(ctx context.Context, projectID string) ([]domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{ProjectId: projectID})
	if err != nil {
		return nil, err
	}
	out := make([]domain.WorktreeRecord, 0, len(resp.GetWorktrees()))
	for _, wt := range resp.GetWorktrees() {
		out = append(out, domain.WorktreeRecord{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch(), Active: wt.GetActive()})
	}
	return out, nil
}
```

Update every existing call site that constructs a `domain.WorktreeRecord` from `RecordWorktreeCreated`'s response (`project_client.go`'s existing `RecordWorktreeCreated` method) to also set `RepoID`/`Active` — `RecordWorktreeCreatedResponse.Worktree` already carries both (`project.proto`'s `Worktree` message, fields 3/6):

```go
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch(), Active: wt.GetActive()}, nil
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build. Any fake `ProjectClient` implementations under `services/git-gateway-service/internal/usecase/*_test.go` (e.g. `worktree_fakes_test.go`) will fail to compile until they also implement `ListWorktrees` — fix those fakes as part of this task so the package builds, even though the new behavior itself is exercised by [TASK-WT-01-07](./TASK-WT-01-07-tests.md).
