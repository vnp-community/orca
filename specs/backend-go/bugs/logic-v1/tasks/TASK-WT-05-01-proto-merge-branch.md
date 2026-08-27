# TASK-WT-05-01: Proto — `MergeBranch` RPC; `FastForwardResponse` gains `result_sha`

**From Solution:** SOL-WT-05
**Priority:** P0 — every other task in this set depends on these wire types
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

[SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md) completes an RPC `git-gateway-service.md` §3 already names (`rpc MergeBranch(MergeBranchRequest) returns (MergeBranchResponse);`, `git-gateway-service.md:101`) but was never added to the real proto — confirmed by this bug's own finding (`grep -n '"merge"' executor.go` shows only `merge --abort`). Not a scope extension.

**Correction against the SOL's own sketch**: the "rebase" merge strategy composes `RebaseFromBase` + `FastForward` (both already real), and needs to report a `result_sha` — but the real `domain.FastForwardResult`/`FastForwardResponse` carry only `Success bool`, no SHA (confirmed in this task set's research: `domain.go:313-315`, `gitgateway.proto:720-722`). This task extends `FastForwardResponse`/`FastForwardResult` with a `result_sha` field so the merge usecase's rebase path can report one without inventing a new GitExecutor method — the same rev-parse-after-op pattern `CreateWorktree`'s executor already uses for `WorktreeCreateResult.HeadSHA`.

## Changes to make

Add to `service GitGatewayService` (near `rpc AbortMerge`, `gitgateway.proto:119`):

```protobuf
  rpc MergeBranch(MergeBranchRequest) returns (MergeBranchResponse);
```

Add the messages (near `AbortMergeResponse`):

```protobuf
message MergeBranchRequest {
  string worktree_id = 1;    // the WINNING worktree; its branch is merged INTO base_branch
  string base_branch = 2;    // typically the repo's default branch
  string strategy = 3;       // "merge" | "squash" | "rebase"
  string commit_message = 4; // optional override for the merge-commit/squash commit
}
message MergeBranchResponse {
  string result_sha = 1;
  bool   has_conflicts = 2;
  repeated string conflicted_paths = 3;
  string conflict_dispatch_key = 4; // pass this to AbortMerge/ConflictOperation/ResolveConflict's worktree_id field when has_conflicts=true — see merge_worktree_into_base.go's usecase doc comment
}
```

Extend the existing `FastForwardResponse` (`gitgateway.proto:720-722`) — additive, does not break existing callers that ignore the new field:

```protobuf
message FastForwardResponse {
  bool success = 1;
  string result_sha = 2; // NEW — HEAD's SHA after a successful fast-forward; needed by MergeBranch's "rebase" strategy to report result_sha
}
```

Extend `domain.FastForwardResult` (`domain.go:313-315`):

```go
type FastForwardResult struct {
	Success   bool
	ResultSHA string // NEW
}
```

Update `localgit.Executor.FastForward` (`executor.go:614-623`) to populate it via a trailing `rev-parse HEAD`:

```go
func (e *Executor) FastForward(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.FastForwardResult, error) {
	args := []string{"pull", "--ff-only"}
	if pushTarget != nil {
		args = append(args, pushTarget.RemoteName, pushTarget.BranchName)
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.FastForwardResult{}, err
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.FastForwardResult{}, err
	}
	return domain.FastForwardResult{Success: true, ResultSHA: strings.TrimSpace(sha)}, nil
}
```

Update `RelayExecutor.FastForward` (in `internal/adapter/grpcclient/relay_executor.go`) to decode the same new field from the agent's response — check that file's existing `FastForward` implementation for its `relay(...)` call shape and add `ResultSha` to the decoded result struct alongside `Success`, matching this file's existing pattern for every other method.

Update the gRPC handler for `FastForward` (in `internal/adapter/grpc/server.go`) to pass `ResultSha: result.ResultSHA` through on the response.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/... ./services/git-gateway-service/...
```

Expected: clean build; `buf breaking` reports only additions. `go build` on `services/git-gateway-service` will fail until `FastForward`'s callers/fakes are updated — fix as part of this task so the package compiles standalone.
