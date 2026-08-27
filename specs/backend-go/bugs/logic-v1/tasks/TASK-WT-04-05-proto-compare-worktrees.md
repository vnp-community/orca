# TASK-WT-04-05: Proto — `CompareWorktrees` RPC; `ProjectClient.GetWorktree` port

**From Solution:** SOL-WT-04
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** TASK-WT-04-03
**Status:** `[x]` DONE — Added CompareWorktrees RPC + messages to gitgateway.proto; ProjectClient.GetWorktree + domain.WorktreeInfo; grpcclient implementation. buf generate + go build clean.

---

## Context

Per [SOL-WT-04](../solutions/SOL-WT-04-so-sanh-worktree.md)'s finding, 3 of BL-WT-04's 4 comparison columns (file count, lines added/removed, per-file status) are already available from `BranchCompare` — already real and wired (`GitChangeEntry.added/removed` at `gitgateway.proto:268-274`, confirmed populated by `localgit.Executor.BranchCompare` at `executor.go:1019`). This task adds the aggregation RPC plus the `ProjectClient.GetWorktree` port needed to enforce BR-WT-13 (same `base_ref` across compared worktrees).

## Changes to make

Add to `service GitGatewayService` in `gitgateway.proto` (near `rpc BranchCompare`, line 41):

```protobuf
  rpc CompareWorktrees(CompareWorktreesRequest) returns (CompareWorktreesResponse);
```

Add the messages (near `BranchCompareResponse`, `gitgateway.proto:307-318`):

```protobuf
message CompareWorktreesRequest {
  repeated string worktree_ids = 1; // 2..N worktrees being compared
}
message CompareWorktreesResponse {
  string base_ref = 1;              // the shared base branch (BR-WT-13)
  repeated WorktreeComparison worktrees = 2;
}
message WorktreeComparison {
  string worktree_id = 1;
  int32 changed_files = 2;
  int32 added_lines = 3;
  int32 removed_lines = 4;
  string merge_base = 5;   // for BR-WT-14 cross-checking — see compare_worktrees.go's usecase comment
  string status = 6;       // BranchCompareResponse.status passthrough
  string error_message = 7;
}
```

`backend-go/services/git-gateway-service/internal/usecase/ports.go` — add `GetWorktree` to `ProjectClient`:

```go
type ProjectClient interface {
	GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error)
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch, baseRef string, lineage domain.WorktreeLineageCapture) (domain.WorktreeRecord, error)
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
	ListWorktrees(ctx context.Context, projectID string) ([]domain.WorktreeRecord, error)
	// GetWorktree wraps project-service's new GetWorktree RPC (added
	// alongside base_ref persistence) — CompareWorktrees uses it to look up
	// each compared worktree's repo_id/branch/base_ref.
	GetWorktree(ctx context.Context, worktreeID string) (domain.WorktreeInfo, error)
}
```

Add `domain.WorktreeInfo` to `domain.go` (distinct from the existing, narrower `WorktreeRecord` — this carries `BaseRef`, which `WorktreeRecord` doesn't):

```go
// WorktreeInfo is project-service's GetWorktree answer — the richer shape
// CompareWorktrees needs (RepoID + Branch + BaseRef), vs. WorktreeRecord's
// narrower ID/Path/Branch used by CreateWorktree's own bookkeeping call.
type WorktreeInfo struct {
	ID      string
	RepoID  string
	Branch  string
	BaseRef string // empty = never backfilled (worktree created before TASK-WT-04-01)
}
```

Implement in `backend-go/services/git-gateway-service/internal/adapter/grpcclient/project_client.go`:

```go
func (p *ProjectClient) GetWorktree(ctx context.Context, worktreeID string) (domain.WorktreeInfo, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeInfo{}, err
	}
	wt, err := p.client.GetWorktree(ctx, &projectv1.GetWorktreeRequest{WorktreeId: worktreeID})
	if err != nil {
		return domain.WorktreeInfo{}, err
	}
	return domain.WorktreeInfo{ID: wt.GetId(), RepoID: wt.GetRepoId(), Branch: wt.GetBranch(), BaseRef: wt.GetBaseRef()}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/... ./services/git-gateway-service/...
```

Expected: clean build; only additions in `buf breaking`. Any fake `ProjectClient` in `services/git-gateway-service/internal/usecase/*_test.go` needs a `GetWorktree` stub added to keep compiling — fix as part of this task.
