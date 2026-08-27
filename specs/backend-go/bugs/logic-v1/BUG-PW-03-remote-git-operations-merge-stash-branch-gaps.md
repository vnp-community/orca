# BUG-PW-03: Remote Git Operations — no merge, no stash, no branch create/soft-delete, no push/pull progress streaming

**Business Logic:** [BL-PW-03](../../../../docs/logic/project-workspace/BL-PW-03-remote-git-operations.md) — Remote Git UI Operations
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A developer can stage/unstage/discard, commit (with AI-generated message), push, pull (with conflict detection), view history, compare branches/commits, and open a PR (with AI-generated description) — all for real. But four explicit acceptance-criteria actions have no backend-go RPC at all: starting a merge, stashing/popping changes, and creating or soft-deleting a branch. Push/pull also cannot show live progress output — the UI can only get a final success/failure result, never the streamed "Counting objects... / Writing objects... 100%" lines the spec calls for.

---

## Spec summary

BL-PW-03 covers the full remote git workflow executed through the relay/dispatcher (`git-gateway-service`): status, diff, stage/unstage/discard, commit (manual + AI), push/pull with streamed progress, conflict detection and AI-assisted resolution, branch list/create/checkout/delete, merge (no-ff), rebase, stash push/pop, log, worktree switching, and PR creation (GitHub CLI or API token) with AI-generated description.

## What backend-go has

`git-gateway-service`'s proto surface has grown far beyond the original 6-RPC set that `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` documented (that report, describing only `GetStatus`/`GetDiff`/`Commit`/`Push`/`Pull`/`GenerateCommitMessage`, is now stale) — it now has ~55 RPCs, and nearly all of them are wired into `wscompat` for real via `registerGitDeepChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:46-700`) and `registerFilesChannels`, both confirmed live in `RegisterRealChannels` (`channels.go:116-117`). Concretely wired and backed by real gRPC/usecase implementations:

- `git.status`, `git.diff` — `channels.go:267-281`
- `git.commit`, `git.push`, `git.pull`, `git.generateCommitMessage` — `channels_git.go:49-118`
- `git.stage`/`bulkStage`, `git.unstage`/`bulkUnstage`, `git.discard`, `git.bulkDiscard` — `channels_git.go:297-367`
- `git.history`, `git.checkIgnored`, `git.forkSync`, `git.upstreamStatus`, `git.commitCompare`, `git.branchCompare`, `git.commitDiff`, `git.branchDiff`, `git.submoduleStatus`, `git.fetch` — `channels_git.go:378-607`
- `git.remoteCommitUrl`, `git.remoteFileUrl`, `git.generatePullRequestFields`, `git.discoverCommitMessageModels` — `channels_git.go:608-700`
- `git.checkout`, `git.localBranches`, `git.fastForward`, `git.rebaseFromBase`, `git.abortRebase`, `git.abortMerge`, `git.conflictOperation`, `git.resolveConflict` — `channels_git.go:151-296`
- Worktree switcher: `worktree.list`, `worktree.set`, `worktree.create`, `worktree.rm`, `worktree.detectedList`, `worktree.forceDeleteBranch`, `worktree.prefetchCreateBase`, `worktree.resolvePrBase`, `worktree.resolveMrBase` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-240` (this resolves `specs/backend-go/bugs/missing-v1/BUG-031-worktree-channels-not-implemented.md`'s findings for `list`/`set`/`rm`/`forceDeleteBranch`/`prefetchCreateBase`/`resolvePrBase`/`resolveMrBase`/`detectedList` — that report is now stale).
- PR creation (Category B / API-token method): `scm.*` → `ScmIntegrationServiceClient.CreatePullRequest` — `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:757-758+`, backed by real proto `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:14`.

## What's missing

- **No "start a merge" RPC.** `gitgateway.proto` has `AbortMerge` (`:118-119`) and a merge-conflict *detector* (`ConflictOperation`, `:120`), but no RPC that actually runs `git merge --no-ff <branch>`. `git-gateway-service`'s usecase directory has `abort_merge.go` but no `merge.go` (confirmed via directory listing). Acceptance criterion "Merge branch (no-ff)" has no backing implementation.
- **No stash support at all.** No `Stash`/`StashPop` RPC anywhere in `gitgateway.proto`, and no `stash*.go` usecase file in `git-gateway-service/internal/usecase/`. Acceptance criterion "Stash push + pop" is entirely unimplemented.
- **No branch-create RPC.** `CheckoutRequest`'s own doc comment (`gitgateway.proto:653-661`) explicitly drops create-branch semantics ("the original design's `create` (git checkout -b) field was dropped... a caller that wants checkout-with-create composes it as a separate branch-creation step (e.g. `git.exec`...)") — but `git.exec` was deliberately never carried forward to backend-go (per `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md`'s Description section, confirmed still true: no `git.exec` channel or RPC exists). Net effect: **there is no way to create a new branch in backend-go at all**, only to check out an existing one (`git.checkout`, `channels_git.go:151-166`).
- **Branch delete only exists as a hard/force delete.** `ForceDeleteBranchRequest` (`gitgateway.proto:634-637`) maps to `git branch -D` semantics only; there is no soft `git branch -d` RPC, so the spec's `'git.branch.delete' → git branch -d|-D <branch>` (BL-PW-03 doc line 34) is only half-covered.
- **No push/pull progress streaming.** Every git RPC in `gitgateway.proto` is a plain unary request/response (confirmed via `grep -in stream backend-go/proto/orca/gitgateway/v1/gitgateway.proto` returning no RPC-level matches) — `PushResponse`/`PullResponse` are single-shot `{success, had_conflicts}` messages (`gitgateway.proto:166-177`). The spec's `pushWithProgress()` async generator (BL-PW-03 doc lines 99-114), which streams lines like "Counting objects: 5, done." to a progress panel, has no server-streaming RPC to back it — `git.push`/`git.pull` return only a final result.
- **PR creation Method 1 (GitHub CLI on the dev server, with per-user `ghConfigDir` isolation) is absent** — only Method 2 (API token via `scm-integration-service`) is wired. The spec allows either method, so this is not a hard blocker for the AC, but it is a real capability gap versus the documented flow.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-032-git-channels-partially-implemented.md` — now stale for the 32 `git.*` methods it lists as missing (all but `git.cancelGenerateCommitMessage`/`git.cancelGeneratePullRequestFields` are now wired); its `git.exec` "deliberately absent" finding is the direct root cause of this report's branch-create gap.
- `specs/backend-go/bugs/missing-v1/BUG-031-worktree-channels-not-implemented.md` — now stale; all 8 methods it lists are wired in `channels_worktree.go`.
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — now stale; see `BUG-PW-02` in this directory for the current, narrower file-explorer gap.

## References

- `docs/logic/project-workspace/BL-PW-03-remote-git-operations.md:19-44,97-114,174-190` — relay git command map, push/pull progress-stream contract, acceptance criteria
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:18-124` — full `GitGatewayService` RPC surface (no Merge/Stash RPCs, no branch-create RPC)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:634-637` — `ForceDeleteBranchRequest` (force-only)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:653-661` — `CheckoutRequest` doc comment dropping create-branch semantics, deferring to unbuilt `git.exec`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:166-177` — `PushResponse`/`PullResponse` (unary, no streaming)
- `backend-go/services/git-gateway-service/internal/usecase/` — directory listing confirms no `merge.go`/`stash*.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:46-975` — `registerGitDeepChannels`/`registerFilesChannels`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:116-119` — confirms both are live in `RegisterRealChannels`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:36-240` — `registerWorktreeChannels`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:757-758` — PR creation via `scm-integration-service`
