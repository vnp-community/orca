# TASK-227: Expose all 20 unreachable `git.*` methods on the agent's Part A dispatcher

**From Solution:** SOL-036 (design direction) + SOL-032 §0 (full corrected method list)
**Priority:** P0 — **blocks every relay-based task in TASK-206–213** (SOL-032); those tasks produce correct Go code that will fail at runtime with `Method not found` against a real agent until this lands
**Service:** `agent/` (not a `backend-go` service — this is the one task in this whole tasks/ directory that changes the agent process, not backend-go)
**File:** `agent/src/relay/agent-git-handler-extended.ts`, `agent/src/relay/agent-rpc-dispatch.ts`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Scope correction from the first version of this task

The first pass at this task only covered `git.status`/`git.diff`/`git.commit`
(3 methods). Tracing the FULL data flow for every method `SOL-032` plans
to add found **20 methods** unreachable on Part A, not 3 — see
`SOL-032`'s §0 correction table for the complete classification (which 8
methods are already reachable and don't need this task; which need only a
param rename vs. a genuine backend-go-side redesign — this task only
covers agent-side *reachability*, not those redesigns).

**8 methods do NOT need this task** — already reachable via Part A's
existing re-exports: `history`, `branchCompare`, `commitCompare`,
`branchDiff`, `commitDiff`, `checkIgnored`, `forkSync`, `submoduleStatus`.

**20 methods need this task.** All 20 already exist, fully implemented and
validated, on Part B (`agent/src/relay/relay.ts` + `dispatcher.ts`'s
`RelayDispatcher`, via `GitHandler` and its ops modules) — this task is
**re-exporting already-built logic through routing**, the exact pattern
Part A already uses for the 8 methods above (see
`agent-git-handler-extended.ts`'s existing `handleGitHistory`/
`handleGitBranchCompare`/etc.), not writing new git logic.

---

## The 20 methods, grouped to match `SOL-032`'s proposal structure

| # | Method | Real params | Part B handler (delegate) | SOL-032 group |
|---|---|---|---|---|
| 1 | `git.status` | `worktreePath, includeIgnored?, limit?, bypassEffectiveUpstreamNegativeCache?` | `getStatusOp` (`git-handler-status-ops.ts:59`) | Part 1 (already-implemented Go side) |
| 2 | `git.diff` | `worktreePath, filePath, staged, compareAgainstHead?` | `computeDiff` (`git-handler-ops.ts:77`) | Part 1 |
| 3 | `git.commit` | `worktreePath, message` | `commitChangesRelay` (`git-handler-worktree-ops.ts:153`) | Part 1 |
| 4 | `git.push` | `worktreePath, pushTarget?, forceWithLease?, publish` | `push` (`relay.ts:1103`) → `resolveRelayPushTarget` (`git-handler-push-target.ts:159`) | Part 1 / Group D |
| 5 | `git.pull` | `worktreePath, pushTarget?` | `pull`→`pullWithArgs` (`relay.ts:1133/1169`) | Part 1 / Group D |
| 6 | `git.checkout` | `worktreePath, branch` | `checkout` (`relay.ts:687`) | Group A |
| 7 | `git.localBranches` | `worktreePath` | `localBranches` (`relay.ts:706`) | Group A |
| 8 | `git.fastForward` | `worktreePath, pushTarget?` | `fastForward`→`pullWithArgs(['--ff-only'])` (`relay.ts:1175`) | Group A |
| 9 | `git.rebaseFromBase` | `worktreePath, baseRef` | `rebaseFromBase` (`relay.ts:1179`) → `resolveGitRemoteRebaseSource` | Group A |
| 10 | `git.abortRebase` | `worktreePath` | `abortRebase` (`relay.ts:677`) | Group A |
| 11 | `git.abortMerge` | `worktreePath` | `abortMerge` (`relay.ts:667`) | Group A |
| 12 | `git.conflictOperation` | `worktreePath` | `conflictOperation` (`relay.ts:871`) → `detectConflictOperation` (`git-handler-status-ops.ts:38`) | Group A — **detector only**, see SOL-032 §0 open question #2 for why the Go-side RPC design needs rework, not this task |
| 13 | `git.discard` | `worktreePath, filePath` | `discard` (`relay.ts:760`) → `removeSafeUntrackedDiscardTarget` | Group A |
| 14 | `git.bulkDiscard` | `worktreePath, filePaths[]` | `bulkDiscard` (`relay.ts:794`) | Group A |
| 15 | `git.stage` | `worktreePath, filePath` | `stage` (`relay.ts:590`) | Group B |
| 16 | `git.unstage` | `worktreePath, filePath` | `unstage` (`relay.ts:622`) | Group B |
| 17 | `git.bulkStage` | `worktreePath, filePaths[]` | `bulkStage` (`relay.ts:633`) | Group B |
| 18 | `git.bulkUnstage` | `worktreePath, filePaths[]` | `bulkUnstage` (`relay.ts:650`) | Group B |
| 19 | `git.fetch` | `worktreePath, pushTarget?` | `fetch` (`relay.ts:954`) | Group D |
| 20 | `git.upstreamStatus` | `worktreePath, pushTarget?` | `upstreamStatus` (`relay.ts:906`) → shared `git-publish-target-status`/`git-effective-upstream` | Group C |

