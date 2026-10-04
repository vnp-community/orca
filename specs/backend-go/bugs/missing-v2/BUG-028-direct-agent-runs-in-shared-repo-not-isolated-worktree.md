# BUG-028: `task.execute`'s direct_agent path runs the agent in the SHARED base repo checkout, not the task's own isolated worktree — and the dev server's live connection is currently unstable

**Service:** `task-service` (root cause), observed via `git-gateway-service` (symptom) and a live infra instability on the dev server itself
**Severity:** 🔴 Critical — every direct_agent task that reaches a real agent dispatch writes/commits into a repo checkout shared across ALL tasks and ALL other work on that repo, on whatever branch happens to be checked out there (confirmed: `main`, not the task's own branch) — not a "small precision gap," a correctness and safety defect.
**Status:** ✅ Fixed, deployed (`2026.09.16-bug028-worktree-path-fix`), and **live-verified** — a fresh `grpcurl` spec-generation dispatch against a task with an existing worktree (the exact reuse case that was broken) landed a real commit (`c923636`) on the task's own `task/fa558891-...` branch, inside its own isolated worktree directory — confirmed via direct SSH to `172.20.2.41`. The shared repo root (`/opt/repos/aiops-v3`) was untouched (still on `main`, still at the prior bug's stray commit `00fc2ca`). One retry along the way failed on the live infra flapping issue in §4 (`INFRA_DEV_SERVER_NOT_CONNECTED`, unrelated to this fix) — not counted against the fix itself; the very next retry, once the dev server reported healthy again, succeeded cleanly.

This document consolidates the entire investigation. Sections 1–2 are **already fixed and deployed** (context only, not re-litigated in detail — see the linked docs). Section 3 is the new, currently-unfixed root cause. Section 4 is a live infra issue found along the way, not a code bug. Section 5 is still open and needs the user's help, not more guessing.

---

## 1. Already fixed and deployed — the dispatch pipeline itself now works

In order, each one blocking the next from ever being observed until fixed:

| # | Bug | Symptom | Fix | Doc |
|---|-----|---------|-----|-----|
| 1 | `TaskDetail` never mounted | Task Detail panel always empty | Wired `<TaskDetail>` into `WorkspaceLayout` | (this session, no separate doc) |
| 2 | `parentId`/`TaskStatus 'open'`/`worktreePath` crash | Tree/Board hid tasks, crash on Run | `*string` for null parentId, added `'open'` status, removed dead `worktreePath` param | [BUG-025](./BUG-025-task-graph-tree-board-invisible-and-execute-crash.md) |
| 3 | `infra.connections` structural gap, tenant/user metadata not forwarded, wrong RPC for the fallback path | `TASK_EXECUTE_NO_CONNECTION`, `TASK_EXECUTE_WORKTREE_FAILED`, `INFRA_RELAY_NO_CONNECTION` | Reachability-based fallback (mirrors git-gateway-service), fixed 3 separate call sites missing user-id/tenant-id forwarding, switched to `RelayByDevServer` for the no-connection-row case | [BUG-026](./BUG-026-task-execute-no-connection-structural-gap.md) (5 bites) |
| 4 | `task.execute` blocked the client RPC for the whole agent run; concurrent re-dispatch; 30s internal timeout; no unattended-write trust | `DeadlineExceeded`, silent no-result runs, task stuck `in_progress` forever, agent refusing to write files | Async redesign, already-in-progress guard, `agent.execPrompt`-specific 15min timeout, `trustPreset: "full"` | [BUG-027](./BUG-027-task-execute-direct-agent-blocks-rpc-timeout.md) (4 bites) |

**Verified live, end to end, bypassing the frontend entirely** (via `grpcurl` directly against `task-service:9090`, to isolate backend correctness from the still-unexplained frontend click issue in §5): a real spec-generation prompt dispatched, the agent actually wrote a 184-line spec file and committed it with the exact requested message. The pipeline built across BUG-025/026/027 **works**. The problem now is **where** it wrote that file.

## 2. What this section is NOT about

`ExecuteBatch`'s own dependency-wave-gating regression (flagged in BUG-027 as a known side effect) is confirmed dead code (no production caller) and is not part of this investigation.

## 3. NEW, unfixed: direct_agent writes into the shared repo root, not the task's isolated worktree

