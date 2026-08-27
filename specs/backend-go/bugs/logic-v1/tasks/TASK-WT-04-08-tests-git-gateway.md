# TASK-WT-04-08: Tests for `base_ref` forwarding and `CompareWorktrees` (BR-WT-13/14)

**From Solution:** SOL-WT-04
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/compare_worktrees_test.go` (new)
**Depends on:** TASK-WT-04-04, TASK-WT-04-06
**Status:** `[x]` DONE — Added TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated + full compare_worktrees_test.go (5 cases incl. fail-fast contrast comment vs SOL-WT-02). git-gateway grpc/server_test.go gained CompareWorktrees/CheckWorktreeDeleteSafety/RemoveWorktree/MergeBranch contract tests; project-service grpc/server_test.go created (new file) with GetWorktree contract tests. go test clean across both services.

---

## Context

Regression coverage per [SOL-WT-04](../solutions/SOL-WT-04-so-sanh-worktree.md)'s Test plan — `TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated` is the direct regression guard against the confirmed silent-drop bug this task set fixes.

## Changes to make

Add to `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go`:
- `TestCreateWorktree_ForwardsBaseRefToRecordWorktreeCreated` — fake `ProjectClient.RecordWorktreeCreated` captures its `baseRef` arg; assert it equals `in.BaseRef`.

Create `backend-go/services/git-gateway-service/internal/usecase/compare_worktrees_test.go`:
- `TestCompareWorktrees_LessThanTwoWorktrees_Rejects` — 0 and 1-element inputs both return `COMPARE_NEEDS_AT_LEAST_TWO`.
- `TestCompareWorktrees_BaseRefMismatch_RejectsBeforeAnyBranchCompareCall` (BR-WT-13) — fake `GetWorktree` returns differing `BaseRef` for two ids; assert the fake executors' `BranchCompare` was never called.
- `TestCompareWorktrees_MissingBaseRef_RejectsWithClearCode` — fake `GetWorktree` returns an empty `BaseRef` for one id; assert `WORKTREE_BASE_REF_UNKNOWN`.
- `TestCompareWorktrees_AggregatesAddedRemovedFromEntries` — fake `BranchCompare` returns multiple `GitChangeEntry` values per worktree; assert `AddedLines`/`RemovedLines` are the correct per-worktree sums.
- `TestCompareWorktrees_OneWorktreeCompareFails_WholeCallFails` — fake `BranchCompare` errors for one of three worktree ids; assert the whole call returns `COMPARE_WORKTREES_FAILED` (deliberately fail-fast, contrast explicitly against [SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md)'s per-item isolation — a comment in the test should say why this usecase does NOT use that pattern).

`backend-go/services/git-gateway-service/internal/adapter/grpc/server_test.go` — add a contract test for `CompareWorktrees`/`GetWorktree` (the latter via `project-service`'s own `server_test.go`), asserting request/response field mapping.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/usecase/... ./services/git-gateway-service/internal/adapter/grpc/...
```

Expected: all cases pass.
