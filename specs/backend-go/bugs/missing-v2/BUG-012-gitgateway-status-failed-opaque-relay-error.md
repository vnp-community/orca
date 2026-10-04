# BUG-012: `GITGATEWAY_STATUS_FAILED` — opaque generic error, real cause unknown because `git-gateway-service` has no cause-logging wired

**Service:** `git-gateway-service`
**File:** `internal/usecase/get_status.go` (`GetStatus.Execute`, where the generic error is raised); real cause is somewhere in `internal/adapter/grpcclient/relay_executor.go`'s `GetStatus` → `agent-git-handler-local-ops.ts`'s `handleGitStatus`/`getStatusOp` chain
**Severity:** Medium — blocks the Git panel from loading status for an otherwise-valid, reachable worktree; frontend surfaces it as an unhandled promise rejection (`GitPanel-*.js`), no user-facing explanation
**Symptom:**
```
Unhandled Promise Rejection: RuntimeRpcCallError: rpc error: code = Internal desc = GITGATEWAY_STATUS_FAILED: failed to get git status
```
**Status:** ✅ **Root cause CONFIRMED (2026-09-15)**, thanks to SOL-012's logging exposing it on the first real recurrence — exact same playbook that worked for BUG-009. ✅ **Blast radius CONFIRMED CRITICAL** (34 direct callers via `dispatchExecutor`, see full audit below). ✅ [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md) deployed + live-verified: the path-echo symptom this bug originally reported is fixed. 🟡 **Reopened as a narrower bug**: a second, distinct issue ([BUG-015](./BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md)) was found immediately during live verification — `dispatchExecutor` still can't reach a dev-server-hosted worktree (wrong executor selected, not wrong path anymore). This bug (BUG-012) is closed for its original symptom; BUG-015 tracks the remainder.

---

## What's confirmed

Live logs, same `trace_id` on both services, for the failing call:

```json
// git-gateway-service — the visible error
{"time":"2026-09-15T07:40:46.721...","level":"ERROR","msg":"rpc failed","method":"/orca.gitgateway.v1.GitGatewayService/GetStatus","error":"rpc error: code = Internal desc = GITGATEWAY_STATUS_FAILED: failed to get git status","trace_id":"115ddae03c75bcfaa902103971447518"}

// infra-fleet-service — the SAME trace_id's RelayByDevServer call, SUCCEEDED
{"time":"2026-09-15T07:40:46.188...","level":"INFO","msg":"rpc ok","method":"/orca.infrafleet.v1.InfraFleetService/RelayByDevServer","duration":5376857,"trace_id":"115ddae03c75bcfaa902103971447518"}
```