### Evidence (from the real dev server, `172.20.2.41`, not inferred)

Task `fa558891-...` already had an existing worktree (`ba591d24-...` at `/opt/repos/aiops-v3-task-fa558891-69c2-4096-b99e-b21084276f75`, branch `task/fa558891-...`, created by an earlier dispatch). A fresh dispatch asked the agent to write `specs/generated/TASK-1-spec.md` and commit it. The agent reported success, and it was telling the truth — just not about *where*:

```
$ ssh 172.20.2.41
$ cd /opt/repos/aiops-v3-task-fa558891-69c2-4096-b99e-b21084276f75   # the task's OWN worktree
$ git log --oneline -5
0a6216d fix(cmdb-go): ...   # pre-existing commits only — nothing new
$ git status
nothing to commit, working tree clean
$ find specs
find: 'specs': No such file or directory

$ cd /opt/repos/aiops-v3                                             # the SHARED base repo checkout
$ git log --oneline -3
00fc2ca docs(task-1): add spec for test task     # <-- here
0a6216d fix(cmdb-go): ...
$ git branch --show-current
main                                                                  # <-- not task/fa558891-...
$ git status
... ~280 pre-existing modified files, untracked files ...             # someone else's / other work in flight
$ find /opt/repos -iname 'TASK-1-spec.md'
/opt/repos/aiops-v3/specs/generated/TASK-1-spec.md                    # only one copy, in the wrong repo
```

The commit exists (confirmed via `git log --all` from any of the three sibling checkouts under `/opt/repos/`, since worktrees of the same repo share one object database) — it is real, it has the right message, right author trailer, right diff. It is simply sitting on `main` in the repo **every task and every other piece of work on this project shares**, not on the task's own branch in its own isolated directory.

### Root cause

`SimpleExecutor.Execute` (`task-service/internal/adapter/grpcclient/simple_executor.go`) resolves the `worktreePath` it sends to `agent.execPrompt` via its own call to `ProjectExecutionResolver.ResolveConnection`. That resolver's fallback path (`resolveViaDefaultRepo`, added by BUG-026 fix #1 — see that doc) returns **the repo's own root path** (`repo.GetUrl()`, e.g. `/opt/repos/aiops-v3`) whenever there's no real `infra.connections` row — which, per BUG-026's own finding, is the system-wide norm, not an edge case. It was never designed to know about a *specific task's* worktree at all; it only resolves "this project's default repo," which is correct for `WorktreeProvisioner.EnsureWorktree`'s **create-new-worktree** path (where the repo root is what `CreateWorktree` needs to branch *from*), but wrong as the actual execution `cwd` once a worktree already exists.

`ExecuteTask.Execute`'s own pre-check already surfaces this gap explicitly in its own comment (`execute_task.go`):

```go
if worktreePath == "" {
    worktreePath = resolvedPath // reuse branch: EnsureWorktree returns "" for path on reuse — see WorktreeProvisioner's doc comment
}
_ = worktreePath // resolved for parity with SOL-TG-04's design; SimpleExecutor/ComplexExecutor resolve their own worktree path today
```

That computed value is **discarded** (`_ = worktreePath`) — `SimpleExecutor.Execute` never receives it and re-resolves its own copy independently, hitting the exact same fallback, with the exact same repo-root answer. And `WorktreeProvisioner.EnsureWorktree`'s reuse branch (`worktree_provisioner.go`) has always returned an empty path on reuse:

```go
if task.WorktreeID != "" {
    return task.WorktreeID, "", nil // reuse — Caller resolves the path separately via ProjectExecutionResolver, unchanged from today.
}
```

That "caller resolves the path separately" assumption was true for a design where `ProjectExecutionResolver` was expected to resolve a *worktree-specific* path (e.g. via a real `infra.connections` row keyed by worktree). It has never been true for the fallback path, which only ever knows about the *repo*. BUG-026's own doc flagged this exact gap as a known, deliberately-deferred imprecision ("structurally unreachable to even observe until this bug's primary gate was fixed") — this is that gap, now observed for real, and it is worse than "imprecision": for any task **reusing** an existing worktree, direct_agent has no code path that ever resolves that worktree's real, isolated directory. It always falls back to the shared repo root.

### Fix plan — fully specified, code-level, ready to implement

