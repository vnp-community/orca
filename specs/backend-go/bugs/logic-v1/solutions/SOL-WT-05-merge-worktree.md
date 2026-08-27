# SOL-WT-05: Implement `MergeBranch` (already specified, never built) with 3 strategies, pre-checks, and optional cleanup composition

**Resolves:** [BUG-WT-05](../BUG-WT-05-merge-worktree-partial.md)
**Service:** `git-gateway-service` (new usecase + executor method; completes an RPC the TDD already names) + `api-gateway` (optional cleanup composition at the edge)
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `MergeBranch` RPC (already named in `git-gateway-service.md` §3, never added to the real proto)
- `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` — `GitExecutor.MergeBranch` (new method); `ProjectClient.GetWorktree` (reused from [SOL-WT-04](./SOL-WT-04-so-sanh-worktree.md))
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go` — `MergeBranch` (`git merge`/`git merge --squash`); "rebase" strategy composes the existing `RebaseFromBase`+`FastForward`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` — `worktree.merge` channel, optional `cleanupWorktreeIds` composition calling the already-real `worktree.rm`
- `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### `MergeBranch` is not a new capability — it's a documented RPC that was never built

`git-gateway-service.md` §3's own API surface sketch **already lists**:

```
rpc MergeBranch(MergeBranchRequest) returns (MergeBranchResponse);
```

(`git-gateway-service.md:101`, under "Branch operations"). The bug's own
finding — `grep -n '"merge"' ... executor.go` shows only `merge --abort` —
confirms this RPC was simply never implemented, unlike most of the rest of
§3's sketch (which §10's migration notes say was intentionally
"redesigned against the real agent contract" for several methods, e.g.
`Checkout`/`ListLocalBranches`/`FastForward`/`ConflictOperation`). Nothing
in git-gateway-service.md's migration notes calls out `MergeBranch` as
descoped or redesigned — it's simply missing. **This solution completes an
already-specified RPC**, not a scope extension, unlike SOL-WT-02's fan-out
saga.

### Where the merge target lives: dispatch via `repo_id`, not `worktree_id`

The winning worktree's *branch* is what gets merged, but the merge itself
must run against the repo's own base-branch checkout (typically the
primary clone, not a `project.worktrees` row) — `git merge <branch>` needs
to execute with `base_branch` checked out. `dispatchExecutor`'s existing
doc comment already establishes that its dispatch key is reused,
unmodified, for repo-scoped operations: "`worktreeID` here is also reused,
unchanged, as the dispatch key for the repo-scoped worktree usecases
(`CreateWorktree`, `DetectWorktrees`, ...)" (`ports.go:373-382`) — resolving
via `ProjectClient.GetRepo(repoID)` first, then `dispatchExecutor(...,
repo.ID)`, exactly as `CreateWorktree.Execute` already does
(`create_worktree.go:42-50`). This solution follows that identical
established pattern rather than inventing a new dispatch shape.

### "Rebase" strategy composes two already-real RPCs — no new git logic for that branch

BL-WT-05's "Rebase" merge-strategy choice (rebase the winner's branch onto
main, then fast-forward main to the rebased tip) maps directly onto two
RPCs `git-gateway-service.md` §10 already confirms are real and
"shippable-now": `RebaseFromBase` (rebases a worktree's branch onto a base
ref, `rebase_from_base.go:23-38`) and `FastForward` (`git pull --ff-only`,
`fast_forward.go:27-40`). This solution's `strategy == "rebase"` branch
calls both, in sequence, against the winning worktree and the base-branch
checkout respectively — no new executor method needed for that path, only
for "merge"/"squash".

### Conflict handling reuses the existing conflict RPCs — flagged extension: broaden their dispatch target

`AbortMerge`/`ConflictOperation`/`ResolveConflict` are real
(`gitgateway.proto`) but every existing request message names its dispatch
field `worktree_id` and every existing caller passes an actual
`project.worktrees` row id. A conflicted `MergeBranch` happens against the
**repo's own checkout**, which has no `worktree_id` in
`project-service`'s bookkeeping. Rather than triplicate
`AbortMerge`/`ConflictOperation`/`ResolveConflict` into repo-scoped
duplicates, this solution proposes broadening what `dispatchExecutor`
already treats as an opaque dispatch key: accept either a real
`worktree_id` or a `repo_id` prefixed distinguishably (e.g.
`"repo:" + repo.ID`, resolved by `ConnectionResolver` the same way
`CreateWorktree` already resolves a bare `repo.ID`) so `MergeBranchResponse`
can hand back a dispatch key the client threads straight into the existing
conflict RPCs unmodified. **This is a genuine, explicitly flagged
extension** beyond what `git-gateway-service.md` documents — the TDD never
anticipated a non-worktree dispatch target for the conflict RPCs. Flagging
it rather than silently choosing one of several equally-plausible designs
(the alternative being three new repo-scoped conflict RPCs, rejected here
as needless duplication of already-real logic).

### Cleanup composition (BR-WT-18) belongs at the edge, optional by construction

`RemoveWorktree` (per [SOL-WT-03](./SOL-WT-03-xoa-worktree.md), now
safety-checked) is real and sufficient — BR-WT-18 only requires that
cleanup be *optional*, not that it be a new backend capability. Composing
"call `worktree.rm` for each of these ids after a successful merge" at
`wscompat`, driven entirely by whether the client includes
`cleanupWorktreeIds` in the request, keeps this optional-by-construction
(an empty/absent list is a complete no-op) and reuses
[SOL-WT-02](./SOL-WT-02-fan-out-worktree.md)'s precedent of edge-level
composition over existing per-item RPCs — consistent with
`08-inter-service-communication.md`'s API Gateway "response aggregation"
responsibility, this time for a post-success chained call rather than a
fan-out.

---

## Design — proto (`gitgateway.proto`)

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
  string conflict_dispatch_key = 4; // pass this to AbortMerge/ConflictOperation/ResolveConflict's worktree_id field when has_conflicts=true — see "Conflict handling" above
}
```

