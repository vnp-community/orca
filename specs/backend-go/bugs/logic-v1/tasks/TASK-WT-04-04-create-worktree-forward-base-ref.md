# TASK-WT-04-04: `git-gateway-service.CreateWorktree` forwards `in.BaseRef` to `RecordWorktreeCreated`

**From Solution:** SOL-WT-04
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`
**Depends on:** TASK-WT-04-03
**Status:** `[x]` DONE — ProjectClient.RecordWorktreeCreated gained baseRef param (grpcclient + ports.go); create_worktree.go forwards in.BaseRef. go build+test clean; TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated added.

---

## Context

Confirmed regression: `CreateWorktree.Execute` already receives `in.BaseRef` (used to run `git worktree add`) but never forwards it to `RecordWorktreeCreated` (`create_worktree.go:60`, only `path, branch, lineage` are passed today). This is the last piece of the `base_ref` backfill — without it, every newly created worktree still has a `NULL` `base_ref` despite [TASK-WT-04-01](./TASK-WT-04-01-schema-base-ref.md)–[TASK-WT-04-03](./TASK-WT-04-03-usecase-grpc-get-worktree.md)'s schema/RPC work.

## Changes to make

`backend-go/services/git-gateway-service/internal/usecase/ports.go` — add `baseRef` to `ProjectClient.RecordWorktreeCreated`:

```go
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error)
```

`backend-go/services/git-gateway-service/internal/adapter/grpcclient/project_client.go` — thread it onto the wire (`RecordWorktreeCreatedRequest.base_ref`, added by [TASK-WT-04-01](./TASK-WT-04-01-schema-base-ref.md)):

```go
func (p *ProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	req := &projectv1.RecordWorktreeCreatedRequest{
		ProjectId: projectID, RepoId: repoID, Path: path, Branch: branch,
		BaseRef: nonEmptyPtr(baseRef),
	}
	req.ParentWorktreeId = nonEmptyPtr(lineage.ParentWorktreeID)
	req.Origin = nonEmptyPtr(lineage.Origin)
	req.CaptureSource = nonEmptyPtr(lineage.CaptureSource)
	req.TaskId = nonEmptyPtr(lineage.TaskID)
	req.OrchestrationRunId = nonEmptyPtr(lineage.OrchestrationRunID)
	req.CoordinatorHandle = nonEmptyPtr(lineage.CoordinatorHandle)
	req.CreatedByTerminalHandle = nonEmptyPtr(lineage.CreatedByTerminalHandle)

	resp, err := p.client.RecordWorktreeCreated(ctx, req)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), RepoID: wt.GetRepoId(), Path: wt.GetPath(), Branch: wt.GetBranch(), Active: wt.GetActive()}, nil
}
```

(If [TASK-WT-01-03](./TASK-WT-01-03-project-client-list-worktrees.md) already added `RepoID`/`Active` to `domain.WorktreeRecord`, this snippet's return line matches that; if not, keep the original `WorktreeRecord{ID, Path, Branch}` shape and just add the `baseRef` param.)

`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go` — update the one call site (`create_worktree.go:60`, or the equivalent line inside [TASK-WT-01-05](./TASK-WT-01-05-usecase-wire-validations.md)'s rewritten `Execute` if that task already landed):

```go
	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch, in.BaseRef, in.Lineage)
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: build fails only on test fakes implementing `ProjectClient` until their `RecordWorktreeCreated` signature is updated to match — fix those as part of this task. A regression test (`in.BaseRef` is forwarded) lands in [TASK-WT-04-08](./TASK-WT-04-08-tests-git-gateway.md).
