# SOL-013: `ConnectionResolver.ResolveConnection` resolves a real filesystem path instead of echoing back the worktree ID when disconnected

> **✅ IMPLEMENTED + DEPLOYED + LIVE-VERIFIED (2026-09-15)**, version `2026.09.15-sol013-fix`. Confirmed via direct `grpcurl` call + live log: `chdir` now targets the real resolved path (`/opt/repos/aiops-v3-golang-production-ready` / `/opt/repos/aiops-v3`) instead of the raw worktree id/composite id — the exact symptom BUG-012's log evidence showed. **However**, `GetStatus` still fails for this worktree, for a NEW, distinct reason found immediately during this verification: see [BUG-015](../BUG-015-dispatch-executor-always-local-never-relays-to-dev-server.md) — `dispatchExecutor`'s executor SELECTION (local vs relay) is separately broken, unrelated to the path value this fix corrected. SOL-013's own scope (path resolution) is fully done and verified. ⚠️ **CRITICAL blast radius confirmed via `gitnexus impact({target: "dispatchExecutor"})`: 38 impacted symbols, 34 direct callers** — practically every worktree-scoped usecase in `git-gateway-service` (`GetStatus`, `GetDiff`, `Commit`, `Push`, `Pull`, `Checkout`, `Stage`/`Unstage`, `Discard`/`BulkDiscard`, `History`, `BranchCompare`/`BranchDiff`, `CommitCompare`/`CommitDiff`, `FastForward`, `RebaseFromBase`, `AbortMerge`/`AbortRebase`, `ConflictOperation`/`ResolveConflict`, `Fetch`, `ForceDeleteBranch`, `ForkSync`, `ListLocalBranches`, `MergeWorktreeIntoBase`, `RemoteCommitURL`/`RemoteFileURL`, `RemoveWorktree`, `CheckIgnored`, `CheckWorktreeDeleteSafety`, `SubmoduleStatus`, `UpstreamStatus`, `CompareWorktrees` — plus transitively `GenerateCommitMessage`/`GeneratePullRequestFields` via `diff_composer.go`). Fixed at the single shared chokepoint (`ConnectionResolver.ResolveConnection`), not in each of the 34 files.

## Bug Reference
- **Bug:** BUG-012 (`GITGATEWAY_STATUS_FAILED` was the reported symptom; this fixes the shared root cause behind it and likely dozens of other silent/未-reported failures for the same reason)
- **CR:** [CR-PW-010](../../../../docs/crs/v3/project-workspace/CR-PW-010-git-status-worktree-id-resolver-broken.md)
- **Severity:** 🔴 CRITICAL blast radius, High severity per-symptom

---

## Root Cause — CONFIRMED (see BUG-012 for the live log evidence)

`internal/adapter/grpcclient/resolver.go`:
```go
func (r *ConnectionResolver) ResolveConnection(ctx context.Context, worktreeID string) (usecase.ResolvedConnection, error) {
    ...
    dispatchID := strings.TrimPrefix(worktreeID, repoDispatchPrefix)
    resp, err := r.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{WorktreeId: dispatchID})
    ...
    if !resp.GetConnected() {
        return usecase.ResolvedConnection{Connected: false, RepoPath: dispatchID}, nil   // ← BUG
    }
    ...
}
```
When infra-fleet-service reports "not connected" for a worktree (its own code comment: *"No infra.connections row exists for these repos — confirmed live: zero rows, system-wide"* — meaning this branch fires for **every** worktree, not an edge case), the function returns the **input ID itself** as `RepoPath`. `dispatchExecutor` (`ports.go:617-626`) then hands this straight to whichever `GitExecutor` (`local` or `relay`) as `cwd`/`repoPath`, with zero validation that it's a real path.

## Impact — CRITICAL, re-confirm before any implementation

```
gitnexus impact({target: "dispatchExecutor", direction: "upstream", repo: "orca"})
→ risk: CRITICAL, impactedCount: 38, direct: 34
```

Every one of the 34 direct callers listed above is potentially affected **every time it runs against a worktree with no `infra.connections` row** — which, per the resolver's own doc comment, is the norm ("zero rows, system-wide"), not the exception. This means the actual live blast radius may be "every git operation on every worktree," not just the `GetStatus` symptom this session happened to catch first. **This has NOT been verified for each of the 34 callers individually** — only `GetStatus` has live log confirmation. Some callers may behave differently (e.g. some may tolerate a bad `cwd` more gracefully, or hit a different code path first) — do not assume all 34 fail identically without checking.

## Proposed fix

### Design: fix once, at the shared resolver — do not touch 34 files individually

Since every caller goes through the same `ConnectionResolver.ResolveConnection`, the fix belongs **there**, not in each of the 34 usecases. Two viable approaches:

**Option A — resolve the real path via project-service when disconnected** (preferred):
```go
if !resp.GetConnected() {
    repoInfo, err := r.projects.GetWorktreeRepo(ctx, dispatchID)  // NEW method, see below
    if err != nil {
        return usecase.ResolvedConnection{}, fmt.Errorf("grpcclient: resolving worktree %q's repo path: %w", dispatchID, err)
    }
    return usecase.ResolvedConnection{Connected: false, RepoPath: repoInfo.URL}, nil
}
```
Requires a **new method** on the `ProjectClient` interface (`ports.go:444`) and a corresponding project-service gRPC capability — project-service already has the data (`project.worktrees.path` and/or `project.repos.url`), likely just needs a new RPC or reuse of an existing one (check `GetRepo`/`ListWorktrees` first before adding a new proto method — may already be enough data available without a new wire contract).