---

## Design — usecase

```go
// internal/usecase/merge_worktree_into_base.go
type MergeWorktreeIntoBase struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

type MergeWorktreeInput struct {
	WorktreeID, BaseBranch, Strategy, CommitMessage string
}

func (uc *MergeWorktreeIntoBase) Execute(ctx context.Context, in MergeWorktreeInput) (domain.MergeResult, error) {
	if in.Strategy != "merge" && in.Strategy != "squash" && in.Strategy != "rebase" {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInvalidArgument, "MERGE_STRATEGY_INVALID", "strategy must be merge, squash, or rebase", nil)
	}

	wt, err := uc.projects.GetWorktree(ctx, in.WorktreeID) // SOL-WT-04's new RPC — gives repo_id + branch
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_NOT_FOUND", "worktree not found", err)
	}

	// BR-WT-16 — must commit all changes before merging.
	wtExecutor, wtPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	status, err := wtExecutor.GetStatus(ctx, wtPath)
	if err == nil && len(status.Files) > 0 {
		return domain.MergeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "MERGE_UNCOMMITTED_CHANGES",
			fmt.Sprintf("%d uncommitted change(s) in the winning worktree", len(status.Files)), nil)
	}

	repo, err := uc.projects.GetRepo(ctx, wt.RepoID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	mainExecutor, mainRepoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "REPO_RESOLVE_FAILED", "failed to resolve host", err)
	}

	if in.Strategy == "rebase" {
		// Rebase the winner's branch onto base_branch, then fast-forward
		// base_branch to the rebased tip — both already-real RPCs.
		if _, err := wtExecutor.RebaseFromBase(ctx, wtPath, in.BaseBranch); err != nil {
			return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_REBASE_FAILED", "rebase onto base failed", err)
		}
		ffResult, err := mainExecutor.FastForward(ctx, mainRepoPath, &domain.PushTargetInput{RemoteName: "origin", Branch: wt.Branch})
		if err != nil {
			return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_FAST_FORWARD_FAILED", "fast-forward of base branch failed", err)
		}
		return domain.MergeResult{ResultSHA: ffResult.SHA}, nil
	}

	// "merge" / "squash" — pre-merge conflict signal via BranchCompare's
	// status field before attempting the real merge (best-effort early
	// warning; the real merge attempt below is still the authoritative
	// conflict detector, per BR-WT-17's "manual resolution only" — this
	// solution never auto-resolves, it only surfaces what git itself
	// reports).
	result, err := mainExecutor.MergeBranch(ctx, mainRepoPath, wt.Branch, in.Strategy, in.CommitMessage)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_FAILED", "merge failed", err)
	}
	if result.HasConflicts {
		// BR-WT-17 — deliberately NOT auto-resolved or auto-aborted. The
		// main repo checkout is left in the conflicted state; the client
		// resolves via the existing ConflictOperation/ResolveConflict/
		// AbortMerge RPCs using result.ConflictDispatchKey (see this file's
		// "Conflict handling" design-rationale note).
		return domain.MergeResult{HasConflicts: true, ConflictedPaths: result.ConflictedPaths, ConflictDispatchKey: dispatchKeyForRepo(repo.ID)}, nil
	}
	return domain.MergeResult{ResultSHA: result.ResultSHA}, nil
}
```

`GitExecutor.MergeBranch` (new, `ports.go` + `localgit/executor.go`):

