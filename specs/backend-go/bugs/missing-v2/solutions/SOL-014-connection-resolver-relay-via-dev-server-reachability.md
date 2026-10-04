# SOL-014: `ConnectionResolver.ResolveConnection` relays via dev-server reachability when there's no `infra.connections` row

> **✅ IMPLEMENTED + DEPLOYED + LIVE-VERIFIED (2026-09-15)**, version `2026.09.15-sol014-fix`. Confirmed end-to-end via direct `grpcurl GetStatus` calls for BOTH of BUG-012's original failing worktree ids — both now return a real `git status` result (`{"branch": "refs/heads/golang-production-ready"}` and `{"branch": "refs/heads/main"}`, respectively), with matching `trace_id` log evidence showing infra-fleet-service's `RelayByDevServer` RPC (`rpc ok`) immediately followed by git-gateway-service's `GetStatus` (`rpc ok`) — the exact mechanism this fix was designed to trigger, not a coincidental pass. **This closes the full BUG-012 → BUG-015 chain**: the Git tab now works end-to-end for a dev-server-hosted worktree, for both the real-UUID and the externally-detected-composite-id cases. Companion to [SOL-013](./SOL-013-connection-resolver-worktree-path-fallback.md), which fixed *what path* is used; this fixes *which host runs the command*. Same shared chokepoint (`ConnectionResolver.ResolveConnection`), same "fix once, don't touch 34 files individually" philosophy — but this time the interface itself had to change, so the mechanical edit touched every call site (a 1-line, uniform, low-risk change each). **Correction to this doc's original audit**: a SECOND dispatch-helper family, `dispatchFilesystemExecutor` (12 more callers — `read_file.go`, `write_file.go`, `write_file_chunk.go`, `read_file_preview.go`, `stat_file.go`, `read_dir.go`, `list_all_files.go`, `list_markdown_documents.go`, `search_files.go`, `create_file.go`, `create_dir.go`, `delete_file.go`), was found and fixed identically only while implementing — the original design's "~46 call sites" estimate should read ~58 (33 `dispatchExecutor` + 15 inline `ResolveConnection` + 12 `dispatchFilesystemExecutor`, minus overlaps already counted). Kept here as a correction, not silently revised, matching this session's own precedent (BUG-009/BUG-011).

## Bug/CR reference
- **Bug:** [BUG-015](../BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md)
- **CR:** [CR-PW-011](../../../../../docs/crs/v3/project-workspace/CR-PW-011-dispatch-executor-never-relays-to-dev-server.md)

## Root cause (recap — see BUG-015 for full live evidence)

`dispatchExecutor`/~15 inline `ResolveConnection` callers pick `relay` vs `local` purely from `ResolvedConnection.Connected`, which is only ever true when `infra.connections` has a row for the worktree — confirmed (again) to be a "zero rows, system-wide" condition. `dispatchExecutorForRepo` (the repo-scoped counterpart, already working for `CreateWorktree`/`DetectWorktrees`) solved this months ago by checking `repo.DevServerID` + `DevServerReachability.IsReachable(...)` independently of `infra.connections`, and threading the result via `usecase.WithDevServerID(ctx, ...)`. The worktree-keyed path (`dispatchExecutor` + inline callers) never got the same treatment.

