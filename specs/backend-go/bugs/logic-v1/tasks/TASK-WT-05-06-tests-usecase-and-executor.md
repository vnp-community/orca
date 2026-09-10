# TASK-WT-05-06: Tests for `MergeWorktreeIntoBase` and `localgit.MergeBranch`

**From Solution:** SOL-WT-05
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base_test.go` (new)
**Depends on:** TASK-WT-05-03, TASK-WT-05-04
**Status:** `[x]` DONE — Added merge_worktree_into_base_test.go (7 cases) and localgit executor_test.go MergeBranch cases (merge/squash/conflict, real git in temp repo). go test clean.

---

## Context

Regression coverage per [SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md)'s Test plan — `_RebaseStrategy_CallsRebaseFromBaseThenFastForward_NotMergeBranch` is the key regression guard distinguishing the two code paths (rebase composes existing RPCs; merge/squash calls the new `MergeBranch`).

## Changes to make

Create `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base_test.go` with fake `ProjectClient`/`GitExecutor` (following this package's existing fake conventions, see `worktree_fakes_test.go`):

- `TestMergeWorktreeIntoBase_InvalidStrategy_RejectsBeforeAnyExecutorCall` — assert zero calls on both fake executors.
- `TestMergeWorktreeIntoBase_UncommittedChangesInWinner_Rejects` (BR-WT-16) — fake `GetStatus` on the winning worktree returns non-empty files; assert `mainExecutor.MergeBranch` is never called.
- `TestMergeWorktreeIntoBase_RebaseStrategy_CallsRebaseFromBaseThenFastForward_NotMergeBranch` — assert the fake `wtExecutor.RebaseFromBase` and `mainExecutor.FastForward` are called, and `mainExecutor.MergeBranch` is NOT.
- `TestMergeWorktreeIntoBase_MergeStrategy_PassesNoFF` — assert the fake `mainExecutor.MergeBranch` call recorded `strategy == "merge"`.
- `TestMergeWorktreeIntoBase_SquashStrategy_CommitsWithMessage` — assert the fake call recorded `strategy == "squash"` and the given `commitMessage`.
- `TestMergeWorktreeIntoBase_Conflict_ReturnsHasConflictsTrue_NotAnError` (BR-WT-17) — fake `MergeBranch` returns `HasConflicts: true`; assert `Execute` returns a nil error and `HasConflicts: true`, and that the usecase does NOT call `AbortMerge` or any resolve method itself (assert those fakes recorded zero calls, if present in this test's fake `GitExecutor`).
- `TestMergeWorktreeIntoBase_RepoScopedDispatch_UsesRepoIDNotWorktreeID` — assert `mainExecutor`'s `MergeBranch`/`FastForward` calls were dispatched via `repo.ID`, not `in.WorktreeID` (regression guard for the repo-scoped dispatch pattern this usecase follows from `CreateWorktree`).

`backend-go/services/git-gateway-service/internal/adapter/localgit/executor_test.go` (integration, real git in a temp repo) — add:
- `TestMergeBranch_MergeStrategy_RoundTrip` — merge a feature branch with no conflicts, assert `ResultSHA` is non-empty and `HasConflicts` is false.
- `TestMergeBranch_SquashStrategy_SingleCommitWithMessage` — assert exactly one new commit is created with the given message.
- `TestMergeBranch_DeliberateConflict_ReturnsHasConflictsTrue_RepoLeftConflicted` — set up two branches with a conflicting change to the same file/line; assert `HasConflicts: true`, `ConflictedPaths` contains the right file, and the repo is left in the conflicted state (not aborted — a subsequent `git status` in the temp repo shows unmerged paths).

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/usecase/... -run MergeWorktreeIntoBase
go test ./services/git-gateway-service/internal/adapter/localgit/... -run MergeBranch
```

Expected: all cases pass.
