# BUG-WT-01: Worktree creation has no business-rule validation — relies entirely on git's raw errors

**Business Logic:** [BL-WT-01](../../../../docs/logic/worktree-management/BL-WT-01-tao-worktree.md) — Tạo Worktree
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user (or agent orchestrator) can call `worktree.create` and get a real worktree on disk plus a real Postgres bookkeeping row — the happy path genuinely works end-to-end. But every named alternate flow and business rule in the spec (name collisions, missing base branch, disk space, name-character restrictions, a 20-worktree cap) is unenforced: the caller gets git's raw stderr wrapped in a generic `WORKTREE_CREATE_FAILED`/`WORKTREE_RESOLVE_FAILED` error instead of the specific guidance the spec promises (alternate-name suggestion, branch list, disk warning). There is also no way to request a custom worktree name/path at all — the spec's own input contract (`name?`, `path?`) has no backend field to carry it.

---

## Spec summary

`BL-WT-01` describes creating a git worktree: pick a base branch, optionally name it and pick a path, run `git worktree add`, write a DB record, start a terminal, and add a sidebar card. It defines 4 business rules (name charset, path-outside-`.git`, no duplicate paths, max 20 worktrees/repo) and 3 alternate flows for name-collision, missing-branch, and low-disk conditions, each with specific recovery UX (suggest an alternate name, list available branches, warn about disk space).

## What backend-go has

- Real end-to-end saga: `CreateWorktree.Execute` (`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:41-71`) resolves the repo via `ProjectClient.GetRepo`, dispatches to local/relay `GitExecutor.CreateWorktree`, then calls `ProjectClient.RecordWorktreeCreated` — with best-effort compensation (`executor.RemoveWorktree`) if bookkeeping fails after the git op succeeds (`create_worktree.go:60-69`).
- Real gRPC surface: `CreateWorktreeRequest`/`CreateWorktreeResponse` (`backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — see `CreateWorktreeRequest` fields: `project_id`, `repo_id`, `branch`, `base_ref`, plus optional lineage fields only) and `Server.CreateWorktree` (`backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:678-695`).
- Real Postgres persistence: `WorktreeRepository.RecordWorktreeCreated` (`backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go:28-47`), inserting into `project.worktrees`.
- Real WS surface: `worktree.create` is registered and wired (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:37-73`), and `registerWorktreeChannels` is actually called from `RegisterRealChannels` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:119`) — the wiring gap `BUG-031` originally reported is resolved; see "See also".
- Path derivation: `Executor.CreateWorktree` (`backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:472-482`) always derives the path as `repoPath + "-" + sanitizeBranchForPath(branch)` and runs `git worktree add <targetPath> -b <branch> <baseRef>` — this incidentally satisfies BR-WT-02 (always a sibling of the repo dir, never inside `.git`) by construction, but only because the caller can never choose a different path.
- Tests confirm the happy path and the compensation path but no validation path: `create_worktree_test.go` has `TestCreateWorktree_HappyPath`, `_ThreadsLineageThroughToRecordWorktreeCreated`, `_BookkeepingFails_CompensatesByRemovingWorktree`, `_BookkeepingFailsAndCompensationFails_ReportsBothFailures`, `_GitCreateFails_NoBookkeepingOrCompensationAttempted` (asserts only a generic `WORKTREE_CREATE_FAILED` even when the underlying git error text is `"branch exists"`), `_RepoNotFound_NoExecutorCallAtAll`.

## What's missing

- **No custom name/path input at all**: `CreateWorktreeRequest` has no `name` or `path` field — the spec's own Input contract (`name?: string`, `path?: string`) cannot be satisfied; the worktree name is always the branch name, sanitized.
- **BR-WT-01** (worktree name charset `a-z0-9-_` only): no validation anywhere in `CreateWorktree.Execute` or `domain` — `grep` for a charset check in `create_worktree.go`/`domain.go` finds none; any branch name is passed straight to `git worktree add -b <branch>`, so git's own branch-name rules are the only gate.
- **BR-WT-03 / [A1]** (duplicate path, "Path already exists" + alternate-name suggestion): no pre-check before running `git worktree add`; `TestCreateWorktree_GitCreateFails_NoBookkeepingOrCompensationAttempted` confirms a `"branch exists"` git failure is wrapped into the same generic `WORKTREE_CREATE_FAILED` code as any other failure — no distinct error code/payload for "already exists", and no alternate-name suggestion logic exists anywhere in the package.
- **BR-WT-04** (max 20 worktrees per repo): no count check anywhere — `grep -rn "20 worktree\|MaxWorktree" backend-go/services/git-gateway-service backend-go/services/project-service` returns nothing.
- **[A2]** (base branch not found → friendly error + list of available branches): `CreateWorktree.Execute` has no branch-existence check and no call to a "list branches" usecase before attempting the git operation; failure surfaces only as the generic `WORKTREE_CREATE_FAILED`.
- **[A3]** (disk-space precondition/warning): no disk-space check anywhere in `create_worktree.go` or `localgit/executor.go`.
- **Postcondition "Terminal PTY khởi tạo trong worktree"**: `CreateWorktree.Execute` has no call to any terminal/PTY service — confirmed by `grep -rln "worktree" backend-go/services/*/internal/usecase/*.go` matching only `git-gateway-service` and `project-service`, no pty/terminal service. Terminal startup is entirely outside this saga (may be a legitimate frontend/agent-orchestration responsibility, but the spec states it as part of this flow's postcondition and nothing in backend-go ties worktree creation to it).
- **Event `worktree:created`**: no explicit domain-event/notification publish found in `create_worktree.go` (the saga returns a plain result; no `notification-service` or WS push call is made from the usecase itself — any server-push would have to come from the wscompat layer separately, unverified here).

## See also

- `specs/backend-go/bugs/missing-v1/BUG-031-worktree-channels-not-implemented.md` — describes the pre-wiring state; now stale for `worktree.create`/`worktree.rm`/`worktree.list`/`worktree.set`/`worktree.detectedList`/`worktree.forceDeleteBranch`/`worktree.prefetchCreateBase`/`worktree.resolvePrBase`/`worktree.resolveMrBase`, all of which are now registered per `channels.go:119` and `channels_worktree.go`. Cite this bug for history, not as an open gap.

## References

- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:11-71`
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go:1-149`
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:472-482`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `CreateWorktreeRequest`/`CreateWorktreeResponse` messages
- `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go:678-695`
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go:28-47`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:37-73`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:119`
- `docs/logic/worktree-management/BL-WT-01-tao-worktree.md`