**The relay call itself succeeded** (`rpc ok`, no `apperrors: internal cause` line at all for this trace) — meaning the dev server agent responded, transport worked, and the underlying WS round-trip completed. The failure therefore happens strictly inside `git-gateway-service`'s own handling of that response — either:
- the agent's `git.status` handler (`agent-git-handler-local-ops.ts`'s `handleGitStatus` → `getStatusOp`) returned a JSON-RPC `error` object (a real git/parse failure on the agent side, caught by `handleGitStatus`'s generic `catch` and turned into `AgentErrorCode.ServerError`), or
- `RelayExecutor.GetStatus` (`relay_executor.go:253`) got a `result` back but failed to unmarshal it into `domain.GitStatus` (a wire-shape mismatch, same class of bug as several already-fixed `missing-v2` entries — BUG-001/005/006's "shared boundary contract" theme).

Manually confirmed the worktree itself is completely healthy — `git status` run directly via SSH on the affected path (`/opt/repos/aiops-v3-golang-production-ready` on `test-01`) returns cleanly:
```
On branch golang-production-ready
nothing to commit, working tree clean
```
So this is **not** a real git-level problem with the worktree — the failure is purely in the relay/parsing layer between git-gateway-service and the agent.

## Why the cause isn't visible yet

`apperrors.SetLogger` (SOL-009) is wired **only in `infra-fleet-service`'s `cmd/server/main.go`** — confirmed via `grep -rn "apperrors.SetLogger" backend-go/services/*/cmd/server/main.go`, exactly one hit. `git-gateway-service` (where `GITGATEWAY_STATUS_FAILED`'s `apperrors.New(..., err)` actually wraps the real cause) has no logger wired at all, so that wrapped `err` — the one piece of information that would tell us exactly what failed — is silently discarded before it ever reaches a log line, docker or otherwise. This is the exact gap SOL-009's own design doc flagged as a known limitation (additive, opt-in per service) and BUG-010 already showed can bite a second, unrelated service (there it was `project-service`, already fixed by extending the pattern — this is now a third instance of the same missing-observability shape, this time in `git-gateway-service`).

## Root Cause — CONFIRMED (2026-09-15, via SOL-012's live log)

```json
{"code":"GITGATEWAY_STATUS_FAILED","cause":"git status --porcelain=v1 -b: chdir 01579d39-64c0-482f-a0e6-eeb577e404f7: no such file or directory: "}
{"code":"GITGATEWAY_STATUS_FAILED","cause":"git status --porcelain=v1 -b: chdir 45f573e8-be1a-4b1a-894b-a7c261fb7331::/opt/repos/aiops-v3: no such file or directory: "}
```

Both `chdir` targets are **not filesystem paths** — the first is a raw `project.worktrees.id` UUID, the second is `mergeDetectedWorktrees`'s synthesized `repoId::path` display key for an externally-detected worktree. Both are being handed to `git status` as `cwd` directly.

Traced to `internal/adapter/grpcclient/resolver.go`'s `ConnectionResolver.ResolveConnection`:
```go
if !resp.GetConnected() {
    return usecase.ResolvedConnection{Connected: false, RepoPath: dispatchID}, nil
}
```
When infra-fleet-service reports no active `connections` row for this worktree (its own doc comment already notes: **"No infra.connections row exists for these repos (confirmed live: zero rows, system-wide)"**), this returns the **input worktree ID itself** as `RepoPath` — clearly a placeholder that was never meant to reach `git status`'s `cwd`, but `GetStatus.Execute` (`get_status.go`) passes it straight through via `dispatchExecutor` with no validation.

This is the exact same "`dispatchExecutor`/`ConnectionResolver` never worked" gap `dispatchExecutorForRepo`'s own doc comment (`ports.go:628-655`) already documents — `CreateWorktree`/`DetectWorktrees`/`PrefetchCreateBase`/`ResolvePrBase`/`ResolveMrBase` were already migrated off `dispatchExecutor` onto the working `dispatchExecutorForRepo` (which reads `repo.URL` directly, no `infra.connections` dependency). **`GetStatus` (and likely `GetDiff`/`History`/every other `WorktreeID`-keyed usecase still using `dispatchExecutor`) was never migrated** — this bug is that migration gap's next casualty.

### Fix direction (not yet implemented — moderate scope, needs a project-service lookup)

`GetStatusInput` only carries a bare `WorktreeID`, but `dispatchExecutorForRepo` needs a full `domain.RepoInfo` (repo URL + dev server ID). A real fix needs either:
1. A `ProjectClient` method resolving `WorktreeID → RepoInfo` (project-service already has the worktree's `repo_id`/`path`/`dev_server_id` in `project.worktrees`/`project.repos` — likely just needs a new/reused gRPC call, not new persisted state), then route `GetStatus` (and siblings) through `dispatchExecutorForRepo` like the already-migrated usecases; or
2. Fix `ConnectionResolver.ResolveConnection`'s `!Connected` branch to look up the real path via project-service instead of echoing back the input ID (narrower fix, same effect for `GetStatus` specifically, but doesn't address the other `dispatchExecutor` callers likely sharing this bug).

**✅ Full audit DONE (2026-09-15)**: `gitnexus impact({target: "dispatchExecutor", direction: "upstream"})` → **risk: CRITICAL, impactedCount: 38, direct: 34**. Grep confirms 32 usecase files call `dispatchExecutor` directly (not `dispatchExecutorForRepo`): `abort_rebase`, `abort_merge`, `check_ignored`, `check_worktree_delete_safety`, `checkout`, `conflict_operation`, `branch_compare`, `branch_diff`, `discard`, `merge_worktree_into_base` (x2 call sites), `fork_sync`, `stage`, `get_diff`, `commit_diff`, `list_local_branches`, `resolve_conflict`, `commit_compare`, `bulk_discard`, `remove_worktree`, `fetch`, `commit`, `get_status`, `force_delete_branch`, `history`, `push`, `pull`, `remote_commit_url`, `remote_file_url`, `unstage`, `rebase_from_base`, `fast_forward`, `submodule_status`, `upstream_status`, `compare_worktrees` — i.e. essentially every worktree-scoped git operation in the service. Depth-2/3 indirect callers: `diff_composer.go`'s `gatherFullDiff`/`gatherFullDiffFromStatus`, then `generate_commit_message.go`/`generate_pull_request_fields.go`.

Note: impact run directly on the concrete `ConnectionResolver.ResolveConnection` implementation (`target_uid` pinned to `resolver.go`) returns `direct: 0, risk: LOW` — this is a false negative caused by Go interface dynamic dispatch (the real call site is `uc.resolver.ResolveConnection(...)` through an interface field, which gitnexus's static call graph doesn't resolve to the concrete method). The `dispatchExecutor`-side number above is the trustworthy one, since `dispatchExecutor` is the concrete function that actually invokes the resolver.

This is not a narrow, single-usecase bug — it's a **CRITICAL, system-wide** gap. See [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md) for the fix design (not yet implemented) and [CR-PW-010](../../../../docs/crs/v3/project-workspace/CR-PW-010-git-status-worktree-id-resolver-broken.md) for the CR.

## Fix direction: SOL-012 (mirrors SOL-009 exactly) — already done, is what exposed the above

Wire `apperrors.SetLogger(logger)` into `git-gateway-service/cmd/server/main.go`, identically to how `infra-fleet-service`'s `main.go` does it (SOL-009/TASK-018) — zero behavior change for clients, purely additive server-side logging. Once deployed, the next `GITGATEWAY_STATUS_FAILED` occurrence will log its real wrapped cause, at which point this bug file should be updated with the actual root cause and either closed as a duplicate of an existing bug shape or filed as its own fix, exactly as BUG-009 evolved after SOL-009 exposed its real cause.

**Not doing the fix inline in this pass** — per this session's explicit instruction to log every bug before continuing to code changes. See [SOL-012](./solutions/SOL-012-git-gateway-service-cause-logging.md) for the design (kept separate and minimal, matching SOL-009's own scope discipline).

## Related

- [BUG-009](./BUG-009-infra-agent-exec-failed-generic-relay-error.md) / [SOL-009](./solutions/SOL-009-apperrors-optional-cause-logging.md) — the original observability fix this bug's solution directly extends.
- [BUG-010](./BUG-010-rebind-repo-dev-server-lookup-failed-missing-tenant-metadata.md) — the second service (`project-service`) this exact "logger not wired everywhere" gap already bit; this is the third.
- [BUG-011](./BUG-011-detected-worktrees-merge-disk-first-drops-db-rows.md) — found in the same investigation session, same underlying task (getting the "aiops-v3" worktree fully usable end to end).
