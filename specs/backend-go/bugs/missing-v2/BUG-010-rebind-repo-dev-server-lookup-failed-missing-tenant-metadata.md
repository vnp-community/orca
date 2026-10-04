# BUG-010: `PROJECT_DEV_SERVER_LOOKUP_FAILED` — `RebindRepoDevServer`/`AddRepo`/`CreateProject` can't validate a dev server exists, because the outbound `ListDevServers` call carries no tenant

**Service:** `project-service`
**File:** `internal/adapter/grpcclient/infra_fleet_dev_server_lister.go` (`InfraFleetDevServerLister.Exists`) — root cause; also `dev_server_hostname_resolver.go` (`InfraFleetHostnameResolver.Hostname`), same bug, degrades silently instead of erroring
**Severity:** **High** — blocks every attempt to bind a repo to a dev server through the UI
**Symptom:**
```
rpc error: code = Internal desc = PROJECT_DEV_SERVER_LOOKUP_FAILED: failed to validate dev server
```
**Status:** ✅ Root cause CONFIRMED (2026-09-15, live log evidence) — fix in [SOL-011](./solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md), TASK-022/023. ✅ **Deployed** (`sync-to-server.sh 2026.09.15-bug010-fix`) — `project-service` confirmed running this version on `b15.openledger.vn`. Not yet confirmed by an actual repro attempt (retrying `repo.rebindDevServer` for "aiops-v3" → `test-01`); update this line once done.

---

## Trigger

Reported live on `b15.openledger.vn` while the user tried to fix [BUG-FE-PW-003](../../../frontend/bugs/project-workspace/BUG-FE-PW-003-project-settings-dialog-no-scroll-cuts-off-content.md)'s underlying data problem the UI way: `ProjectSwitcher`'s ⚙️ → Project Settings → General tab → pick `test-01` for repo "aiops-v3" → Save → `repo.rebindDevServer` → this error, every time, 3 retries in a row, all failing identically.

## Root Cause — CONFIRMED via live logs

`project-service`'s `RebindRepoDevServer.Execute` (also `AddRepo.Execute`, `CreateProject.Execute` — all three share the same `DevServerLister` port) calls:

```go
if in.NewDevServerID != "" {
    exists, err := uc.devServers.Exists(ctx, tenantID, in.NewDevServerID)
    if err != nil {
        return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
    }
    ...
```

`Exists` is implemented by `InfraFleetDevServerLister` (`infra_fleet_dev_server_lister.go`), which dials `infra-fleet-service.ListDevServers` **directly on the incoming `ctx`**, without calling this package's own `withTenantMetadata(ctx)` helper first — the exact same bug shape as [BUG-008](./BUG-008-project-profile-resolve-missing-tenant-metadata.md) (`TenantProfileResolver.GetResolvedProfile`), just a different call site. `project-service`'s own sibling files (`task_execution_checker.go`, `workflow_execution_checker.go`, and — since BUG-008's fix — `profile_resolver.go`) all call `withTenantMetadata` before their outbound RPC; this one never did.

Confirmed with exact log evidence, both sides of the call, same `trace_id` across both services:

```json
// project-service — the visible error
{"time":"2026-09-15T06:14:11.660955928Z","level":"ERROR","msg":"rpc failed","service":"project-service","method":"/orca.project.v1.ProjectService/RebindRepoDevServer","error":"rpc error: code = Internal desc = PROJECT_DEV_SERVER_LOOKUP_FAILED: failed to validate dev server","trace_id":"d9b0371db38203cb4effa1a7bfb1415b"}

// infra-fleet-service — the real cause, one hop downstream, same trace_id
{"time":"2026-09-15T06:14:11.660693133Z","level":"ERROR","msg":"apperrors: internal cause (not sent to client)","service":"infra-fleet-service","code":"INFRA_NO_TENANT","cause":"tenant: no tenant_id in context"}
{"time":"2026-09-15T06:14:11.660758441Z","level":"ERROR","msg":"rpc failed","service":"infra-fleet-service","method":"/orca.infrafleet.v1.InfraFleetService/ListDevServers","error":"rpc error: code = Unauthenticated desc = INFRA_NO_TENANT: no tenant in request context","trace_id":"d9b0371db38203cb4effa1a7bfb1415b"}
```

`infra-fleet-service`'s `ListDevServers` usecase requires a tenant ID via `tenant.RequireTenantID(ctx)`, which reads it from **inbound** gRPC metadata (populated by `grpcmw.TenantExtractionInterceptor`). Since `project-service`'s outbound call never re-attaches that metadata for this hop, the callee sees no tenant at all and fails closed with `INFRA_NO_TENANT` — 100% deterministic, not intermittent (3/3 retries failed identically, same cause every time).

**Note the contrast with the same log window's `ListDevServers` calls that succeeded** (`"rpc ok"`, correct `tenant_id"00000000-0000-0000-0000-000000000001"`) — those are a *different* caller: the frontend's own `devServer.list` wscompat channel, which goes through `api-gateway`'s `gatewaygrpc.AttachIdentity` and does attach identity correctly. Only the `project-service → infra-fleet-service` server-to-server hop is broken.

## Why this wasn't caught by BUG-008/SOL-008's fix

BUG-008 fixed the *one* call site reported at the time (`TenantProfileResolver.GetResolvedProfile`). `missing-v2/solutions/README.md`'s own "Cross-cutting design theme" already flagged this exact risk: *"wscompat's pattern of 'N handlers must each remember to call a helper' is proving failure-prone."* This is the same shape one layer down — `project-service`'s own `grpcclient` package has 5 files, and only fixing the one that was reported (BUG-008) left 2 more (`infra_fleet_dev_server_lister.go`, `dev_server_hostname_resolver.go`) with the identical gap, undiscovered until this specific RPC was actually exercised live.

## Impact

- **`repo.rebindDevServer`** (this bug's trigger) — always fails when `NewDevServerID != ""` (the "unbind"/empty case skips the `Exists` check entirely and would succeed, which is itself a red herring that hides this bug for the "clear the binding" path).
- **`repo.add`** (`AddRepo.Execute`) and **`project.create`**'s follow-up (`CreateProject.Execute`) share the same `DevServerLister.Exists` port — any new repo/project creation that specifies a non-empty `devServerId` hits the same failure. This explains why the repo bindings that *do* exist correctly in production (`aiops-v3` under "AI-Ops", `vnp-asm`) predate this regression, or were set through a path that doesn't hit this validation (not confirmed which — out of scope for this bug, noted for awareness only).
- `InfraFleetHostnameResolver.Hostname` (display-only, used by `GetProjectContext`'s `dev_server_hostname` field) has the identical bug but degrades silently (its own doc comment: "a lookup failure never fails the whole read") — so it just always returns an empty hostname instead of erroring. Lower severity, same root cause, fixed in the same solution since it's the same 2-line change.

## Related

- [BUG-008](./BUG-008-project-profile-resolve-missing-tenant-metadata.md) — identical bug shape, different call site, same service.
- [SOL-011](./solutions/SOL-011-dev-server-lister-forward-tenant-metadata.md) — fix.
- [BUG-FE-PW-003](../../../frontend/bugs/project-workspace/BUG-FE-PW-003-project-settings-dialog-no-scroll-cuts-off-content.md) — the frontend bug the user was trying to work around when this one surfaced; both were blocking the same end-to-end task (bind repo "aiops-v3" to dev server `test-01` so `WORKTREE_CREATE_FAILED` stops happening).
