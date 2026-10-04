# BUG-020: `git.status` wscompat channel returns the raw `GetStatusResponse` proto (`files`/`branch`) instead of the frontend's expected `GitStatusResult` shape (`entries`/`branch`/`head`/`upstreamStatus`) — crashes `GitPanel`

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels.go` (`registerGitChannels`'s `git.status` handler)
**Severity:** 🔴 Critical — crashes the entire Git tab render as soon as `git.status` ever succeeds (which it never did before SOL-013/014 — this is the next layer in that same investigation chain)
**Status:** ✅ Root cause CONFIRMED via live browser error + source read. ✅ Fix (Phase 1, crash-stopping) implemented + unit-tested (build/vet/test all pass, `detect_changes`/`impact` clean, LOW risk), ✅ **deployed** (`2026.09.15-gitstatus-shape-fix`). Phase 2 (real staged/unstaged tracking) spec'd but not implemented — see below.

## Symptom (live, user-reported)

```
Unhandled Promise Rejection: TypeError: s.entries is not iterable
    at K (GitPanel-CCC1rPeI.js:2:1062)
```
Reported immediately after CR-PW-012's fix let a worktree selection actually land on (and render) the Git tab for the first time with real backend data.

## Root cause — confirmed via source read, both sides

1. `frontend/src/renderer/src/hooks/useGit.ts:41`:
   ```ts
   for (const entry of status.entries) { ... }
   ```
   No null-guard — assumes `status.entries` is always a real array (`GitStatusResult.entries: GitStatusEntry[]`, `frontend/src/shared/git-status-types.ts:56`).
2. `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:522-542` (`git.status`):
   ```go
   resp, err := client.GetStatus(ctx, &gitgatewayv1.GetStatusRequest{...})
   ...
   return resp, nil
   ```
   Returns the **raw gRPC proto response** with zero field mapping. `gitgatewayv1.GetStatusResponse` (`proto/orca/gitgateway/v1/gitgateway.proto:191-194`) only has:
   ```proto
   message GetStatusResponse {
     repeated FileStatus files = 1;
     string branch = 2;
   }
   ```
   — field name **`files`**, not `entries`; **no `head`, no `upstreamStatus` fields at all**. So the JSON actually sent to the frontend is `{"files": [...], "branch": "..."}`, and `status.entries` is `undefined` — `for (const entry of undefined)` throws exactly `TypeError: entries is not iterable` (minified to `s.entries is not iterable`).

This is **not a "sometimes empty" bug** (like BUG-018's nil-slice-encodes-as-null) — it's a **complete, unconditional field-name/shape mismatch that has existed since `git.status` was first wired**, never caught before because `GetStatus` never successfully returned *any* response until SOL-013/SOL-014 fixed the underlying dispatch/relay chain earlier in this same session.

## Secondary gap found in the same area (real, but bigger scope — see "Phase 2" below)

`gitgatewayv1.FileStatus` (`{path, state}`) has **no staged/unstaged ("area") concept at all**, but the frontend's `GitStatusEntry` (`{path, status, area: 'staged'|'unstaged'}`) requires one — `StagingArea.tsx`/`useGit.ts`'s `toGitFileChanges` splits changes into staged vs. unstaged columns by `entry.area`. `localgit/executor.go`'s `parsePorcelainStatus` already extracts the full 2-character porcelain `XY` code (`code := line[:2]`) but `fileStateFromPorcelainCode` collapses it into a single `FileState`, discarding the X (index/staged) vs Y (worktree/unstaged) distinction porcelain v1 already carries — the raw data needed IS available, just never threaded through the domain model.

## Fix — Phase 1 (implemented, stops the crash)

Map `resp` into the frontend's expected shape in the `git.status` wscompat handler instead of returning the raw proto: `files`→`entries` (renamed), `state`→`status` per entry, `area` defaulted to `"unstaged"` for every entry (git-gateway-service doesn't distinguish staged files yet — see Phase 2), `head`/`upstreamStatus` omitted (the frontend's own `toWorkspaceGitStatus` already treats missing `upstreamStatus` as `ahead: 0, behind: 0`, and a non-empty `branch` from a real response means `describeBranch` never reaches its `head`-dependent branches).

**Known limitation of Phase 1**: every changed file will show as "unstaged" regardless of its real staged state — the crash is fixed and the file list/counts are accurate, but the staged/unstaged split in `StagingArea.tsx` is not yet accurate. Flagged clearly, not silently left broken-but-hidden.

## Fix — Phase 2 (spec'd, not implemented — needs its own pass)

Add real staged/unstaged tracking through the full chain:
1. `domain.FileStatus` (git-gateway-service): add an `Area` field (`"staged"|"unstaged"`).
2. `localgit/executor.go`'s `parsePorcelainStatus`/`fileStateFromPorcelainCode`: derive `Area` from the XY code's X (staged) vs Y (unstaged) position — porcelain v1 can report a SINGLE path with BOTH a staged AND unstaged change simultaneously (e.g. `"MM"` — staged modification, then further unstaged modification on top) — properly representing this may require emitting up to 2 domain.FileStatus rows per porcelain line, one per non-space side, not just 1.
3. **The relay/agent path needs its own check** — `RelayExecutor.GetStatus` (`relay_executor.go:252-257`) unmarshals the Dev Server Agent's `git.status` JSON-RPC response directly into `domain.GitStatus`/`domain.FileStatus` — even if the agent's own response already computed something richer, Go's `json.Unmarshal` silently drops unknown fields, so this Go-side struct is the actual bottleneck for BOTH the local and relay paths. The agent's own `git.status` handler (not traced in this pass) would need to independently confirm it can supply equivalent per-file area info, or itself needs the same porcelain-XY-based fix.
4. `gitgatewayv1.FileStatus` proto: add `area` field; `gitgatewayv1.GetStatusResponse`: consider whether ahead/behind (`upstreamStatus`) and detached-HEAD (`head`) info should also be added here, matching the frontend's fuller `GitStatusResult` contract, or whether the frontend should keep degrading gracefully without them (currently does, via `?? 0`/`describeBranch`'s branch-first checks) — **product/scope decision, not made in this pass**.
5. Update `channels.go`'s `git.status` mapping (Phase 1's code) to pass through the real `area` per entry instead of the hardcoded `"unstaged"` default.

## Testing

- **Phase 1**: unit test for the `git.status` wscompat handler asserting the JSON response has `entries` (not `files`), each entry has `status`+`area`, matching the frontend's `GitStatusResult`/`GitStatusEntry` shape exactly (field names, not just presence).
- **Phase 2** (when implemented): `localgit/executor_test.go` cases for porcelain codes with a non-space X (e.g. `"M "`, `"A "`) asserting `Area: staged`, vs. a non-space-only-Y code (e.g. `" M"`) asserting `Area: unstaged`, and a both-sides-dirty code (e.g. `"MM"`) asserting 2 emitted rows.

## Related

- [SOL-013](./solutions/SOL-013-connection-resolver-worktree-path-fallback.md) / [SOL-014](./solutions/SOL-014-connection-resolver-relay-via-dev-server-reachability.md) — the fixes that made `GetStatus` succeed for the first time, exposing this bug
- [CR-PW-012](../../../docs/crs/v3/project-workspace/CR-PW-012-worktree-click-forces-terminal-view-out-of-beta-workspace.md) — the frontend navigation fix that let a user actually reach the Git tab and trigger this crash
- [BUG-018](./BUG-018-jira-linear-labels-null-not-empty-array.md) — a related but distinct class of wscompat-layer wire-shape bug (nil-vs-empty), found earlier in the same session
