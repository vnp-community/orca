# TASK-AG-TG-002: Add `handleShellExecStream` and wire `shell.execStream`

**Task ID:** TASK-AG-TG-002
**Priority:** 🟡 P2 (CR-TG-006 — task-graph streaming, not yet scheduled)
**Solution Ref:** [SOL-AG-TG-002](../solutions/SOL-AG-TG-002-agent-chunk-streaming.md)
**Depends on:** None in code. TASK-AG-TG-001 reviews and documents the same
`stream.chunk`/`stream.end`/`stream.started` convention this task reuses (both trace back to the
same real precedent, `git.execStream`), but the two handlers touch disjoint files
(`agent-print-mode-exec.ts`/`agent-rpc-dispatch-agent-exec.ts` vs. `fs-agent-extensions.ts`/
`agent-rpc-dispatch-misc.ts`) with no shared function or import between them — either task can be
implemented first, and implementing this one does not require TASK-AG-TG-001 to exist.
**Status:** `[x]` DONE

---

## Execution notes (2026-09-09)

Implemented exactly as sketched: `handleShellExecStream` added to
`fs-agent-extensions.ts` (module-local `sendFrame` helper +
`WebSocket`/`WireState`/`encodeDataFrame` imports added — a second copy, not
a shared factor with `agent-print-mode-exec.ts`'s own `sendFrame`, per the
task's "not required for either to work standalone" note), reusing
`handleShellExec`'s script/traceId/extraEnv/timeoutMs extraction and
`SHELL_EXEC_DEFAULT_TIMEOUT_MS`/`SHELL_EXEC_MAX_TIMEOUT_MS` constants as-is.
`handleShellExec` itself is untouched. `case 'shell.execStream'` added to
`agent-rpc-dispatch-misc.ts` alongside `shell.exec`, using the same
`void handleXxxStream(...)` + literal `{ type: 'stream.started' }` shape.

**Open Question 2 (`_state` → `state` rename) resolved as specified:**
`dispatchMiscRpc`'s last parameter renamed from `_state` to `state`, and its
"Why unused" comment rewritten to explain `shell.execStream` uses it again —
no other case in the switch referenced the old `_state` name (confirmed via
`grep -n "_state" agent-rpc-dispatch-misc.ts` after the rename — the only
remaining hit is the comment's own back-reference to the old name). No
`route()` change needed; `agent-rpc-dispatch.ts:349` already passes `state`
positionally.

**Open Question 3 (chunking granularity) resolved consistently with
TASK-AG-TG-001: raw `data` event verbatim, no line-splitting.** Same
reasoning — simpler, lower latency, avoids holding back a trailing partial
line, and `git.execStream`'s line-buffering is a git-specific concern.
Documented in a code comment above the new function.

**Open Question 1 (output-size cap) resolved: left uncapped, relying on the
caller/connection lifecycle.** Unlike `handleShellExec`, `handleShellExecStream`
never accumulates stdout/stderr into a buffer — each chunk is forwarded via
`sendFrame` and discarded immediately — so `SHELL_EXEC_MAX_OUTPUT_BYTES`'s
memory-growth concern (the reason the non-streaming handler truncates) does
not apply; there is nothing to truncate. Documented in a code comment.

Verify:

```
cd agent && npx tsc --noEmit
  # 53 pre-existing errors in unrelated test files (AgentConfig missing
  # orcaHttpUrl/apiSecret, AgentBinarySpec missing apiKeyEnvVar, ws-headers
  # typing) — none in fs-agent-extensions.ts or agent-rpc-dispatch-misc.ts
cd agent && npx vitest run src/relay/__tests__/fs-agent-extensions.test.ts src/relay/agent-rpc-dispatch-misc.test.ts
  # 53 passed (49 pre-existing tests unmodified + 4 new handleShellExecStream
  # tests using real `sh -c` spawns [stream.chunk×N + stream.end with real
  # exit code, missing-script error frame, SIGKILL-on-timeout → exitCode -1,
  # non-zero exit code propagation] + 2 new dispatchMiscRpc shell.execStream
  # tests [stream.started returned before the fire-and-forget handler
  # settles, ServerError on handler-import failure])
cd agent && pnpm test   # full package suite: 352 files / 4022 tests passed,
                         # 10 pre-existing skips, 0 failures
```

New tests added to `agent/src/relay/__tests__/fs-agent-extensions.test.ts`
(handler, real spawns — no `child_process` mock, consistent with this file's
existing `handlePreflightCheck` tests) and
`agent/src/relay/agent-rpc-dispatch-misc.test.ts` (dispatch case, mocked
handler via `vi.doMock`).

## Context

**Files:**
- `agent/src/relay/fs-agent-extensions.ts` (new `handleShellExecStream`, `handleShellExec`
  untouched — `fs-agent-extensions.ts:541-595` is the current `handleShellExec`)
- `agent/src/relay/agent-rpc-dispatch-misc.ts` (new `case 'shell.execStream'`, alongside the
  existing `case 'shell.exec'` at lines 188-196)

**Problem:** `shell.exec` (`handleShellExec`, `fs-agent-extensions.ts:541-595`) buffers
`child.stdout`/`child.stderr` into strings (capped at `SHELL_EXEC_MAX_OUTPUT_BYTES`, with a
`truncated` flag) and resolves once on `close`. CR-TG-006 needs a streaming sibling for the
workflow `shell` step type so long-running scripts can show live output, without touching
`StepExecutors.executeShell()`'s existing `relay.call('shell.exec', { script, env, traceId })`
contract.

**The precedent this task copies — same one TASK-AG-TG-001 copies, read `agent-git-handler.ts`'s
real `handleGitExecStream` (`agent-git-handler.ts:234-325`) and `agent-rpc-dispatch-git.ts`'s real
`case 'git.execStream'` in full before coding.** Key shape, verbatim from the real code:

```typescript
// agent-rpc-dispatch-git.ts — case 'git.execStream' (the pattern to copy)
case 'git.execStream': {
  try {
    const { handleGitExecStream } = await import('./agent-git-handler')
    void handleGitExecStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
    return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.execStream unavailable: ${msg}`)
  }
}
```

**Real complication this task must resolve, not present for TASK-AG-TG-001:**
`agent-rpc-dispatch-misc.ts`'s `dispatchMiscRpc` currently receives `WireState` as its **last**
parameter but named `_state` (underscore-prefixed, unused) with this comment
(`agent-rpc-dispatch-misc.ts:23-27`):

```typescript
  // Why unused: vm.provision (the one case here that needed WireState for
  // its stream.chunk/stream.end frames) moved to agent-rpc-dispatch-vm.ts
  // (max-lines split). Kept in the signature for shape-consistency with
  // every other dispatchXxxRpc function route() calls positionally
  // (dispatchFsRpc/dispatchBrowserRpc etc. all take the same param set).
  _state: WireState
