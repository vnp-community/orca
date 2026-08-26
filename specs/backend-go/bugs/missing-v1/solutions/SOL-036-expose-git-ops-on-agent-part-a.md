# SOL-036: Re-expose Part B's git ops modules through Part A's dispatcher

**Resolves:** [BUG-036](../BUG-036-git-relay-methods-unreachable-on-agent.md)
**Service:** `agent/` (Part A dispatcher) — the one solution in this
directory whose fix lives entirely outside `backend-go/`, because the
defect it resolves is a routing gap in the agent process backend-go's
transport already talks to, not a missing capability in `backend-go`
itself.
**Affected files (proposed):**
- `agent/src/relay/agent-rpc-dispatch.ts`
- (no changes to `git-handler-status-ops.ts`, `git-handler-ops.ts`,
  `git-handler-worktree-ops.ts`, etc. — this reuses that logic verbatim)
**Status:** 📋 Proposed — not yet implemented

---

## Why this is the right fix, not just *a* fix

`BUG-036` names two directions. This solution picks **direction 1**
(extend Part A) over direction 2 (redesign `RelayExecutor` around
`git.exec`), because Part A **already does exactly this** for 8 other
git methods — `git.history`/`branchCompare`/`commitCompare`/`branchDiff`/
`commitDiff`/`checkIgnored`/`forkSync`/`submoduleStatus` are all
implemented in `agent-git-handler-extended.ts` as thin re-exports of Part
B's ops modules (`git-handler-ops.ts`, `git-handler-commit-diff-ops.ts`,
`git-handler-check-ignore.ts`, `git-handler-status-ops.ts`,
`git-handler-submodule-ops.ts`), per
`specs/agent/api/agent-rpc-catalog-git-fs.md`'s own description. This
solution is "do the same thing for 6 more methods," not a new pattern —
lower risk than direction 2, which would duplicate Part B's parsing logic
a second time inside `git-gateway-service` in Go.

`specs/agent/api/gaps-and-findings.md` §4 already flagged "porting Part
B's ~20 dedicated per-operation RPCs to Part A" as a real, scoped fix that
was deferred as "higher-risk, not attempted" in that pass — this solution
is the narrower slice of that work `backend-go` actually needs right now
(6 methods, not 20), justified by backend-go's architecture making it a
hard blocker rather than an aspirational cleanup.

---

## Methods to add to `agent-rpc-dispatch.ts`

| Method | Reuses | Notes |
|---|---|---|
| `git.status` | `getStatusOp` (`git-handler-status-ops.ts:59`) | matches `agent-git-handler-extended.ts`'s existing import style for the 8 already-ported methods |
| `git.diff` | `computeDiff` (`git-handler-ops.ts:77`) | **per-file** — see "Companion fix" below |
| `git.stage` / `git.unstage` / `git.bulkStage` / `git.bulkUnstage` | Part B's inline handlers (`relay.ts` lines 590/622/633/650) — these aren't in a separate ops module like the others, so this task also needs to extract them into one (mirroring how `checkIgnoredPathsOp` was already extracted for a Part-B-only handler) | `git-gateway-service` doesn't call these directly yet (no `Stage`/`Unstage` usecase exists) — add the agent-side handler now anyway, since `SOL-032`'s staging-operations task group will need it |
| `git.commit` | `commitChangesRelay` (`git-handler-worktree-ops.ts:153`) | **params differ from `RelayExecutor`'s current call** — real contract is `{worktreePath, message}` only, no `paths` array (Part B assumes already-staged via a prior `git.stage` call) — `RelayExecutor.Commit`'s Go signature needs the same fix (drop `paths`, or call `git.stage`/`git.bulkStage` first) |
| `git.push` / `git.pull` | not found in either Part A or Part B's confirmed method tables in `agent-rpc-catalog-git-fs.md` — **these may not exist as dedicated RPCs on the agent at all**, even on Part B | Verify against the live `agent/` source before assuming Part B has these; if not, they need to be built new (via `git.exec` with `push`/`pull` args, matching `ALLOWED_GIT_SUBCOMMANDS`), not just re-exposed |

## Sketch — the re-export pattern (mirrors the 8 existing ports exactly)