**Scope check done first, not assumed:** `WorktreeProvisioner.EnsureWorktree`'s CREATE branch (a task with no existing `WorktreeID`) is already correct — confirmed via `project.worktrees` directly for this exact task's own first-ever dispatch: `path = /opt/repos/aiops-v3-task-fa558891-...`, a real isolated directory, because `CreateWorktree`'s real gRPC response already carries the right `path` and that branch returns it verbatim (`worktree_provisioner.go`: `return resp.GetWorktreeId(), resp.GetPath(), nil`). **Only the REUSE branch is broken** (`if task.WorktreeID != "" { return task.WorktreeID, "", nil }` — empty path, always). Every dispatch after a task's first one hits this branch, which is why the bug wasn't caught until a task was re-executed. This narrows the fix to exactly two files, no broader redesign needed.

**Why `ComplexExecutor` doesn't have this bug, and what to mirror from it:** `ComplexExecutor.Execute` already takes `worktreeID` as an explicit 4th parameter (`ports.go:291`, "threaded through so orchestration-service's coordinator_run knows which worktree its dispatched work runs in") — `SimpleExecutor.Execute` is the one port that DOESN'T receive it, forcing it to re-derive worktree info independently through `ProjectExecutionResolver`, which structurally only knows about *repos*, never a specific *worktree*. The fix makes `SimpleExecutor` symmetric with the sibling port that already gets this right, not a new pattern.

**Four changes, in dependency order:**