```

Adding `shell.execStream` here means this file now *does* need `WireState` again — rename the
parameter from `_state` to `state` (drop the underscore) and update the "Why unused" comment
above it, since it stops being true. `route()` in `agent-rpc-dispatch.ts` already passes `state`
positionally into `dispatchMiscRpc(rpc, tools, config, log, ws, state)`
(`agent-rpc-dispatch.ts:349`) — no change needed there, only inside `agent-rpc-dispatch-misc.ts`'s
own signature and the new case body.

## Implementation Sketch

`handleShellExec`'s real body (`fs-agent-extensions.ts:541-595`) spawns via
`spawn('sh', ['-c', script], { env: spawnEnv })` — not `spec.binary`/`args` like the agent-exec
path — and has its own `SHELL_EXEC_DEFAULT_TIMEOUT_MS`/`SHELL_EXEC_MAX_TIMEOUT_MS` timeout
constants and a `SHELL_EXEC_MAX_OUTPUT_BYTES` truncation cap that streaming bypasses (streaming
has no accumulated buffer to cap in the same way — see Open Questions). Reuse the same
`script`/`traceId`/`extraEnv`/`timeoutMs` param extraction and validation
(`fs-agent-extensions.ts:546-559`) as-is:

```typescript
// fs-agent-extensions.ts — new function, handleShellExec is NOT modified
export async function handleShellExecStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig
): Promise<void> {
  const script = typeof params.script === 'string' ? params.script : ''
  const traceId = typeof params.traceId === 'string' ? params.traceId : undefined
  const extraEnv = (params.env && typeof params.env === 'object' && !Array.isArray(params.env))
    ? params.env as Record<string, string>
    : {}
  const timeoutMs = typeof params.timeoutMs === 'number'
    ? Math.min(Math.max(params.timeoutMs, 1_000), SHELL_EXEC_MAX_TIMEOUT_MS)
    : SHELL_EXEC_DEFAULT_TIMEOUT_MS
  const span = fsTracer.start({ method: 'shell.execStream', scriptLen: script.length, traceId })

  if (!script) {
    span.fail('missing param: script', { method: 'shell.execStream' })
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: script' }
    })
    return
  }

  const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
  const child = spawn('sh', ['-c', script], { env: spawnEnv })

  function sendChunk(text: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      result: { type: 'stream.chunk', line: text, ...(source ? { source } : {}) }
    })
  }

  const timer = setTimeout(() => {
    try { child.kill('SIGKILL') } catch { /* ignore */ }
    span.fail('timed out', { timeoutMs })
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode: -1 } })
  }, timeoutMs)

  child.stdout.on('data', (d: Buffer) => sendChunk(d.toString('utf8')))
  child.stderr.on('data', (d: Buffer) => sendChunk(d.toString('utf8'), 'stderr'))
  child.on('close', (code) => {
    clearTimeout(timer)
    span.ok({ exitCode: code ?? 0 })
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode: code ?? 0 } })
  })
  child.on('error', (err) => {
    clearTimeout(timer)
    span.fail(err)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    })
  })
}
```

`fs-agent-extensions.ts` needs the same new imports `handleAgentExecPromptStream` needs in
TASK-AG-TG-001 — `WebSocket` (type), `WireState` (type) and `encodeDataFrame` from
`orca-dev-agent-transport` — plus a local `sendFrame` helper (copy `agent-git-handler.ts`'s
module-local one, or factor a shared helper if both this task and TASK-AG-TG-001 land close
together; not required for either to work standalone).

```typescript
// agent-rpc-dispatch-misc.ts — new case, alongside the existing 'shell.exec'
// case at lines 188-196 (untouched); requires renaming the function's last
// param from `_state` to `state` (see Context above)
case 'shell.execStream': {
  try {
    const { handleShellExecStream } = await import('./fs-agent-extensions')
    void handleShellExecStream(ws, state, rpc.id, rpc.params ?? {}, config)
    return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `shell.execStream unavailable: ${msg}`)
  }
}
```

## Open Questions To Resolve Before Coding

1. **Output-size cap** — `handleShellExec`'s `SHELL_EXEC_MAX_OUTPUT_BYTES` truncation applies to
   the accumulated buffer it returns; a streaming handler has no such buffer. Decide whether to
   cap total bytes streamed per invocation (killing the process past some limit, similar to the
   timeout branch) or leave it uncapped and rely on the caller/connection lifecycle — document
   whichever is chosen.
2. **`_state` → `state` rename fallout** — confirm no other case in `dispatchMiscRpc`'s switch
   silently relied on the parameter being unused (grep the file for `_state` after the rename to
   confirm zero remaining references); update the outdated "Why unused" comment
   (`agent-rpc-dispatch-misc.ts:23-27`) rather than leaving it describing a state that no longer
   holds.
3. **Chunking granularity** — same open question as TASK-AG-TG-001 (raw `data` event vs.
   line-buffered like `handleGitExecStream`); decide independently or consistently with that
   task's resolution, and document either way.

## Tests To Add

Files: `agent/src/relay/__tests__/fs-agent-extensions.test.ts` (handler),
`agent/src/relay/agent-rpc-dispatch-misc.test.ts` (dispatch case)

- `handleShellExecStream` sends one `stream.chunk` frame per stdout/stderr `data` event (or per
  line, per Open Question 3), then exactly one `stream.end` frame with the correct `exitCode`, all
  sharing the request `id` — model this on `agent-ephemeral-vm-handler.test.ts`'s
  `describe('handleVmProvision', ...)` block (`agent-ephemeral-vm-handler.test.ts:208-248`), the
  one real streaming-handler test already in this codebase (local `MockWs` class +
  `orca-dev-agent-transport`'s `createWireState`/`decodeFrame`).
- Missing `script` param sends a single error frame, never a `stream.chunk`/`stream.end`.
- Timeout kills the child and sends `stream.end` (exact `exitCode` value per Open Question 1's
  resolution), mirroring `handleShellExec`'s existing timeout test if one exists in
  `fs-agent-extensions.test.ts` — check before assuming it does.
- `case 'shell.execStream'` in `dispatchMiscRpc` returns `{ type: 'stream.started' }` synchronously
  without awaiting the handler (assert via a handler mock that resolves after the dispatch call
  already returned).
- `handleShellExec`/`case 'shell.exec'` (non-streaming) behavior is provably unchanged — existing
  tests pass with zero modification.

## Verify

```bash
cd /opt/repos/orca/agent && npx tsc --noEmit
cd /opt/repos/orca/agent && npx vitest run src/relay/__tests__/fs-agent-extensions.test.ts src/relay/agent-rpc-dispatch-misc.test.ts
```

Full package suite (per `agent/package.json`'s `"test": "vitest run"` script):

```bash
cd /opt/repos/orca/agent && pnpm test
```

---

## Acceptance Criteria

- [x] `handleShellExecStream` added to `fs-agent-extensions.ts`; `handleShellExec`'s code and
      exported signature are unchanged.
- [x] `case 'shell.execStream'` added to `agent-rpc-dispatch-misc.ts`, following the exact
      `void handleXxxStream(...)` + literal `{ type: 'stream.started' }` return shape used by
      `git.execStream`/`vm.provision`/`agent.spawn` in this codebase today.
- [x] `dispatchMiscRpc`'s `_state` parameter is renamed to `state` and its stale "Why unused"
      comment is updated to reflect that `shell.execStream` now uses it.
- [x] Output-size-cap and chunking-granularity open questions (above) are explicitly decided and
      documented in code comments, not left implicit.
- [x] Existing `shell.exec` callers' tests pass unmodified.
- [x] New tests (above) pass.
