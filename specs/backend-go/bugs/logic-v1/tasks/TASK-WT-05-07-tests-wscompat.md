# TASK-WT-05-07: Tests for the `worktree.merge` channel and its cleanup composition

**From Solution:** SOL-WT-05
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`
**Depends on:** TASK-WT-05-05
**Status:** `[x]` DONE — TestWorktreeMerge_HappyPath/_ConflictedMerge_NeverCallsRemoveWorktree_EvenWithCleanupIDsSet/_CleanupOneFails_OthersStillRemoved_MergeResponseStillReturned added; all pass

---

## Context

Regression coverage for BR-WT-18's optional cleanup composition, per [SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md)'s Test plan — `_ConflictedMerge_NeverCallsRemoveWorktree_EvenWithCleanupIDsSet` is the key regression guard against auto-cleanup after a failed/conflicted merge.

## Changes to make

Add to `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`:

- `TestWorktreeMerge_HappyPath` — fake `gitClient.MergeBranch` returns a successful, non-conflicted response; no `cleanupWorktreeIds` given; assert the channel returns the raw `MergeBranchResponse` and the fake `RemoveWorktree` was never called.
- `TestWorktreeMerge_ConflictedMerge_NeverCallsRemoveWorktree_EvenWithCleanupIDsSet` — fake `MergeBranch` returns `HasConflicts: true`; `cleanupWorktreeIds` is non-empty; assert the fake `gitClient.RemoveWorktree` recorded zero calls.
- `TestWorktreeMerge_CleanupOneFails_OthersStillRemoved_MergeResponseStillReturned` (BR-WT-18 isolation) — fake `MergeBranch` succeeds without conflicts; fake `RemoveWorktree` errors for one of three `cleanupWorktreeIds`; assert the response's `"merge"` key still carries the successful merge result and `"cleanup"` map has `"removed"` for the two that succeeded and the error string for the one that failed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -run WorktreeMerge
```

Expected: all cases pass.
