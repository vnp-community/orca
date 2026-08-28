# BUG-WT-03: Worktree deletion has no backend safety checks — no uncommitted-changes/agent-running/lock checks, no agent-kill or terminal-close step

**Business Logic:** [BL-WT-03](../../../../docs/logic/worktree-management/BL-WT-03-xoa-worktree.md) — Xóa Worktree An Toàn
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** `worktree.rm` genuinely removes the on-disk worktree (`git worktree remove [--force]`) and hard-deletes the Postgres bookkeeping row — that much works. But none of the spec's "safety checks before showing the confirm dialog" exist server-side: there is no backend step that inspects uncommitted/untracked changes, checks whether an agent is running in that worktree, or reports which process holds a lock on the directory. The only protection a caller gets for free is git's own built-in refusal to `worktree remove` a dirty tree when `force=false` — everything else (agent kill, terminal-session close, structured "X files uncommitted" warning, "processes holding a lock" list) is entirely unimplemented, meaning a client that blindly calls `worktree.rm(force=true)` can destroy uncommitted work and orphan a running agent process with no backend guardrail at all.

---

## Spec summary

`BL-WT-03` requires the system to run 4 safety checks (uncommitted changes, untracked files, agent-running status, other-process directory locks) before the delete confirmation dialog, then on confirm: kill any running agent, close terminal sessions, run `git worktree remove --force`, delete the DB record, and update the sidebar — atomically, with 3 recovery choices when changes are uncommitted (Discard & Delete / Commit First / Cancel) and 2 when an agent is running (Stop & Delete / Cancel).

## What backend-go has

- Real RPC + saga: `RemoveWorktree.Execute` (`backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:33-45`) dispatches to the owning host's `GitExecutor.RemoveWorktree(ctx, repoPath, force)` then calls `ProjectClient.RecordWorktreeRemoved`.
- Real git op: `Executor.RemoveWorktree` (`backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:487-495`) runs `git worktree remove [--force] <path>` — git's own refusal to remove a dirty worktree without `--force` is the only enforcement of "don't delete uncommitted work" that exists anywhere in this path; it is not a backend-go safety check, it is git's default behavior, and it is bypassed entirely whenever the caller passes `force=true` (which `RemoveWorktreeRequest.force` lets any caller do unconditionally — `gitgateway.proto`'s `RemoveWorktreeRequest`).
- Real hard-delete bookkeeping: `WorktreeRepository.RecordWorktreeRemoved` (`backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go:52-61`) — `DELETE FROM project.worktrees WHERE id = $1`, satisfying BR-WT-12 (DB row removed alongside filesystem).
- Real WS wiring: `worktree.rm` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:80-92`) is a direct 1:1 passthrough to `gitClient.RemoveWorktree` with a client-supplied `force` bool — no additional server-side logic between the WS call and the gRPC call.
- Terminal state semantics: `RemoveWorktree.Execute`'s own doc comment (`remove_worktree.go:9-14`) explicitly states the design choice — "on-disk gone, bookkeeping stale is a safe terminal state" — which addresses BR-WT-11 (no half-deleted state) for the git+DB pair specifically, but says nothing about agent/terminal state.

## What's missing

- **Pre-delete safety checks (spec step 2, all 4 sub-checks)**: `RemoveWorktree.Execute` calls no `GetStatus`-equivalent check before removing — no uncommitted-changes check, no untracked-files check, no agent-running check, no directory-lock/process check. `grep`-confirmed: `remove_worktree.go` contains exactly 3 calls (`dispatchExecutor`, `executor.RemoveWorktree`, `uc.projects.RecordWorktreeRemoved`) and nothing else.
- **BR-WT-09** ("never delete a worktree with uncommitted changes without explicit confirmation"): not enforced by backend-go logic — only git's default dirty-worktree refusal exists, and it is fully bypassed by `force=true`, which the RPC accepts unconditionally with no separate "did the caller actually confirm" signal.
- **BR-WT-10** ("never delete while agent status == running"): entirely unenforced — no cross-service call to any agent/orchestration state exists in `remove_worktree.go`, and no such check exists anywhere else in the removal path (`grep -rln "RecordWorktreeRemoved\|WorktreeRemoved" backend-go/services/*/internal/**/*.go` shows only `project-service` and `git-gateway-service` touch this flow — no `orchestration-service` or task/agent-state service is consulted).
- **[A1]/[A2] structured recovery payloads**: no backend response shape carries "X files uncommitted" counts, a discard-and-delete vs. commit-first vs. cancel option set, or an agent-running warning — `RemoveWorktreeResponse` is `google.protobuf.Empty` (`gitgateway.proto`'s `RemoveWorktree` RPC signature), so there is no data for a client to build these dialogs from even if it wanted to.
- **[A3]** (list of processes holding a lock on the directory + force-delete option): no such lookup exists anywhere in `git-gateway-service`.
- **Kill agent process / close terminal sessions (spec step 5a/5b)**: no call to any pty/terminal/agent-lifecycle service from `RemoveWorktree.Execute` — confirmed by the same absence noted in BUG-WT-01 (no non-`project-service`/`git-gateway-service` package references `worktree` removal at all).
- **Atomicity across the full described scope (BR-WT-11)**: the git+DB pair is handled as a safe terminal state per the doc comment above, but because agent-kill/terminal-close aren't part of the saga at all, a delete can complete with a still-running orphaned agent process pointed at a now-nonexistent working directory — a genuine half-deleted state at the process level, just not at the git/DB level.

## See also

- None found — `specs/backend-go/bugs/missing-v1/` and `api-v1/` document `worktree.rm`'s channel-wiring gap (now resolved) but do not analyze the missing pre-delete safety-check business logic; this is a business-logic gap, not a wiring gap.

## References

- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:1-45`
- `backend-go/services/git-gateway-service/internal/adapter/localgit/executor.go:487-495`
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `RemoveWorktreeRequest`/RPC signature (`returns (google.protobuf.Empty)`)
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go:52-61`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:80-92`
- `docs/logic/worktree-management/BL-WT-03-xoa-worktree.md`
