# BUG-019: `RelayExecutor.RemoveWorktree` never sends a `cwd` param — agent falls back to `AGENT_WORK_DIR`, which doesn't exist on real dev servers

**Service:** `git-gateway-service`
**File:** `internal/adapter/grpcclient/relay_executor.go` (`RemoveWorktree`)
**Severity:** 🔴 High — `git worktree remove` fails for every worktree on every dev-server-hosted repo
**Status:** ✅ Root cause CONFIRMED end-to-end (2026-09-15) via a full live repro chain on the real dev server (`test-01`, 172.20.2.41) — SSH access, direct Node reproduction of the agent's exact spawn call, and filesystem check. ✅ Fixed + unit-tested (build/vet/test all pass), ✅ **deployed** (version `2026.09.15-worktree-remove-cwd-fix`). Live end-to-end verification via the actual UI still pending — `RemoveWorktree` is destructive, so it wasn't re-run directly against the user's real worktree; awaiting the user's own retry.

## Symptom (live, user-reported, persisted across a dev-server-agent redeploy)

```
rpc error: code = Internal desc = WORKTREE_REMOVE_FAILED: git worktree remove failed
```
`git-gateway-service`'s own cause-logging (SOL-012) showed: `grpcclient: relayByDevServer git.worktree.remove: rpc error: code = Internal desc = INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent`, and `infra-fleet-service`'s own cause-logging showed the actual agent-reported error: `"spawn git ENOENT"`.

Redeploying/restarting the Dev Server Agent (ruling out a stale-binary or duplicate-process theory) did NOT fix it — confirming this is a genuine code bug, not an environment/deployment issue.

## Root cause — confirmed end-to-end via live repro

1. `relay_executor.go`'s `RemoveWorktree`:
   ```go
   return r.relay(ctx, worktreePath, "git.worktree.remove", map[string]any{
       "path": worktreePath, "force": force,
   }, nil)
   ```
   Sends `path` and `force` — **never sends `cwd`**.
2. The agent's real `git.worktree.remove` handler (`agent-git-worktree-handler.ts`'s `handleGitWorktreeRemove` — confirmed as the ONLY registration for this method name, in `agent-rpc-dispatch-git.ts`'s dispatch table; `git-handler.ts`'s `removeWorktreeOp`/`git-handler-worktree-remove.ts` is a separate, unrelated code path not wired to this RPC method at all) calls:
   ```ts
   const result = await handleGitExec(id, { args, cwd: params.cwd, timeout: 15_000, ... }, config, log)
   ```
   `params.cwd` is `undefined` (never sent).
3. `handleGitExec`:
   ```ts
   const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
   ```
   Falls back to `config.workDir`, which comes from the `AGENT_WORK_DIR` env var the agent was started with (`start-test-01.sh`: `AGENT_WORK_DIR="/home/ubuntu/projects"`).
4. **`/home/ubuntu/projects` does not exist** on the real dev server (`ls`: "No such file or directory") — repos actually live at `/opt/repos/...`. Node's `child_process.spawn('git', args, { cwd: '<nonexistent dir>' })` fails with `Error: spawn git ENOENT` (a well-known Node gotcha: ENOENT is attributed to the spawned command when the `cwd` itself doesn't exist, not just when the binary is missing).

**Live-reproduced directly**: running the exact same `child_process.execFile('git', ['worktree', 'remove', ...], { cwd: '/opt/repos/aiops-v3-golang-production-ready', env: <agent's real env> })` on the dev server via SSH succeeds normally (a real git error, not ENOENT); the failure is specifically and only about the missing `cwd`.

## Fix

Send `cwd: worktreePath` alongside `path`/`force` in `RemoveWorktree`'s relay params — the worktree's own (already-resolved-by-SOL-013/014) real path is a valid, existing directory inside the repo, and running `git worktree remove <path>` with `cwd` set to that same path works (confirmed via the live repro above).

## Related

- Investigated in the same session as [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md)/[SOL-014](./solutions/SOL-014-connection-resolver-relay-via-dev-server-reachability.md) — this is a genuinely separate bug (a missing RPC param, not a path-resolution or executor-selection issue); `dispatchExecutor`'s own path resolution was confirmed correct throughout this investigation.
- A duplicate-dev-server-agent-process theory was investigated and ruled out (found and fixed as incidental cleanup — the agent runs under systemd, `orca-agent-test-01.service`; manual process management should go through `systemctl`, not raw `kill`/`nohup`).