**Key existing mechanism (already built, unmodified by this fix)**: `RelayExecutor.relay()` (`relay_executor.go:64`) already checks `usecase.DevServerIDFromContext(ctx)` FIRST, and calls `RelayByDevServer` instead of the connectionId-keyed `Relay` RPC when present — completely independent of which usecase or dispatch path populated it. This is the ONE shared chokepoint every `GitExecutor`/`FilesystemExecutor`/`AICompleter` method (~52 of them, per that function's own doc comment) already funnels through. **This fix only needs to get the right `ctx` to that chokepoint — no new relay mechanism.**

## Design

### Why the interface must change

`ConnectionResolver.ResolveConnection(ctx, worktreeID) (ResolvedConnection, error)` cannot report "please relay to this dev server" without a way to get `WithDevServerID(ctx, ...)`'s result back to the caller — Go contexts are immutable value types; a `context.WithValue` call inside `ResolveConnection` produces a new `context.Context` that's thrown away the moment the function returns, unless it's part of the return value. So the interface becomes:

```go
type ConnectionResolver interface {
	ResolveConnection(ctx context.Context, worktreeID string) (context.Context, ResolvedConnection, error)
}
```

Every caller must now do `ctx, conn, err := resolver.ResolveConnection(ctx, id)` (rebinding its own `ctx`) instead of `conn, err := ...` — a 1-line, mechanical, uniform edit, but one that touches every call site (`gitnexus impact` on the interface method: **50 impacted, MEDIUM risk, direct: 2** — the `dispatchExecutor` wrapper plus depth-2 fan-out covering the ~46 real call sites once both `dispatchExecutor`'s 33 callers and the ~15 inline callers are counted).

### `grpcclient.ConnectionResolver` changes

Add a `reachability usecase.DevServerReachability` dependency (3rd constructor param, alongside SOL-013's `projects`). Extend the `!Connected` branch:

```go
if !resp.GetConnected() {
    local, err := r.resolveLocal(ctx, dispatchID, isRepoScoped)  // SOL-013's resolveLocalRepoPath, extended
    if err != nil {
        return ctx, usecase.ResolvedConnection{}, fmt.Errorf(...)
    }
    if local.DevServerID != "" {
        reachable, err := r.reachability.IsReachable(ctx, local.DevServerID)
        if err != nil {
            return ctx, usecase.ResolvedConnection{}, fmt.Errorf(...)
        }
        if reachable {
            relayCtx := usecase.WithDevServerID(ctx, local.DevServerID)
            if local.HiddenTargetID != "" {
                relayCtx = usecase.WithHiddenTargetID(relayCtx, local.HiddenTargetID)
            }
            return relayCtx, usecase.ResolvedConnection{Connected: true, RepoPath: local.Path, HiddenTargetID: local.HiddenTargetID}, nil
        }
    }
    return ctx, usecase.ResolvedConnection{Connected: false, RepoPath: local.Path}, nil
}
```

`resolveLocalRepoPath` (SOL-013) is extended to also resolve `DevServerID`/`HiddenTargetID` alongside `Path`, for all 3 dispatch-id shapes:
- **Composite `repoId::path`** (external worktree): path is embedded (unchanged from SOL-013); the extracted `repoID` is now ALSO used for a `GetRepo` call to learn `DevServerID`. If that `GetRepo` call fails, degrade to `Path`-only (no `DevServerID`) rather than failing the whole resolution — the path is still valid and usable locally; the dev-server enrichment is a bonus, not load-bearing.
- **`repo:`-prefixed** (repo-scoped): already calls `GetRepo` for `Path` (=`repo.URL`) — `DevServerID`/`HiddenTargetID` come along for free from the same call, no extra RPC.
- **Real worktree id**: already calls `GetWorktree` for `Path` — needs a SECOND call, `GetRepo(wt.RepoID)`, for `DevServerID`. Same degrade-gracefully-on-failure posture as the composite case: a `GetRepo` failure here must not invalidate the already-resolved `Path`.

`ConnectionID` and `Mode` are deliberately left unset (zero value) for this new relay path — see "Not fixed by this pass" below for why, and what that means for callers that read them.

### `dispatchExecutor` (ports.go)

```go
func dispatchExecutor(ctx context.Context, resolver ConnectionResolver, local, relay GitExecutor, worktreeID string) (context.Context, GitExecutor, string, error) {
	ctx, conn, err := resolver.ResolveConnection(ctx, worktreeID)
	if err != nil {
		return ctx, nil, "", err
	}
	if conn.Connected {
		return ctx, relay, conn.RepoPath, nil
	}
	return ctx, local, conn.RepoPath, nil
}
```
Now mirrors `dispatchExecutorForRepo`'s 4-value return exactly.

### Call sites (the mechanical part)

Two uniform patterns, ~46 files total (audited via grep, not guessed):

1. **33 files** call `dispatchExecutor(...)` as `X, Y, err := dispatchExecutor(CTXVAR, uc.resolver, uc.local, uc.relay, ID)` → becomes `CTXVAR, X, Y, err := dispatchExecutor(CTXVAR, uc.resolver, uc.local, uc.relay, ID)` (reusing whatever context variable that call site already uses — `ctx` in all but `compare_worktrees.go`'s `gctx` and `merge_worktree_into_base.go`'s two distinct dispatch calls with their own var names).
2. **14 files** call `uc.resolver.ResolveConnection(ctx, ID)` inline as `conn, err := ...` → `ctx, conn, err := ...`.
3. **2 files** (`remove_worktree.go`, `check_worktree_delete_safety.go`) have a SECOND, separate inline `ResolveConnection` call whose only use of `conn` is `conn.Connected`/`conn.ConnectionID` for a `TerminalSessionLister` check — unrelated to git dispatch. The returned ctx there is discarded (`_`) since no relay-aware executor call follows it in that branch.

Every one of the ~46 sites was read (not sed-blind) to confirm this pattern before editing — see "Verification" below.

## Not fixed by this pass (explicit, matching CR-PW-011's own scope note)

1. **`Mode` (SSH-relay-mode fail-closed) is not resolved for this new path.** `MergeBranch`/`CreateBranch`/`DeleteBranch`/`StashPush`/`StashPop`/`PushStream`/`PullStream` all check `conn.Mode == CONNECTION_MODE_RELAY_SSH` to refuse an operation the ssh-relay transport doesn't support (SOL-PW-03), BEFORE attempting it. `DevServerReachability.IsReachable` only returns a bool, no mode — so this check silently never fires for a connection resolved via this new path (same as `dispatchExecutorForRepo`'s pre-existing, accepted limitation). This was already a *latent* gap (the check never fired at all, since `Connected` was always false) — this fix is the first time a real relay path is exercised, turning a latent gap into a live one. Flagged as a fast-follow, not blocking this fix.
2. **`ConnectionID`-keyed calls that don't go through `relay()`'s shared chokepoint** don't benefit from this fix and keep failing the same way they always have (not a regression — these usecases were ALREADY 100% non-functional for every worktree, since `Connected` was always false before):
   - `WatchWorktreeFiles` → `FileWatchStreamer.StreamFileChanges` calls infra-fleet-service's `StreamFileChanges` RPC directly with `ConnectionId: conn.ConnectionID` (empty for this path) — does not read ctx's `DevServerID` at all (only `relay()`/nothing else does). Still broken for dev-server-hosted worktrees after this fix, same as before.
   - `RemoveWorktree`'s/`CheckWorktreeDeleteSafety`'s BR-WT-10 active-session check (`TerminalSessionLister.ListSessions(ctx, conn.ConnectionID)`) — same shape, silently no-ops (fails open, per that code's existing `lErr == nil` guard) rather than erroring.
3. **`RenameFile`/`CopyFile`/`ReadFileChunk` will now correctly return `ErrFileOpNotSupportedOverRelay`/`ErrChunkedReadNotSupportedRemote`** for a dev-server-hosted worktree, instead of silently attempting `uc.local.*` against a path that was never on this host's filesystem (which would have failed with a confusing ENOENT-style error). This is a deliberate, pre-existing, already-documented limitation (BUG-009-era) being correctly enforced for the first time, not a new bug — flagged here so it isn't mistaken for a regression when observed live.
4. Same `infra.connections`-table redesign question CR-PW-010 already put out of scope — untouched.

## Testing — done

1. **6 new unit tests** in `grpcclient_test.go` for `ConnectionResolver.ResolveConnection`'s relay-via-reachability branch: reachable dev server → `Connected: true` + ctx carries `WithDevServerID`/`WithHiddenTargetID`, empty `ConnectionID` (`TestConnectionResolver_ResolveConnection_NotConnected_ReachableDevServer_RelaysViaDevServerID`); unreachable → falls back to `Connected: false` + real path, no `DevServerID` in ctx (`..._UnreachableDevServer_FallsBackToLocal`); `DevServerID == ""` → `IsReachable` never called, unchanged SOL-013 behavior (`..._NoDevServerBound_UnchangedSOL013Behavior`); reachability check error → propagates (`..._ReachabilityCheckFails_Propagates`); composite-id's `GetRepo` enrichment failure degrades to path-only rather than failing the whole call (`..._ExternalWorktree_DevServerLookupFails_DegradesToPathOnly`).
2. **1 new unit test** in `dispatch_test.go` (`TestGetStatus_ConnectionResolverEnrichesCtx_ThreadsThroughToExecutor`) using a new `fakeConnectionResolverReturnsCtx` fake — proves `dispatchExecutor` passes the resolver-enriched ctx (not the original) to the executor call, the exact mechanism the whole fix depends on.
3. All ~58 call sites' own pre-existing tests still pass unchanged after the mechanical ctx-rebind — `go build`, `go vet`, and full `go test ./services/git-gateway-service/...` all pass, package-wide.
4. `gitnexus detect_changes` (vs. `main`) shows no unexpected affected execution flows beyond what earlier, already-deployed changes in this session touched — nothing new flagged from this specific change.
5. Live re-verification (mirrors SOL-013's own verification): planned post-deploy — re-run the same `grpcurl GetStatus` call against the `aiops-v3` worktree, this time expecting `RelayByDevServer` to be attempted (visible in infra-fleet-service's own logs) instead of a local `chdir` failure.