1. **`services/task-service/internal/adapter/grpcclient/worktree_provisioner.go`** — fix the reuse branch to resolve the real path via project-service's `GetWorktree` (exact same tenant+user-forwarding pattern this file's own `resolveRepoID` already established, reused verbatim — not reinvented):
   ```go
   func (p *WorktreeProvisioner) EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error) {
       if task.WorktreeID != "" {
           path, err := p.resolveWorktreePath(ctx, task.WorktreeID)
           if err != nil {
               return "", "", err // fail closed — see this method's doc comment for why a silent repo-root fallback here would just reintroduce BUG-028
           }
           return task.WorktreeID, path, nil
       }
       // ... unchanged create branch ...
   }

   // resolveWorktreePath resolves an EXISTING worktree's real, isolated
   // filesystem path via project-service's GetWorktree — the same RPC
   // git-gateway-service's own ConnectionResolver.resolveLocal already
   // calls for the identical purpose. Added for BUG-028: this method's
   // reuse branch used to return an empty path, silently relying on
   // callers (SimpleExecutor.Execute) to fall back to a resolver that only
   // knows about the project's REPO, never a specific worktree — every
   // task re-execution ran the agent in the shared repo root instead of
   // this task's own isolated directory. Errors here MUST propagate (not
   // degrade to a repo-root guess) — a wrong-but-quiet path is exactly
   // this bug, not an acceptable fallback.
   func (p *WorktreeProvisioner) resolveWorktreePath(ctx context.Context, worktreeID string) (string, error) {
       tenantID, err := tenant.RequireTenantID(ctx)
       if err != nil {
           return "", err
       }
       userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome — see resolveRepoID's identical comment
       projectCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)
       resp, err := p.projects.GetWorktree(projectCtx, &projectv1.GetWorktreeRequest{WorktreeId: worktreeID})
       if err != nil {
           return "", fmt.Errorf("worktree_provisioner: get worktree %q: %w", worktreeID, err)
       }
       return resp.GetPath(), nil
   }
   ```
   (`tenant`, `metadata`, `grpcmw` are already imported in this file for `resolveRepoID`'s identical pattern — no new imports.)

2. **`services/task-service/internal/usecase/ports.go`** — update `WorktreeProvisioner`'s doc comment (the "callers resolve the reuse-branch path separately via ProjectExecutionResolver" line is the exact wrong assumption this bug is fixing — delete it) and thread `worktreePath` into `SimpleExecutor`:
   ```go
   type SimpleExecutor interface {
       Execute(ctx context.Context, tenantID, taskID, requestID, worktreePath, prompt string) (executionRef string, err error)
   }
   ```

3. **`services/task-service/internal/adapter/grpcclient/simple_executor.go`** — `Execute` gains the `worktreePath` parameter and uses it directly instead of re-resolving one:
   ```go
   func (s *SimpleExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreePath, prompt string) (string, error) {
       // ... unchanged: load task, ancestors, completedDeps ...

       connectionID, _, _, devServerID, connected, err := s.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
       // ^ 2nd return value (the resolver's own worktreePath guess) is now
       // deliberately discarded with `_` — see this method's own doc
       // comment: ResolveConnection only ever knows about the project's
       // REPO, never a specific worktree, so its path guess is wrong for
       // any task reusing an existing worktree (BUG-028). connectionID/
       // devServerID/connected are still exactly what this call is for —
       // unchanged from BUG-026's fix.
       if err != nil { ... }        // unchanged
       if !connected { ... }        // unchanged
       if worktreePath == "" {      // unchanged check, now checking the PARAMETER instead of the resolver's guess
           return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_WORKTREE_PATH", "task's connected dev server has no worktree path recorded", nil)
       }
       // ... rest of the method (buildExecutePrompt, params, profile
       // resolution, relay dispatch) already uses the local `worktreePath`
       // variable verbatim — no further change needed there, since it's
       // now the parameter instead of a resolver-derived local.
   }
   ```

4. **`services/task-service/internal/usecase/execute_task.go`** — stop discarding the already-correctly-resolved path and thread it through both the direct-dispatch call site and `dispatchDirectAgentAsync`:
   ```go
   // Remove: _ = worktreePath // resolved for parity with SOL-TG-04's design; ...
   // (the comment's own premise — "SimpleExecutor resolves its own worktree path today" — is exactly what this bug fixes)

   if engine == domain.EngineDirectAgent {
       return uc.dispatchDirectAgentAsync(ctx, tenantID, userID, in, task, worktreePath, link.ID, previousStatus, dispatchStart)
   }
   ```
   ```go
   func (uc *ExecuteTask) dispatchDirectAgentAsync(ctx context.Context, tenantID, userID string, in ExecuteTaskInput, task domain.Task, worktreePath, linkID string, previousStatus domain.Status, dispatchStart time.Time) (ExecuteResult, error) {
       // ... dispatchCtx construction unchanged ...
       uc.runAsync(func() {
           ref, err := uc.simple.Execute(dispatchCtx, tenantID, in.TaskID, in.RequestID, worktreePath, in.Prompt)
           // ... rest unchanged ...
       })
       return ExecuteResult{Async: true}, nil
   }
   ```
   Note: `worktreePath` here is the value from `if worktreePath == "" { worktreePath = resolvedPath }` (the existing reuse-branch-empty-path fallback in `Execute`) — after fix #1, `EnsureWorktree` never returns an empty path on success (it fails closed instead), so this fallback becomes dead code for the success path and only fires if `EnsureWorktree` itself somehow returned `("", "", nil)` (a contract violation it no longer produces) — left in place as a harmless defensive no-op, not removed, since removing it is a separate, unrelated cleanup.

**Blast radius, checked, not assumed:** `SimpleExecutor` has exactly one production caller (`ExecuteTask.dispatchDirectAgentAsync`, itself only reachable via `ExecuteTask.Execute`) and one test file (`simple_executor_test.go`) — grepped directly, not guessed. Every existing test in that file constructs its own `agentExecPromptParams`/calls `exec.Execute(...)` with a hardcoded worktreePath argument already (`fakeProjectExecutionResolver.worktreePath`) — updating call sites to pass that same string as the new explicit parameter (instead of relying on the fake resolver to echo it back) is mechanical, not a design change to any test's intent.

**Testing plan for the fix:**
- `TestWorktreeProvisioner_ReuseExistingWorktree_ResolvesRealPathViaGetWorktree` (new) — reuse branch with `task.WorktreeID` set returns the path from a fake `GetWorktree` response, not empty.
- `TestWorktreeProvisioner_ReuseExistingWorktree_GetWorktreeError_FailsClosed` (new) — a `GetWorktree` error propagates as `EnsureWorktree`'s own error, never degrades to a guessed path.
- Existing `simple_executor_test.go` suite: every test updated to pass an explicit `worktreePath` argument (already have one available per-test via `fakeProjectExecutionResolver.worktreePath`, just moved from an implicit resolver return to an explicit call argument) — behavior-preserving for all of them except the two new ones below.
- `TestSimpleExecutor_Execute_UsesPassedWorktreePath_NotResolverGuess` (new) — the actual regression: resolver configured with ONE `worktreePath` ("wrong/repo-root"), `Execute` called with a DIFFERENT explicit `worktreePath` ("right/isolated/worktree") — asserts the dispatched `agent.execPrompt` params carry the explicit one, proving the resolver's guess is genuinely ignored, not just usually-the-same-value.
- `TestExecuteTask_DirectAgentDispatch_PassesRealWorktreePathOnReuse` (new, `execute_task_test.go`) — a task with an existing `WorktreeID` dispatches with the fake `WorktreeProvisioner`'s reuse-resolved path threaded all the way to the fake `SimpleExecutor`, not the fake `ProjectExecutionResolver`'s repo-level path.
- Live re-verification identical to this bug's own discovery method: `grpcurl` dispatch of a spec-generation prompt against a task that ALREADY has a worktree (the exact reuse case that was broken), then SSH to the dev server and confirm the commit lands in the task's own worktree directory, on the task's own branch — not `/opt/repos/aiops-v3` on `main`.

## 4. Live infra instability — NOT a code bug from this session, flagged for the dev server operator

While investigating §3, the dev server's live agent connection was observed flapping — repeated, real failures unrelated to any of the code touched in BUG-025/026/027/028:

```
GITGATEWAY_STATUS_FAILED: grpcclient: relayByDevServer git.status: ... INFRA_DEV_SERVER_NOT_CONNECTED: this dev server has no live agent connection right now
WORKTREE_DETECT_FAILED: git worktree list --porcelain: chdir /opt/repos/aiops-v3: no such file or directory
GITGATEWAY_CHECK_HOOKS_FAILED: read hooks dir: open /opt/repos/aiops-v3/.git/hooks: no such file or directory
ReadDir: open /opt/repos/aiops-v3-golang-production-ready: no such file or directory
```

These alternate, second to second, with the exact same paths succeeding moments later — not a permanent missing directory (confirmed present via direct SSH), but a **flapping live connection** between infra-fleet-service and the Dev Server Agent process on `172.20.2.41`. This matches the earlier "dev server disconnected" event seen in this same investigation (BUG-026's timeline) and is consistent with the agent process itself being unstable under load — plausibly aggravated by the repeated manual `grpcurl` dispatches used to verify BUG-027's fixes in this same session, but not conclusively caused by them.

**Not a task-service/git-gateway-service code defect** — nothing in this session's changes touches the Dev Server Agent process or its connection lifecycle. Recommended next step, outside this codebase: check the Dev Server Agent process's own health/logs on `172.20.2.41` directly (process restarts, memory pressure, network blips between it and the `b15.openledger.vn` backend host).

## 5. Still open — the frontend "Run with Agent" click producing zero backend traffic

Reported separately, still unresolved: clicking "Generate Spec" then "Run with Agent" in the task's AI tab produced **no RPC at all** at api-gateway or task-service for a sustained window (only routine `task.get` polling). Every hypothesis reachable from server-side logs was reviewed (stale `isTaskRunning`/`isDispatching` guard state from BUG-027's second bite) and could not be confirmed or ruled out without the browser's own DevTools output. **Still needs**: Console tab (any red error) and Network tab (was a `runtime`/`task.execute` request even attempted) from an actual repro. This is not guessed at further here.

## What this doc is asking for

§3's fix is done, deployed, and live-verified — BL-TG-06's prerequisite is satisfied. Still open:
1. What to do with the stray `00fc2ca` commit sitting on `main` in the shared `/opt/repos/aiops-v3` checkout (asked twice, not yet answered): revert it, or leave it.
2. Whether `172.20.2.41`'s Dev Server Agent process is something the user can check/restart directly, since §4 (live connection flapping) is outside what this codebase's changes can fix — it caused one retry to fail during this very fix's own verification, resolving on its own moments later.
3. The DevTools Console/Network evidence for §5 (frontend click producing no backend traffic), whenever the user can grab it.

## Related

- [BUG-025](./BUG-025-task-graph-tree-board-invisible-and-execute-crash.md), [BUG-026](./BUG-026-task-execute-no-connection-structural-gap.md), [BUG-027](./BUG-027-task-execute-direct-agent-blocks-rpc-timeout.md) — the full chain that had to be fixed, in order, before this bug's failure mode was even reachable.