```go
// MergeBranch runs `git merge --no-ff <branch>` ("merge" strategy) or
// `git merge --squash <branch>` followed by `git commit -m <message>`
// ("squash" strategy). "rebase" is handled entirely by the caller composing
// RebaseFromBase+FastForward — this method is never called for that
// strategy, see MergeWorktreeIntoBase.Execute.
func (e *Executor) MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error) {
	args := []string{"merge"}
	switch strategy {
	case "merge":
		args = append(args, "--no-ff", branch)
	case "squash":
		args = append(args, "--squash", branch)
	}
	out, err := e.run(ctx, repoPath, args...)
	if err != nil {
		if isConflictErr(out, err) { // git exits non-zero with "CONFLICT" markers in stdout/stderr on a real conflict
			paths, _ := e.conflictedPaths(ctx, repoPath) // reuses ConflictOperation's existing marker-file scan
			return domain.MergeResult{HasConflicts: true, ConflictedPaths: paths}, nil // not an error — a conflict is an expected outcome BR-WT-17 requires surfacing, not failing
		}
		return domain.MergeResult{}, err
	}
	if strategy == "squash" {
		msg := commitMessage
		if msg == "" {
			msg = "Squash merge " + branch
		}
		if _, err := e.run(ctx, repoPath, "commit", "-m", msg); err != nil {
			return domain.MergeResult{}, err
		}
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.MergeResult{}, err
	}
	return domain.MergeResult{ResultSHA: strings.TrimSpace(sha)}, nil
}
```

---

## Design — wiring (`wscompat`)

```go
r.Register("worktree.merge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type mergeArgs struct {
		WorktreeID, BaseBranch, Strategy, CommitMessage string
		CleanupWorktreeIDs                              []string // BR-WT-18 — optional
	}
	in, err := decodeArg[mergeArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	resp, err := gitClient.MergeBranch(ctx, &gitgatewayv1.MergeBranchRequest{
		WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch, Strategy: in.Strategy, CommitMessage: in.CommitMessage,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetHasConflicts() || len(in.CleanupWorktreeIDs) == 0 {
		return resp, nil // never auto-cleanup on a conflicted merge
	}

	// BR-WT-18 — optional, best-effort, per-item isolated the same way
	// SOL-WT-02's fan-out is (one cleanup failure must not mask the
	// successful merge response).
	cleanupResults := make(map[string]string, len(in.CleanupWorktreeIDs))
	for _, wtID := range in.CleanupWorktreeIDs {
		if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: wtID, Force: false}); err != nil {
			cleanupResults[wtID] = err.Error()
		} else {
			cleanupResults[wtID] = "removed"
		}
	}
	return map[string]any{"merge": resp, "cleanup": cleanupResults}, nil
})
```

---

## Test plan

- `usecase/merge_worktree_into_base_test.go`:
  - `_InvalidStrategy_RejectsBeforeAnyExecutorCall`
  - `_UncommittedChangesInWinner_Rejects` (BR-WT-16, assert `mainExecutor.MergeBranch` never called)
  - `_RebaseStrategy_CallsRebaseFromBaseThenFastForward_NotMergeBranch` (regression guard distinguishing the two code paths)
  - `_MergeStrategy_PassesNoFF`
  - `_SquashStrategy_CommitsWithMessage`
  - `_Conflict_ReturnsHasConflictsTrue_NotAnError` (BR-WT-17 — asserts the usecase does NOT call `AbortMerge` or any resolve method itself)
- `adapter/localgit/executor_test.go` (integration, real git in a temp repo) — `MergeBranch` merge/squash round-trip; a deliberately conflicting merge returns `HasConflicts: true` with correct `ConflictedPaths`, and the repo is left in the conflicted state (not aborted).
- `wscompat/channels_worktree_test.go`:
  - `worktree.merge` happy path
  - `_ConflictedMerge_NeverCallsRemoveWorktree_EvenWithCleanupIDsSet` (regression guard against auto-cleanup after a failed/conflicted merge)
  - `_CleanupOneFails_OthersStillRemoved_MergeResponseStillReturned` (BR-WT-18 isolation)

## References

- `specs/backend-go/bugs/logic-v1/BUG-WT-05-merge-worktree-partial.md` — full gap list
- `specs/backend-go/tdd/services/git-gateway-service.md:101` (§3 — `MergeBranch` already named in the RPC sketch), `:324-352` (§10 migration notes — confirms which methods were redesigned vs. simply unbuilt)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — one-usecase-per-RPC convention this solution follows
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:47-67` (API Gateway response-aggregation responsibility, grounding the optional-cleanup composition)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:366-392` (`dispatchExecutor`'s doc comment — repo-scoped dispatch precedent this solution follows for the merge target)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:42-50` (the `GetRepo` → `dispatchExecutor(..., repo.ID)` pattern reused here)
- `backend-go/services/git-gateway-service/internal/usecase/fast_forward.go:27-40`, `rebase_from_base.go:23-38` (the two already-real RPCs the "rebase" strategy composes)
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:649` (only existing `git merge` invocation is `merge --abort`, confirming the gap)
- `backend-go/services/scm-integration-service/internal/usecase/merge_pull_request.go:1-65` (the unrelated PR-merge capability the bug correctly distinguishes this from)
- [SOL-WT-03](./SOL-WT-03-xoa-worktree.md) — the now-safety-checked `RemoveWorktree` this solution's cleanup composition calls
- [SOL-WT-04](./SOL-WT-04-so-sanh-worktree.md) — the new `ProjectClient.GetWorktree` this solution also depends on
- `docs/logic/worktree-management/BL-WT-05-merge-worktree.md`
