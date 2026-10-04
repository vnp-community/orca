# BUG-015: `dispatchExecutor` always picks `local` executor for a worktree-keyed dispatch — never relays to the repo's real dev server

**Service:** `git-gateway-service`
**File:** `internal/usecase/ports.go`'s `dispatchExecutor`, `internal/adapter/grpcclient/resolver.go`'s `ConnectionResolver.ResolveConnection`
**Severity:** 🔴 High — this is the layer directly beneath BUG-012/CR-PW-010/SOL-013; fixing SOL-013 alone does not make the Git tab work for any worktree hosted on a real (non-git-gateway-service-local) dev server.
**Status:** ✅ **Fixed, deployed, and live-verified (2026-09-15)** — [SOL-014](./solutions/SOL-014-connection-resolver-relay-via-dev-server-reachability.md), version `2026.09.15-sol014-fix`. Live `grpcurl GetStatus` against the exact worktree this bug's evidence came from now succeeds (`{"branch": "refs/heads/golang-production-ready"}`), with `trace_id`-matched logs confirming `RelayByDevServer` → `GetStatus` both `rpc ok`.

---

## What's confirmed

Live `grpcurl` call to `git-gateway-service`'s `GetStatus` RPC for worktree `01579d39-64c0-482f-a0e6-eeb577e404f7` (post SOL-013 deploy, version `2026.09.15-sol013-fix`):

```json
{"code":"GITGATEWAY_STATUS_FAILED","cause":"git status --porcelain=v1 -b: chdir /opt/repos/aiops-v3-golang-production-ready: no such file or directory: "}
```

SOL-013 is confirmed working exactly as designed — the `chdir` target is now the **real** path (`/opt/repos/aiops-v3-golang-production-ready`, matching `project.worktrees.path` in the DB) instead of the raw worktree UUID. But the call still fails, because:

1. `docker inspect orca-go-git-gateway` shows this container's only mount is its own private `/data/repos` volume — it has **no filesystem access** to `/opt/repos/aiops-v3-golang-production-ready`, which lives on a separate dev-server host.
2. `project.repos` confirms this repo (`45f573e8-be1a-4b1a-894b-a7c261fb7331`) has `dev_server_id = a1825a89-0053-4b64-a4f7-d5b476f9f1e0` — a real, bound dev server. This is NOT a "no dev server bound, operate locally" case.
3. Yet `dispatchExecutor` (`ports.go`) dispatched to `local`, not `relay` — because its only decision input is `ConnectionResolver.ResolveConnection`'s `Connected` bool, which comes from `infra.connections` having a row for this worktree — confirmed (same "zero rows, system-wide" fact CR-PW-010 already documented) to never be true.

## Root cause

`dispatchExecutor`'s dispatch decision is a strict binary:
```go
func dispatchExecutor(ctx context.Context, resolver ConnectionResolver, local, relay GitExecutor, worktreeID string) (GitExecutor, string, error) {
    conn, err := resolver.ResolveConnection(ctx, worktreeID)
    ...
    if conn.Connected {
        return relay, conn.RepoPath, nil
    }
    return local, conn.RepoPath, nil   // ← reached for EVERY worktree, since infra.connections is always empty
}
```
Unlike `dispatchExecutorForRepo` (used by `CreateWorktree`/`DetectWorktrees` et al.), which independently checks `repo.DevServerID` + `DevServerReachability.IsReachable(...)` and threads the result via `WithDevServerID(ctx, ...)` so `RelayExecutor` can call infra-fleet-service's `RelayByDevServer` RPC, `dispatchExecutor` has no equivalent fallback. It trusts `infra.connections` exclusively — and since that table has zero rows system-wide, every worktree-keyed usecase (`GetStatus`, `GetDiff`, `Commit`, ... the same ~34 callers CR-PW-010 already audited) always dispatches to `local`, regardless of whether the worktree's repo actually has a live, reachable dev server.

SOL-013 fixed **what path is used** (the immediate visible symptom). This bug is **which host runs the command** — a distinct decision, made by the same function, from the same broken assumption (`infra.connections` will have rows) that CR-PW-010 already flagged as a known, explicitly out-of-scope architectural question ("tại sao bảng này luôn rỗng 'system-wide'").

## Why this wasn't caught before deploying SOL-013

SOL-013's own unit tests (fakes) never exercise a worktree whose repo has a real `DevServerID` — they only assert the **path value** resolves correctly, which is all SOL-013 promised. This gap surfaced immediately on live re-verification against the exact worktree BUG-012's original log evidence came from, which happens to be dev-server-hosted (the realistic case in this deployment, not an edge case).

## Fix direction (not designed yet — needs its own CR before implementing)

Same shape as `dispatchExecutorForRepo`'s existing solution, applied to the worktree-keyed path:
1. When `ConnectionResolver.ResolveConnection` resolves `!Connected` and the underlying repo has a non-empty `DevServerID` (available via the same `GetWorktree`→`GetRepo` chain SOL-013 already added), check `DevServerReachability.IsReachable(repo.DevServerID)`.
2. If reachable, dispatch should return `relay` and thread the `DevServerID` into `ctx` (`WithDevServerID`) so `RelayExecutor` calls `RelayByDevServer` instead of `Relay` — exactly mirroring `dispatchExecutorForRepo`.
3. This requires widening `dispatchExecutor`'s signature to also return `ctx` (like `dispatchExecutorForRepo` already does) — which means **touching all ~34 call sites** CR-PW-010 already enumerated, to thread the returned `ctx` through to their `local`/`relay` executor calls. Same CRITICAL blast radius as CR-PW-010, but a bigger diff (signature change propagated to every caller, not a single chokepoint fix).

**Not implemented in this pass** — deliberately stopping here to log this before touching `dispatchExecutor`'s signature again, matching this session's own "spec before code" convention, especially given the size of the diff this implies.

## Related

- [BUG-012](./BUG-012-gitgateway-status-failed-opaque-relay-error.md) / [CR-PW-010](../../../../docs/crs/v3/project-workspace/CR-PW-010-git-status-worktree-id-resolver-broken.md) / [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md) — SOL-013 fixed the path-echo symptom; this bug is the next layer, found immediately while live-verifying SOL-013's deploy.
- `dispatchExecutorForRepo` (`ports.go`) — the existing, working pattern this fix should mirror.
