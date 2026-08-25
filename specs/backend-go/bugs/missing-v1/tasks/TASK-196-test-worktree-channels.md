# TASK-196: Tests for the worktree saga usecases, `ForceDeleteBranch`, and `worktree.*` `wscompat` channels

**From Solution:** SOL-031 (design part 5: tests)
**Priority:** P1
**Service:** `git-gateway-service` + `api-gateway`
**File:** `internal/usecase/create_worktree_test.go`, `remove_worktree_test.go`, `force_delete_branch_test.go`, `resolve_pr_base_test.go` (all new, `git-gateway-service`); `services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go` (new)
**Depends on:** TASK-192, TASK-193, TASK-194, TASK-195
**Status:** `[ ]` TODO

---

## Context

Covers the regression-critical paths this solution's design calls out
explicitly: `CreateWorktree`'s compensating rollback (and the
compensation-also-fails case, which must never silently swallow the second
failure), `RemoveWorktree`'s deliberate NO-compensation behavior,
`ForceDeleteBranch`'s compile-time-required-interface-method regression
guard (the direct test for the old TS optional-method crash-bug class),
and `worktree.detectedList`'s cross-service aggregation correctness
(orphan detection, and that one client erroring fails the whole call rather
than returning a partial silent result).

## Changes to make

### `services/git-gateway-service/internal/usecase/create_worktree_test.go` (new)

Fake `ConnectionResolver`/`ProjectClient`/`GitExecutor` (two `GitExecutor`
fakes — `local`/`relay` — per `dispatchExecutor`'s existing dispatch
convention; `ConnectionResolver` fake controls which one is selected).
Cases:

- Happy path: `Execute` returns `{WorktreeID, Path, HeadSHA}` sourced from
  the fake `GitExecutor.CreateWorktree`'s result and the fake
  `ProjectClient.RecordWorktreeCreated`'s returned `WorktreeRecord.ID`.
- `ProjectClient.RecordWorktreeCreated` fails → assert
  `GitExecutor.RemoveWorktree` was called exactly once, with the SAME path
  `CreateWorktree` returned, and `force: true` — the compensation call
  (regression guard against orphaned worktrees). Returned error is
  `WORKTREE_BOOKKEEPING_FAILED`.
- `ProjectClient.RecordWorktreeCreated` fails AND the compensating
  `GitExecutor.RemoveWorktree` ALSO fails → assert the returned error's
  message names BOTH failures and the orphaned path (regression guard:
  never silently swallow the second failure — grep the error string for
  both the bookkeeping error's text and the rollback error's text, not
  just a generic "failed" message).
- `GitExecutor.CreateWorktree` itself fails → `ProjectClient.RecordWorktreeCreated`
  is never called (assert call count 0) and no compensation is attempted
  (nothing to compensate — the git op never succeeded).
- `ProjectClient.GetRepo` fails → `WORKTREE_REPO_NOT_FOUND`, no
  `GitExecutor` method is called at all.

### `services/git-gateway-service/internal/usecase/remove_worktree_test.go` (new)

Cases:

- Happy path: `GitExecutor.RemoveWorktree` then `ProjectClient.RecordWorktreeRemoved`,
  both called exactly once, `Execute` returns `nil`.
- `ProjectClient.RecordWorktreeRemoved` fails → assert `GitExecutor.CreateWorktree`
  and `GitExecutor.RemoveWorktree` were each called EXACTLY the number of
  times the happy path calls them (i.e. `RemoveWorktree` once, `CreateWorktree`
  zero times) — explicitly asserting NO compensating git operation runs on
  this failure path, unlike `CreateWorktree`'s. Returned error is
  `WORKTREE_BOOKKEEPING_STALE`, not a generic internal error.
- `GitExecutor.RemoveWorktree` itself fails → `ProjectClient.RecordWorktreeRemoved`
  is never called (assert call count 0), returned error is
  `WORKTREE_REMOVE_FAILED`.

### `services/git-gateway-service/internal/usecase/force_delete_branch_test.go` (new)

Table-driven over BOTH `localgit.Executor` (real, exercising the actual
`git branch -D` command against a temp repo — follow whatever
`localgit`-package test harness this repo's existing `localgit` tests
already use, e.g. a `t.TempDir()` + `git init` + `git branch` fixture) AND
a fake relay `GitExecutor` — this is the direct regression test for the old
optional-interface-method crash-bug class BUG-031 cites: because
`GitExecutor.ForceDeleteBranch` is now a REQUIRED interface method
(TASK-194), a test asserting `usecase.ForceDeleteBranch.Execute` succeeds
against BOTH implementations would have failed to even COMPILE under the
old TS-style optional-interface design (there is no way in Go to "forget"
to implement a required interface method and still satisfy the interface).
Cases:

- `localgit.Executor.ForceDeleteBranch` deletes an existing local branch in
  a real temp git repo; the branch is confirmed gone via `git branch --list`.
- Fake relay `GitExecutor.ForceDeleteBranch` returning
  `grpcclient.ErrForceDeleteBranchUnsupported` (or `domain`'s equivalent
  sentinel, per TASK-194 Step 4's import-cycle resolution) → `usecase.ForceDeleteBranch.Execute`
  returns `WORKTREE_FORCE_DELETE_UNSUPPORTED` (assert via `apperrors.ToGRPCStatus`'s
  resulting `codes.FailedPrecondition`), not a generic internal error and
  not a panic.
- Fake relay `GitExecutor.ForceDeleteBranch` returning any OTHER error →
  `WORKTREE_FORCE_DELETE_FAILED`, distinct from the unsupported case above.

### `services/git-gateway-service/internal/usecase/resolve_pr_base_test.go` (new)

Fake `SCMClient` + fake `GitExecutor`. Cases:

- Happy path: `SCMClient.GetPullRequestBase` returns a branch,
  `GitExecutor.FetchAndResolveRef` resolves it to a SHA — `Execute` returns
  `{Branch, SHA}` matching both fakes' outputs.
- `SCMClient.GetPullRequestBase` succeeds but `GitExecutor.FetchAndResolveRef`
  fails (base ref not fetchable locally) → `WORKTREE_BASE_REF_UNRESOLVABLE`,
  and the returned error does NOT leak the SCM-side branch name/SHA data as
  if it were a resolved result (assert the returned `domain.ResolvedBase`
  is the zero value on error).
- `SCMClient.GetPullRequestBase` itself fails (e.g. the
  `scmclient.Client`'s current `apperrors.KindUnimplemented` stub from
  TASK-193 Step 5, until that RPC exists) → `WORKTREE_PR_BASE_LOOKUP_FAILED`,
  `GitExecutor.FetchAndResolveRef` is never called.

Add an equivalent, symmetrical `resolve_mr_base_test.go` covering the same
three cases through `SCMClient.GetMergeRequestBase`/`ResolveMrBase.Execute`
— may be a near-duplicate of `resolve_pr_base_test.go`'s cases with
`Mr`/`Pr` swapped; do not skip it just because it mirrors the PR version.

### `services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go` (new)

Fake `gitgatewayv1.GitGatewayServiceClient` + fake
`projectv1.ProjectServiceClient` (follow `channels_test.go`'s existing
fake-client pattern in this package). Cases:

- One test per direct unary wrapper (`worktree.create`, `worktree.rm`,
  `worktree.forceDeleteBranch`, `worktree.prefetchCreateBase`,
  `worktree.resolvePrBase`, `worktree.resolveMrBase`, `worktree.list`,
  `worktree.set`) — asserts the decoded args map onto the expected request
  fields and the response is returned unmodified (or, for `worktree.list`/
  `worktree.set`, that these call `projectClient`, NOT `gitClient` — a
  regression guard on BUG-031's "always-local bookkeeping, no
  git-gateway-service involvement" dispatch-model finding).
- `worktree.detectedList`: a path present on disk (fake
  `GitGatewayServiceClient.DetectWorktrees`'s `OnDiskPaths`) but absent
  from bookkeeping (fake `ProjectServiceClient.ListWorktrees`'s
  `Worktrees`) appears in `orphanedPaths`; a path present in both does
  NOT appear.
- `worktree.detectedList`: the fake `GitGatewayServiceClient.DetectWorktrees`
  call errors → the whole channel call fails (via `errgroup.Wait`), and
  `orphanedPaths` is never computed from a partial result — assert the
  handler's returned error is non-nil and no result value leaks through.
- `worktree.detectedList`: both fake calls succeed with zero elements each
  → `orphanedPaths` is an empty (not nil-vs-empty-ambiguous — pick
  whichever this handler's implementation actually produces and assert
  that consistently) slice, not an error.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go test ./internal/usecase/... ./internal/adapter/localgit/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -count=1 -v
```
