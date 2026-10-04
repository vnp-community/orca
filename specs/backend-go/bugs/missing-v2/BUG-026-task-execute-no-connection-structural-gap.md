# BUG-026: `TASK_EXECUTE_NO_CONNECTION` is a structural gap, not a missing project setting — `ProjectExecutionResolver` never had git-gateway-service's already-proven reachability fallback

**Service:** `task-service`
**Severity:** 🔴 Critical — every `task.execute` call fails this gate before ever attempting to run an agent, for every project, always (not specific to "aiops-v3")
**Status:** ✅ Root cause confirmed via full source trace + live DB query (not guessed — the previous 2 messages in this session incorrectly told the user to "assign a dev server to the project," which turned out to already be done). ✅ Fixed + unit-tested. Deploy pending final verification.

## What was wrong with the earlier "assign a dev server to the project" guidance

That guidance was based on checking `project.dev_server_id`, which was empty — but that column is a **Phase 10 deprecation leftover nothing reads anymore** (confirmed via `frontend/src/renderer/src/components/project/ProjectDevServerSection.tsx`'s own header comment: dev-server ownership moved onto `project.repos.dev_server_id`). Checking the real field:
```
select id, project_id, url, dev_server_id from project.repos where project_id = '<aiops-v3's id>';
 45f573e8-... | 325d2acc-... | /opt/repos/aiops-v3 | a1825a89-...  (= "test-01")
```
**The repo was already correctly bound to a dev server the whole time.** The actual bug was never in the project's configuration.

## Root cause — traced end to end, not inferred

1. `SimpleExecutor.Execute` (`task-service/internal/adapter/grpcclient/simple_executor.go`) calls `ProjectExecutionResolver.ResolveConnection(ctx, tenantID, task.ProjectID)` and fails the whole `Execute` call with `TASK_EXECUTE_NO_CONNECTION` if it reports `connected=false`.
2. The old `ProjectExecutionResolver.ResolveConnection` (`project_execution_resolver.go`) called infra-fleet-service's `ResolveConnection` RPC with `ConnectionId: projectID` — i.e., it asks: "is there a row in `infra.connections` whose primary key literally equals this project's id?"
3. **Confirmed via the real postgres query for `infra.connections`: 0 rows, for any project, in the whole tenant.** This is not specific to "aiops-v3" or a fluke — it's structural:
   - `infra.connections.id` is always `uuid.NewString()`, minted fresh inside `usecase.CreateConnection.Execute` / `usecase.EstablishConnection.Execute` — **neither of these two usecases (the only 2 places that ever insert into this table) accepts a caller-supplied id**, and `CreateConnectionRequest`'s proto has no such field either.
   - Nothing in this codebase has ever inserted a row whose `id` equals a `project.id`. The lookup `ProjectExecutionResolver` was making could never succeed for any project, ever.
4. **git-gateway-service already hit and fixed the exact same shape of bug**, one level down (worktree-keyed, not project-keyed) — BUG-012/CR-PW-010/SOL-013 and BUG-015/CR-PW-011/SOL-014, both from earlier in this session. Its `ConnectionResolver.ResolveConnection` (`git-gateway-service/internal/adapter/grpcclient/resolver.go`) explicitly documents: *"this is the 'no infra.connections row for this worktree' branch, confirmed live to be the system-wide, zero-row norm — not a rare edge case"* — and falls back to resolving the resource's own dev-server binding directly, then checking **live** reachability via infra-fleet-service's `GetFleetHealth`, rather than ever requiring an `infra.connections` row.
5. **`task-service`'s `ProjectExecutionResolver` was never given this same fallback** — it's a sibling resolver (same author intent, same TDD lineage — its own doc comment says it "mirrors git-gateway-service's ConnectionResolver exactly") that simply never got the SOL-013/SOL-014 fix applied to it.

## Fix

`ProjectExecutionResolver.ResolveConnection`, on `!resp.GetConnected()`, now calls a new `resolveViaDefaultRepo`:
1. `project-service.ListRepos(projectId)` → take `repos[0]` (lowest `position` — the exact same "project's default repo" convention `WorktreeProvisioner.resolveRepoID` already established for this same service, not a new one invented here).
2. If that repo has a `DevServerId`, check `infra-fleet-service.GetFleetHealth` for a live-reachable sample for it (new `DevServerReachability` adapter, a line-for-line port of git-gateway-service's own `adapter/grpcclient/reachability.go`).
3. Reachable → `connected=true`, `worktreePath` = the repo's own root path (`Repo.Url`, which — per `WorktreeProvisioner`'s own resolveRepoID doc comment — doubles as an absolute filesystem path for these repos). Not reachable, no dev server bound, or no repos at all → `connected=false`, same as before (never a hard error — `ExecuteTask` already turns `!connected` into the user-facing `TASK_EXECUTE_NO_CONNECTION`, unchanged).

**Known, documented imprecision not fixed here:** for a task **reusing** an existing worktree (not this fallback's first-execution case), `resolvedPath` from this fallback is the repo's root, not that specific worktree's own subfolder — `ExecuteTask.Execute` only falls back to `resolvedPath` when `WorktreeProvisioner.EnsureWorktree`'s own reuse branch returns an empty path, which was already a pre-existing, separate, smaller precision gap unrelated to whether this bug's fallback exists at all — flagged, not silently ignored, not fixed in this pass since **it was structurally unreachable to even observe until this bug's primary gate was fixed** (every task's first execution failed before ever getting there).

## Second bite, found only after adding server-side diagnostics — `apperrors.New(..., err)`'s cause never reaches the client

The first deploy of the fallback above **still failed with the identical client-visible error**, live-confirmed by the user retrying twice. `execute_task.go`'s `apperrors.New(apperrors.KindFailedPrecondition, "TASK_EXECUTE_NO_CONNECTION", ..., err)` DOES wrap the real `err` — but `apperrors.ToGRPCStatus` only forwards Kind/Code/Message to the gRPC status the client receives, never the wrapped cause, and the "rpc failed" log line logs that same client-facing status string, not the underlying Go error chain. **The real cause was invisible in every log this bug's first pass checked.**

Added a one-line `slog.WarnContext` logging `resolver_error` explicitly right before that return (a permanent, real fix for the next time any resolver failure needs diagnosing, not a throwaway hack), redeployed, asked the user to retry once more, and read:
```
resolver_error: grpcclient: ListRepos("325d2acc-..."): rpc error: code = Unauthenticated desc = PROJECT_NO_USER: no user in request context
```

**Root cause #2:** `resolveViaDefaultRepo`'s call to `project-service.ListRepos` was using the SAME `ctx` `withTenantMetadata` had already stamped for the `infra-fleet-service` call earlier in `ResolveConnection` — but `withTenantMetadata`'s own doc comment says it is **"scoped to infra-fleet-service calls only"**. `project-service.ListRepos` is membership-gated (`requireProjectAccess`) and needs BOTH tenant AND acting-user identity forwarded — exactly the same requirement `ProjectContextResolver.GetProjectContext` (a sibling, already-correct caller of a different project-service RPC) already documents and handles via `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)`. `resolveViaDefaultRepo` never did this — it inherited tenant-only metadata never meant for a project-service call at all.

**Fix #2:** `resolveViaDefaultRepo` now builds its own outgoing context for the `ListRepos` call specifically, forwarding both tenant and user id, mirroring `GetProjectContext`'s exact pattern (including its "user id absent → project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome" posture — this resolver does not invent a fallback identity).

### Testing (fix #2)

- `TestProjectExecutionResolver_FallbackForwardsUserIDToListRepos` — asserts the outgoing context `ListRepos` actually receives carries both `grpcmw.MetadataTenantID` and `grpcmw.MetadataUserID`.
- Live-verified this time via task-service's own new diagnostic log (`resolver_error` field), not by inference alone — the whole point of the fix #2 detour.

## Testing

- `TestProjectExecutionResolver_NotConnected_FallsBackToReachableDefaultRepo` — the core regression: infra-fleet-service reports not-connected, but the default repo's dev server is reachable → `connected=true`.
- `TestProjectExecutionResolver_NotConnected_DefaultRepoUnreachable` / `TestProjectExecutionResolver_NotConnected_DefaultRepoHasNoDevServer` / `TestProjectExecutionResolver_NotConnected_NoFallbackAvailable` — the 3 ways the fallback still correctly reports not-connected.
- Existing `TestProjectExecutionResolver_Connected`/`_NoTenantInContext` updated for the new 3-arg constructor, behavior unchanged.
- **Not yet verified live** (pending this deploy): `GetFleetHealth`'s actual live sample for `test-01` (`a1825a89-...`) — the agent is confirmed connected via its own application log (`readyState=1`, regular heartbeat/health-probe frames), but this doc doesn't independently confirm infra-fleet-service's health-check ticker has already classified it `reachable=true` at the moment of testing. If `task.execute` still fails after this deploy, the next thing to check is `infra-fleet-service`'s own health-check cadence/state for this exact dev server id, not another guess.

## Third bite — the identical bug in `WorktreeProvisioner.resolveRepoID`, one hop further down the same call chain

Fix #2 got the user past `TASK_EXECUTE_NO_CONNECTION`, but the very next retry produced a NEW, different error: `TASK_EXECUTE_WORKTREE_FAILED: failed to provision worktree`. Progress — this is a later step in `SimpleExecutor.Execute` (`ExecuteTask.Execute` calls `uc.worktrees.EnsureWorktree` right after the connection gate passes), not a regression of the same bug.

Added the same permanent diagnostic-logging pattern at this failure site in `execute_task.go` (`slog.WarnContext` logging `worktree_error` right before the `TASK_EXECUTE_WORKTREE_FAILED` `apperrors.New` return) — the same fix #2 taught this investigation to always do before asking for another live retry.

Before spending a retry round-trip on it, re-read `WorktreeProvisioner.resolveRepoID` (`worktree_provisioner.go`) — the function `EnsureWorktree` calls to pick a repo before creating a worktree — and found it has the **exact same shape of bug** as fix #2, independently: it called `p.projects.ListRepos` under `withTenantMetadata`'s tenant-only context, never forwarding the acting user's id, so any live retry would have hit `PROJECT_NO_USER` there too, one call deeper into the same request.

**Fix #3:** `resolveRepoID` now builds its own outgoing context for its `ListRepos` call, forwarding both tenant and user id via `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)` — the identical pattern as fix #2, applied to the sibling call site.

### Testing (fix #3)

- `TestWorktreeProvisioner_ResolveRepoIDForwardsUserIDToListRepos` — asserts the outgoing context `ListRepos` receives (via `fakeProjectServiceClient.gotListReposCtx`) carries both `grpcmw.MetadataTenantID` and `grpcmw.MetadataUserID`.
- `go build`, `go vet`, `go test ./services/task-service/...` all pass.
- **Not yet live-verified** — pending this deploy and a user retry.

## Fourth bite — `SimpleExecutor.Execute`'s own `Relay` call never forwarded tenant id either

Fix #3 got the user past `TASK_EXECUTE_WORKTREE_FAILED`, but the next retry produced yet another new error: `TASK_EXECUTE_FAILED: execution dispatch failed`. Same posture as before — checked task-service's own log first (only the generic swallowed `"rpc failed"` line), then checked **infra-fleet-service's** log for the same trace id (not just task-service's), since this failure is one hop further down the call chain than any of the first 3 fixes. Found the real cause directly, without needing another diagnostic-logging round-trip:

```
{"level":"ERROR","msg":"apperrors: internal cause (not sent to client)","code":"INFRA_NO_TENANT","cause":"tenant: no tenant_id in context"}
{"level":"ERROR","msg":"rpc failed","method":"/orca.infrafleet.v1.InfraFleetService/Relay","error":"rpc error: code = Unauthenticated desc = INFRA_NO_TENANT: no tenant in request context"}
```

**Root cause #4:** `SimpleExecutor.Execute` (`simple_executor.go`) calls `s.relay.Relay(ctx, &infrafleetv1.RelayRequest{Method: "agent.execPrompt", ...})` using the **plain incoming `ctx` directly** — never wrapped with `withTenantMetadata`, unlike every other infra-fleet-service call site in this package (`ProjectExecutionResolver`, `DevServerReachability`). The incoming `ctx` carries tenant only as an inbound-extracted Go value (readable via `tenant.RequireTenantID(ctx)`), never restamped onto an outgoing gRPC call's metadata — exactly the same class of gap as fixes #2/#3, just against infra-fleet-service instead of project-service, and in the one call site that mattered most (the actual agent dispatch, not a lookup before it).

This is a 4th, independent instance of the same underlying lesson repeated through this whole investigation: **a caller having tenant/user identity as a Go context value proves nothing about whether the NEXT outbound gRPC call actually carries it** — each adapter must explicitly build its own outgoing metadata, and nothing enforces that uniformly across call sites today.

**Fix #4:** `Execute` now builds `relayCtx, err := withTenantMetadata(ctx)` once, uses it for the concurrent `StreamExecOutput` call (via `streamCtx := context.WithCancel(relayCtx)`, fixing the same latent gap in `AgentExecOutputRelay`'s best-effort streaming path too) and the unary `Relay` call — mirroring the existing convention in this same file's sibling resolvers rather than inventing a new one.

### Testing (fix #4)

- `TestSimpleExecutor_Execute_ForwardsTenantIDToRelay` — asserts the outgoing context `Relay` receives carries `grpcmw.MetadataTenantID`.
- `TestSimpleExecutor_Execute_NoTenantInContext_FailsClosed` — a ctx with no tenant value fails closed (no Relay call at all) rather than dispatching with blank tenant metadata.
- Every pre-existing `SimpleExecutor` test previously called `Execute` with `context.Background()` (no tenant context value) alongside an explicit `tenantID` string parameter that was never actually used to build outgoing metadata — this masked what would have been a real bug in tests all along. Updated every call site to `ctxWithTenant(t)` (tenant "tenant-1", matching the parameter each test already passes) so these tests exercise the same context shape production code actually produces (`ExecuteTask.Execute` extracts tenant onto `ctx` via `tenant.RequireTenantID` before ever calling `SimpleExecutor.Execute`).
- `go build`, `go vet`, `go test ./services/task-service/...` all pass.
- **Not yet live-verified** — pending this deploy and a user retry.

## Fifth bite — `connectionId` is empty on the fallback path, and `Relay` requires a non-empty one

Fix #4 stopped `INFRA_NO_TENANT`, but the very next retry hit `TASK_EXECUTE_FAILED` again — this time from a **different** infra-fleet-service error, found by reading infra-fleet-service's log for the new request's trace id (not task-service's own log, which only had the generic swallowed line, and not another diagnostic-logging round-trip — the `worker_error`/`dispatch_error` logs added in fixes #2-#4 already made task-service's own log useless for anything past `Relay`'s own boundary, so the fix was to look one service further, same technique as fix #4):

```
"dispatch_error":"simple_executor: relay agent.execPrompt: rpc error: code = InvalidArgument desc = INFRA_RELAY_NO_CONNECTION: connectionId is required"
```

**Root cause #5:** `resolveViaDefaultRepo` (fix #1's fallback, used because `infra.connections` has zero rows system-wide — the norm, not an edge case) returns `connectionID=""` on success, by design — there is no real `infra.connections` row to source one from. But `SimpleExecutor.Execute` unconditionally passed that empty `connectionID` to `Relay`'s `ConnectionId` field, and infra-fleet-service's real `Relay` RPC requires a non-empty one. **Every project in this system resolves through the fallback** (0-row norm), so this was not a rare gap — it made `Relay` fail on literally every dispatch that got this far.

git-gateway-service's `RelayExecutor.relay` already solved this exact problem one layer up: infra-fleet-service has a **`RelayByDevServer`** RPC (`dev_server_id` + `method` + `params_json`, no connection row needed at all) specifically for this "resolved via dev-server fallback, no real connection" case. `ConnectionResolver.ResolveConnection`'s own `!Connected` branch threads the repo's `DevServerID` through `ctx` (`usecase.WithDevServerID`), and `relay()` checks `DevServerIDFromContext(ctx)` first, calling `RelayByDevServer` instead of `Relay` when present.

**Fix #5:** `ProjectExecutionResolver.ResolveConnection` (interface + both implementations) now returns a 4th value, `devServerID` — non-empty only on the fallback path, sourced from the same repo lookup `resolveViaDefaultRepo` already does for the reachability check. `SimpleExecutor.Execute` branches on it directly (a plain return value, not threaded through `ctx` — task-service has exactly one caller of this resolver, not git-gateway-service's ~52, so context-threading would be needless indirection here): `devServerID != ""` → `RelayByDevServer(DevServerId: devServerID, ...)`; otherwise the original `Relay(ConnectionId: connectionID, ...)` unchanged, for the (currently dead, but not deleted) real-connection-row path.

**Known, flagged-not-fixed sibling gap:** `AIDecompose`/`GenerateAgentPrompt` (`ai_decompose.go`, `generate_agent_prompt.go`) also call `ResolveConnection` and pass its `connectionID` straight to `AICompleter.Complete`, which only ever calls the connectionId-keyed `Relay` (no `RelayByDevServer` branch). These two call sites will hit the identical `INFRA_RELAY_NO_CONNECTION` the moment a user actually exercises AI-decompose or generate-prompt on a project resolved via the fallback — not yet hit live (this investigation's user flow has only exercised `task.execute`/`SimpleExecutor`), so not guessed at or fixed here; flagged for whoever picks up either of those two features next.

### Testing (fix #5)

- `TestProjectExecutionResolver_NotConnected_FallsBackToReachableDefaultRepo` extended to assert `devServerID` is returned from the fallback.
- `TestSimpleExecutor_Execute_DevServerIDSet_UsesRelayByDevServer` — the core regression: with `devServerID` set, `RelayByDevServer` is called (with the right `dev_server_id`/`method`) and the connectionId-keyed `Relay` is NOT.
- All pre-existing `SimpleExecutor`/`ProjectExecutionResolver` tests (including fixes #2-#4's) still pass with the widened return signature.
- `go build`, `go vet`, `go test ./services/task-service/...` all pass.
- **Not yet live-verified** — pending this deploy and a user retry.

## Related

- SOL-013 / SOL-014 (this session, git-gateway-service) — the original fix this bug's solution mirrors, one layer up.
- [BUG-025](./BUG-025-task-graph-tree-board-invisible-and-execute-crash.md) — the sibling investigation (Tree/Board visibility + the `worktreePath` crash) from the same live-testing pass; this bug is the 4th, deeper issue found right after those were fixed, once the user could actually reach "Run with Agent" without crashing.