## Changes to make

### `agent-git-handler-extended.ts` — add 20 re-export handlers

Follow the exact pattern the file already uses for its 8 existing
re-exports (`handleGitHistory` etc.) — decode `rpc.params`, call the
delegate, wrap success/error as a `JsonRpcResponse`. Three representative
sketches (the other 17 are the same shape with different delegates/params
per the table above):

```typescript
import { getStatusOp } from './git-handler-status-ops'
import { computeDiff } from './git-handler-ops'
import { commitChangesRelay } from './git-handler-worktree-ops'
import { push, pull, checkout, localBranches, fastForward, rebaseFromBase,
         abortRebase, abortMerge, conflictOperation, discard, bulkDiscard,
         stage, unstage, bulkStage, bulkUnstage, fetch, upstreamStatus } from './relay-git-ops' // extract these 17 from relay.ts's inline handlers into an importable module first — see "Prerequisite refactor" below

export async function handleGitStatus(id: JsonRpcId, params: unknown): Promise<JsonRpcResponse> {
  const p = params as { worktreePath: string; includeIgnored?: boolean; limit?: number; bypassEffectiveUpstreamNegativeCache?: boolean }
  try {
    return makeSuccess(id, await getStatusOp(p))
  } catch (err: unknown) {
    return makeError(id, AgentErrorCode.ServerError, err instanceof Error ? err.message : String(err))
  }
}

export async function handleGitDiff(id: JsonRpcId, params: unknown): Promise<JsonRpcResponse> {
  // filePath is REQUIRED — backend-go's current RelayExecutor.GetDiff
  // doesn't send one yet; that's TASK-228's fix, not this task's problem.
  const p = params as { worktreePath: string; filePath: string; staged?: boolean; compareAgainstHead?: boolean }
  try {
    return makeSuccess(id, await computeDiff(p))
  } catch (err: unknown) {
    return makeError(id, AgentErrorCode.ServerError, err instanceof Error ? err.message : String(err))
  }
}

export async function handleGitConflictOperation(id: JsonRpcId, params: unknown): Promise<JsonRpcResponse> {
  // Detector only — returns 'merge'|'rebase'|'cherry-pick'|'unknown'. Does
  // NOT take path/operation params, unlike SOL-032's original (incorrect)
  // Go-side design assumed. See SOL-032 §0 open question #2.
  const p = params as { worktreePath: string }
  try {
    return makeSuccess(id, await conflictOperation(p))
  } catch (err: unknown) {
    return makeError(id, AgentErrorCode.ServerError, err instanceof Error ? err.message : String(err))
  }
}
```

Repeat for the remaining 17 methods per the table's delegate column,
following this exact try/catch/`makeSuccess`/`makeError` shape.

### Prerequisite refactor — Part B's non-ops-module handlers