```typescript
// agent-git-handler-extended.ts — add alongside the existing 8 re-exports
import { getStatusOp } from './git-handler-status-ops'
import { computeDiff } from './git-handler-ops'
import { commitChangesRelay } from './git-handler-worktree-ops'

export async function handleGitStatus(id: JsonRpcId, params: unknown): Promise<JsonRpcResponse> {
  const { worktreePath, includeIgnored, limit, bypassEffectiveUpstreamNegativeCache } = params as GitStatusParams
  const result = await getStatusOp({ worktreePath, includeIgnored, limit, bypassEffectiveUpstreamNegativeCache })
  return makeSuccess(id, result)
}

export async function handleGitDiff(id: JsonRpcId, params: unknown): Promise<JsonRpcResponse> {
  const { worktreePath, filePath, staged, compareAgainstHead } = params as GitDiffParams
  const result = await computeDiff({ worktreePath, filePath, staged, compareAgainstHead })
  return makeSuccess(id, result)
}

export async function handleGitCommit(id: JsonRpcId, params: unknown, clientId: string): Promise<JsonRpcResponse> {
  const { worktreePath, message } = params as GitCommitParams
  const result = await commitChangesRelay({ worktreePath, message, clientId })
  return makeSuccess(id, result)
}
```

```typescript
// agent-rpc-dispatch.ts — add cases next to the existing git.history/branchCompare/etc.
case 'git.status':
  return handleGitStatus(rpc.id, rpc.params)
case 'git.diff':
  return handleGitDiff(rpc.id, rpc.params)
case 'git.commit':
  return handleGitCommit(rpc.id, rpc.params, clientId)
// git.stage/unstage/bulkStage/bulkUnstage similarly, once extracted per the table above
```

---

## Companion fix required in `backend-go` — `GetDiff`'s per-file shape

Once `git.diff` is reachable, `RelayExecutor.GetDiff` still sends the
wrong params (`{repoPath, staged}`, no `filePath`) and expects the wrong
response shape (whole-repo `domain.DiffResult` vs. Part B's single-file
diff object). This solution's agent-side fix does **not** resolve that —
it's a separate `git-gateway-service` change:

```go
// usecase/ports.go — GitExecutor.GetDiff needs a filePath param
GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error)
```

`GetDiff`'s usecase (currently returning one `DiffResult` for the whole
repo) needs to become either (a) a per-file API matching the frontend's
actual `git.diff` call shape (check `rpc-catalog.md`'s `git.diff` call
sites — `DiffViewer.tsx` diffs one file at a time in the UI, so a
per-file RPC may already be the right shape and it's `git-gateway-service`'s
current whole-repo assumption that's wrong, not the agent's contract), or
(b) a new usecase that calls `git.status` first to enumerate files, then
`git.diff` per file, and assembles a whole-repo result — check which one
the frontend's real call site expects before choosing. Flagged here as a
required follow-up task, not designed in full — resolving BUG-036's
reachability problem is this solution's scope; this shape question is
adjacent but needs its own investigation into the frontend's actual
`DiffViewer.tsx`/`use-code-review.ts` call shape first.

---

## Test plan

- Agent-side: unit tests for the 3 new `handleGit*` functions (mirror
  `agent-git-handler-extended.test.ts`'s existing coverage of the 8
  ported methods — same params-in/ops-called/response-out shape).
- Integration: a real (or faithfully faked) agent process reachable over
  `direct-websocket` mode, exercised via `infra-fleet-service`'s `Relay`
  RPC end-to-end — assert `git.status`/`git.diff`/`git.commit` return
  real data instead of `MethodNotFound`. This is the regression test that
  actually catches what unit tests (fake gRPC clients) miss — BUG-036 was
  invisible to this project's existing test suite for exactly that reason.

## References

- [BUG-036](../BUG-036-git-relay-methods-unreachable-on-agent.md) — full trace of the defect this fixes
- `agent/src/relay/agent-git-handler-extended.ts` — the existing 8-method re-export precedent
- `specs/agent/api/agent-rpc-catalog-git-fs.md` — Part B's real `git.status`/`git.diff`/`git.commit` contracts
- `specs/agent/api/gaps-and-findings.md` §4 — the pre-existing, deferred "port Part B RPCs to Part A" recommendation this narrows and completes for the methods `backend-go` actually needs
