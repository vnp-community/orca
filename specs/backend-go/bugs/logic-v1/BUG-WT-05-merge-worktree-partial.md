# BUG-WT-05: No local "merge worktree branch into main" operation — only PR/MR-based merge exists, with no strategy choice or pre-merge checks

**Business Logic:** [BL-WT-05](../../../../docs/logic/worktree-management/BL-WT-05-merge-worktree.md) — Merge Worktree Thắng vào Nhánh Chính
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** There is no backend operation that takes a winning worktree's branch and merges it into the repo's main branch directly (locally), with a choice of merge-commit/squash/rebase strategy, after checking for conflicts and that everything is committed. The only real "merge" capability anywhere in backend-go is `scm-integration-service`'s `MergePullRequest`, which merges an *already-opened* GitHub/GitLab pull/merge request via the provider's API — a materially different precondition and flow than the spec describes, and one that implements none of the spec's pre-merge checks. Cleanup of the losing worktrees (BR-WT-18) is composable from the already-real `worktree.rm`, but nothing orchestrates "merge, then optionally clean up the rest" as one flow.

---

## Spec summary

`BL-WT-05` describes merging the chosen "winner" worktree's changes into the main branch after two checks (no conflict with main, all changes committed), with the user picking a merge strategy (merge commit / squash / rebase), then optionally cleaning up the other worktrees. Conflict resolution must always be manual (BR-WT-17), and cleanup is optional (BR-WT-18).

## What backend-go has

- Real, unrelated merge capability: `MergePullRequest` (`backend-go/services/scm-integration-service/internal/usecase/merge_pull_request.go:34-59`) resolves a provider credential, resolves the SCM provider adapter, then calls `provider.MergePullRequest(ctx, cred, repo, number, MergePullRequestInput{MergeMethod, CommitTitle, CommitMessage})` — i.e. it merges a pull/merge request that must already exist on GitHub/GitLab, via that provider's own merge API, not a local `git merge` against the worktree's branch. `MergeMethod` is the closest thing to a strategy choice (`gitgateway`/`scmintegration` proto: `MergePullRequestRequest`), and it is real, tested, and wired through wscompat (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:83-90`).
- Adjacent git primitives in `git-gateway-service` that operate in the *opposite* direction from what's needed: `FastForward` (`backend-go/services/git-gateway-service/internal/usecase/fast_forward.go:27-40`) runs `git pull --ff-only [<remote> <branch>]` *inside the worktree*, updating the worktree from a remote/push target — it does not merge the worktree's changes into main. `RebaseFromBase` (`backend-go/services/git-gateway-service/internal/usecase/rebase_from_base.go:23-38`) rebases the worktree's own branch onto a base ref — again, brings base changes into the worktree, not the reverse.
- `AbortMerge`/`AbortRebase`/`ConflictOperation`/`ResolveConflict` RPCs exist (`gitgateway.proto`) for handling an in-progress merge/rebase conflict inside a worktree — but there is no corresponding "start a merge" operation for them to be aborting/resolving in this flow; `grep -n '"merge"' backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` shows the only `git merge` invocation anywhere in the executor is `merge --abort` (line 649) — confirming no code path ever runs `git merge <branch>` to begin one.
- `RemoveWorktree` (`backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`, see `BUG-WT-03`) is real and could serve BR-WT-18's optional cleanup step if a caller invoked it per losing worktree — but nothing wires "merge succeeded" to "offer cleanup of the others."

## What's missing

- **No local merge-into-main operation**: no RPC, usecase, or executor method runs `git merge <worktree-branch>` against the base/main branch. The spec's core action ("Hệ thống thực thi merge" after choosing a strategy) has no backend counterpart for the local-worktree-into-main case at all.
- **No merge-strategy selection for a local merge**: "merge commit / squash / rebase" as a strategy choice only exists on the unrelated `MergePullRequest` path (`MergeMethod` field), which requires a pre-existing SCM pull/merge request — it cannot be used to merge an arbitrary worktree branch into local `main` without first pushing the branch and opening a PR/MR (a separate flow, not part of this saga).
- **BR-WT-16** (must commit all changes before merging): no pre-merge check exists anywhere — `MergePullRequest.Execute` never inspects the worktree's git status; there is no code path that checks "all changes committed" before any merge-like action.
- **Pre-merge conflict check**: no usecase calls anything resembling "will this merge conflict with main" before executing a merge — `BranchCompare`'s `status` field (`"ready" | "invalid-base" | "unborn-head" | "no-merge-base" | "loading" | "error"`, `gitgateway.proto`) could in principle inform this, but nothing in a merge flow calls it as a precondition.
- **BR-WT-17** (conflict resolution must always be manual, never auto-resolved): trivially true today only because no merge operation exists to auto-resolve in the first place, not because of a deliberate no-auto-resolve guard in a real merge flow.
- **BR-WT-18** (optional cleanup of other worktrees after merge): no orchestration ties a successful merge to a "delete the other N-1 worktrees" step — `RemoveWorktree` exists and is real (see `BUG-WT-03`) but must be invoked separately, N times, by the caller with no backend awareness that a merge just happened.

## See also

- `specs/backend-go/bugs/logic-v1/BUG-WT-03-xoa-worktree-partial.md` — the `RemoveWorktree` usecase this flow's optional cleanup step would need to call.
- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — background on which `git.*` methods are and aren't wired; the merge-adjacent methods it lists (`git.abortMerge`, `git.checkout`, `git.fastForward`, `git.rebaseFromBase`) are real RPCs per this report's own findings, but none of them constitute a "start a merge" operation.

## References

- `backend-go/services/scm-integration-service/internal/usecase/merge_pull_request.go:1-59`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:83-90`
- `backend-go/services/git-gateway-service/internal/usecase/fast_forward.go:1-40`
- `backend-go/services/git-gateway-service/internal/usecase/rebase_from_base.go:1-38`
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:649` — only `git merge` invocation in the executor is `merge --abort`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `AbortMergeRequest`/`AbortMergeResponse`, `FastForwardRequest`, `RebaseFromBaseRequest`, `BranchCompareResponse.status`
- `docs/logic/worktree-management/BL-WT-05-merge-worktree.md`