8 of the 20 (`push`, `pull`, `checkout`, `localBranches`, `fastForward`,
`rebaseFromBase`, `abortRebase`, `abortMerge`, `conflictOperation`,
`discard`, `bulkDiscard`, `stage`, `unstage`, `bulkStage`, `bulkUnstage`,
`fetch`, `upstreamStatus` — i.e. everything except `status`/`diff`/`commit`)
are implemented as **inline handler functions directly inside `relay.ts`**
(per the Part B method table's "Handler" column, e.g. `push @1103`), not
in a separate, independently-importable ops module the way `status`/`diff`/
`history`/etc. already are. Before this task's re-export handlers can
import them, extract each into an importable function (a mechanical
refactor — move the function body, keep the logic identical, export it —
not a rewrite) into a shared module (e.g. `relay-git-ops.ts`, or split
across the existing `git-handler-*-ops.ts` files by category to match the
codebase's existing per-concern module split). This is real prerequisite
work, not a formality — confirm the actual current file structure before
assuming a one-line import will work.

### `agent-rpc-dispatch.ts` — register the 20 new cases

Add next to the existing 8 `git.*` cases:

```typescript
case 'git.status':
  return handleGitStatus(rpc.id, rpc.params)
case 'git.diff':
  return handleGitDiff(rpc.id, rpc.params)
case 'git.commit':
  return handleGitCommit(rpc.id, rpc.params, clientId) // needs clientId for getClientGitIdentity — confirm the variable already in scope here
case 'git.push':
  return handleGitPush(rpc.id, rpc.params)
case 'git.pull':
  return handleGitPull(rpc.id, rpc.params)
case 'git.checkout':
  return handleGitCheckout(rpc.id, rpc.params)
case 'git.localBranches':
  return handleGitLocalBranches(rpc.id, rpc.params)
case 'git.fastForward':
  return handleGitFastForward(rpc.id, rpc.params)
case 'git.rebaseFromBase':
  return handleGitRebaseFromBase(rpc.id, rpc.params)
case 'git.abortRebase':
  return handleGitAbortRebase(rpc.id, rpc.params)
case 'git.abortMerge':
  return handleGitAbortMerge(rpc.id, rpc.params)
case 'git.conflictOperation':
  return handleGitConflictOperation(rpc.id, rpc.params)
case 'git.discard':
  return handleGitDiscard(rpc.id, rpc.params)
case 'git.bulkDiscard':
  return handleGitBulkDiscard(rpc.id, rpc.params)
case 'git.stage':
  return handleGitStage(rpc.id, rpc.params)
case 'git.unstage':
  return handleGitUnstage(rpc.id, rpc.params)
case 'git.bulkStage':
  return handleGitBulkStage(rpc.id, rpc.params)
case 'git.bulkUnstage':
  return handleGitBulkUnstage(rpc.id, rpc.params)
case 'git.fetch':
  return handleGitFetch(rpc.id, rpc.params)
case 'git.upstreamStatus':
  return handleGitUpstreamStatus(rpc.id, rpc.params)
```

## Verify

```bash
cd /opt/repos/orca/agent
npm run typecheck
npm test -- agent-git-handler-extended
```

Then, against a real or faithfully faked agent connection (via
`infra-fleet-service`'s `Relay` RPC): confirm all 20 methods return real
data instead of `Method not found`. This is the regression test that
actually catches what unit tests (fake gRPC clients) miss — BUG-036 was
invisible to this project's existing test suite for exactly that reason.

## What this task does NOT do

It makes the real Part B contract *reachable*. It does not fix
`git-gateway-service`'s Go-side request/response shapes to match that
contract — that's [TASK-228](./TASK-228-fix-existing-relay-param-names-and-diff-shape.md)
for the 5 already-implemented methods, and SOL-032 §0's per-group
correction notes for the rest (several of which — `push`/`pull`/
`fastForward`/`fetch`'s `pushTarget`, `conflictOperation`'s real
detector-only shape, `submoduleStatus`'s per-submodule granularity,
`localBranches`'s narrower response — need a design decision before
`TASK-207`/`TASK-210` can be implemented as originally written).