**Option B — narrower, GetStatus-only fix**: migrate `GetStatus` specifically off `dispatchExecutor` onto `dispatchExecutorForRepo` (the already-working alternative `CreateWorktree`/`DetectWorktrees` use), by first resolving `WorktreeID → domain.RepoInfo` inline in `GetStatus.Execute`. Fixes the one reported symptom fast, but **leaves the other 33 callers with the identical latent bug** — not recommended as the final fix, only as a stopgap if Option A's project-service work needs more lead time.

### Recommendation

Given the CRITICAL blast radius, do **Option A** — fixing the shared resolver closes this for all 34 callers at once, and is the same total effort as auditing+patching them individually would require anyway (since each would need the same "resolve real path" logic if done inline).

## Implementation (2026-09-15) — what actually shipped

Option A's premise ("a new `ProjectClient` method is likely needed") turned out to be **half true**: `ProjectClient.GetWorktree(worktreeID) → domain.WorktreeInfo` **already existed** (added for `CompareWorktrees`), but its wire mapping dropped `Worktree.path` (project.proto field 4) on the floor because `CompareWorktrees` never needed it. No new proto/RPC was required — just:

1. **`domain.WorktreeInfo`**: added a `Path string` field.
2. **`ProjectClient.GetWorktree`** (`grpcclient/project_client.go`): map `wt.GetPath()` into it (1-line addition).
3. **`ConnectionResolver`** (`grpcclient/resolver.go`): added a `projects usecase.ProjectClient` field (`NewConnectionResolver`'s signature now takes it as a second param — LOW risk per `gitnexus impact`, 1 direct caller: `cmd/server/main.go`'s `run`). The `!Connected` branch now calls a new `resolveLocalRepoPath` helper instead of echoing `dispatchID`:
   - **`"repoId::path"` composite id** (externally-detected worktrees, `mergeDetectedWorktrees`) → path is already embedded in the id; split on `"::"` directly, **no RPC call** (there is no `project.worktrees` row for these — that's exactly why this shape exists).
   - **`"repo:"`-prefixed id** (`MergeWorktreeIntoBase`'s repo-scoped dispatch) → `projects.GetRepo(ctx, repoID).URL`.
   - **otherwise, a real worktree id** → `projects.GetWorktree(ctx, worktreeID).Path`.
4. **`cmd/server/main.go`**: reordered so `projectClient` is constructed before `resolver` (it wasn't previously, since `resolver` didn't need it), and threaded through.

This one change point fixes all 34 `dispatchExecutor` callers identically — none of the 34 usecase files themselves were touched.

## Not done in this solution / known residual gaps

- **Not fixed**: the underlying "why does `infra.connections` have zero rows system-wide" design question (explicitly out of scope for CR-PW-010, see that CR's "Không thuộc phạm vi" section) — this fix makes the `!Connected` path correct, it doesn't address whether `Connected` should be true more often.
- **Not audited individually**: whether any of the 34 callers has its own pre-existing defensive/compensating logic around a bad `RepoPath` that this fix might now interact with differently — spot-checked via reading `get_status.go`/`commit.go`/`history.go`/`get_diff.go`'s call sites (all use the same simple `executor, repoPath, err := dispatchExecutor(...)` pattern, no divergent handling found), not all 34.
- **A real worktree whose project-service row was deleted/never existed**: `GetWorktree` now returns a real error instead of the old silent "operate on a bogus path" behavior — a behavior change (fail loud vs. fail silently-wrong), considered strictly better but not explicitly requested/signed off as a UX change.
- **Not live-verified against `b15.openledger.vn`** yet at the time of writing — only unit-tested. Per this session's established pattern (BUG-012/BUG-013), should be confirmed against the actual worktree/log evidence that originally surfaced this bug (`01579d39-...`/`45f573e8-...::/opt/repos/aiops-v3`) after deploy.

## Testing — done

Added to `grpcclient_test.go`:
1. `TestConnectionResolver_ResolveConnection_NotConnected_ResolvesRealWorktreePath` — replaces the old test that asserted the buggy echo-back behavior as "expected"; now asserts a real path from `GetWorktree`.
2. `TestConnectionResolver_ResolveConnection_NotConnected_ExternalWorktree_UsesEmbeddedPath` — the `"repoId::path"` composite-id case, and that `GetWorktree` is NOT called for it.
3. `TestConnectionResolver_ResolveConnection_NotConnected_RepoScoped_UsesGetRepo` — the `"repo:"`-prefix case.
4. `TestConnectionResolver_ResolveConnection_NotConnected_ProjectServiceLookupFails` — error propagation, not a silent wrong path.
5. Updated the 4 pre-existing resolver tests to pass a `fakeProjectClient` (constructor signature change).

Full `go build ./services/git-gateway-service/...` and `go test ./services/git-gateway-service/...` both pass (all packages, including `internal/usecase`'s existing ~34-usecase test suite, unaffected). `gitnexus detect_changes` confirms the touched-symbol set matches exactly what this change intended (no unexpected blast radius).
