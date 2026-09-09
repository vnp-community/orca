# TASK-AG-TG-001: Add `handleAgentExecPromptStream` and wire `agent.execPromptStream`

**Task ID:** TASK-AG-TG-001
**Priority:** 🟡 P2 (CR-TG-006 — task-graph streaming, not yet scheduled)
**Solution Ref:** [SOL-AG-TG-002](../solutions/SOL-AG-TG-002-agent-chunk-streaming.md)
**Depends on:** None (self-contained; see TASK-AG-TG-002 for the sibling `shell.execStream`
task, which shares this task's reviewed pattern but has no code dependency on it — see that
task's Context for the exact relationship).
**Status:** [ ] TODO

---

## Context

**Files:**
- `agent/src/relay/agent-print-mode-exec.ts` (new `handleAgentExecPromptStream`,
  `handleAgentExecPrompt` untouched)
- `agent/src/relay/agent-rpc-dispatch-agent-exec.ts` (new `case 'agent.execPromptStream'`,
  alongside the existing `case 'agent.execPrompt'` at lines 176-189)

**Problem:** `agent.execPrompt` (`handleAgentExecPrompt`, `agent-print-mode-exec.ts:33-178`)
buffers `child.stdout`/`child.stderr` into strings and resolves exactly once on `close`
— no incremental delivery to the caller. CR-TG-006 needs a streaming variant for
`task-service`'s complex executor so the caller can show live output while a step runs,
without touching the buffer-then-return contract every existing caller
(`task-service.SimpleExecutor`, `ProfileAwareAgentSpawner.spawn()`, `StepExecutors.executeAgent()`)
already depends on.

**The precedent this task copies — read in full before coding:** `agent-git-handler.ts`'s real
`handleGitExecStream` (`agent-git-handler.ts:234-325`) and its dispatch site,
`agent-rpc-dispatch-git.ts`'s real `case 'git.execStream'`:

```typescript
// agent-rpc-dispatch-git.ts — case 'git.execStream' (verbatim, current code)
case 'git.execStream': {
  try {
    const { handleGitExecStream } = await import('./agent-git-handler')
    // Streaming: fire-and-forget, sends multiple frames asynchronously
    void handleGitExecStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
    return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.execStream unavailable: ${msg}`)
  }
}
```

**Correction vs. SOL-AG-TG-002's phrasing:** the solution doc says *"the dispatcher sends the
initial `stream.started` response before calling the handler"* (paraphrasing
`handleGitExecStream`'s own doc comment). Reading the real dispatch site shows the actual order
is: the dispatcher calls the handler first via `void handleXxxStream(...)` (fire-and-forget —
this synchronously runs the handler up to its first `await`, then returns a pending promise the
dispatcher does not await), and only then returns the `{ type: 'stream.started' }` response
value. Functionally equivalent to "sent before any stream frame arrives" (the response frame and
any stream frames the handler's synchronous prefix might emit are both encoded by the same
`ws.send` call site in `agent-rpc-dispatch.ts`'s `dispatch()`, which processes one RPC at a time),
but the mechanism is "fire the promise, then return a literal value" — not two sequential sends.
Use the same `void handleAgentExecPromptStream(...)` + literal `stream.started` return shape,
not two separate `sendFrame` calls.

`handleGitExecStream`'s real signature and frame shape (`agent-git-handler.ts:244-290`):

```typescript
export async function handleGitExecStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<void> {
  // ...
  function sendChunk(line: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      result: { type: 'stream.chunk', line, ...(source ? { source } : {}) }
    })
  }
  function sendEnd(exitCode: number): void {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } })
  }
  // ...
}

// module-local helper, not exported:
function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}
```

`handleGitExecStream` line-buffers (`chunk.toString('utf8').split('\n').filter(...).forEach(...)`)
before calling `sendChunk`. SOL-AG-TG-002's own sketch for `handleAgentExecPromptStream` sends
each raw `data` event verbatim instead (no line-splitting) — this is a real divergence from the
precedent, not a mismatch to fix silently. Decide explicitly (see Open Questions) rather than
copying line-buffering by default just because the precedent does it.

## Implementation Sketch

`agent-print-mode-exec.ts` currently has no `WebSocket`/`WireState` imports and no local
`sendFrame` helper — both must be added (mirroring `agent-git-handler.ts`'s imports:
`import type WebSocket from 'ws'`, `import { encodeDataFrame } from 'orca-dev-agent-transport'`,
`import type { WireState } from 'orca-dev-agent-transport'`). Everything else —
`resolveAgentSpec`, `buildAgentEnv`, the `claude`-only model gate, the `--print <prompt>` arg
construction, the `trustPresetFull` YOLO flag — is identical setup to `handleAgentExecPrompt`
(`agent-print-mode-exec.ts:75-121`) and should be reused as-is, not reimplemented:

```typescript
// agent-print-mode-exec.ts — new function, handleAgentExecPrompt is NOT modified
export async function handleAgentExecPromptStream(
  ws: WebSocket,
  wireState: WireState,
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<void> {
  // ...identical prompt/worktreePath/stepId/trustPreset/model/accountId/extraEnv/timeoutMs
  // extraction, spec resolution, and unsupported-model gate as handleAgentExecPrompt...
  // Validation failures (missing prompt/worktreePath, unsupported model) use sendFrame with
  // an `error` field, same as handleGitExecStream's validation-error branch — NOT a thrown
  // exception, since this function returns void and has no caller to catch it.

  let env: Record<string, string>
  try {
    env = await buildAgentEnv(
      { accountId, userId: '', taskId: stepId ?? '', cwd: worktreePath, model: modelId, extraEnv },
      spec, config, null, log, span.id
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.PermissionDenied, message: msg }
    })
    return
  }

  const child = spawn(spec.binary, args, {
    cwd: worktreePath,
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe']
  })

  function sendChunk(text: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      result: { type: 'stream.chunk', line: text, ...(source ? { source } : {}) }
    })
  }

  child.stdout?.on('data', (d: Buffer) => sendChunk(d.toString('utf8')))
  child.stderr?.on('data', (d: Buffer) => sendChunk(d.toString('utf8'), 'stderr'))
  child.on('close', (code) => {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode: code ?? -1 } })
  })
  child.on('error', (err) => {
    sendFrame(ws, wireState, {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.ServerError, message: err.message }
    })
  })
}

function sendFrame(ws: WebSocket, wireState: WireState, payload: object): void {
  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(wireState, JSON.stringify(payload)))
  }
}
```

Note: `handleAgentExecPrompt` has no `timeoutMs` kill-and-report-partial-output path exposed via
`sendFrame` in this sketch — decide whether the streaming variant keeps the same
`DEFAULT_TIMEOUT_MS`/`MAX_TIMEOUT_MS` timeout-then-`SIGKILL` behavior (it should, for parity) and
emit `stream.end` with the killed process's synthetic exit code, same as `handleGitExecStream`
has no equivalent timeout at all today (git.execStream relies on the caller/connection lifecycle,
not an internal timer) — this is a real difference between the two precedents; pick one
explicitly and document the choice in code, don't let it default silently.

```typescript
// agent-rpc-dispatch-agent-exec.ts — new case, alongside the existing 'agent.execPrompt'
// case at lines 176-189 (untouched)
case 'agent.execPromptStream': {
  try {
    const { handleAgentExecPromptStream } = await import('./agent-print-mode-exec')
    void handleAgentExecPromptStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
    return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.execPromptStream unavailable: ${msg}`)
  }
}
```

`dispatchAgentExecRpc` already receives `ws`/`state` as parameters (`agent-rpc-dispatch-agent-exec.ts:15-21`)
and already uses the same `void handleXxx(...)` + literal `stream.started`/`spawn.accepted` return
shape for `agent.spawn` (lines 24-34) — no new parameters need to be threaded through `route()`
in `agent-rpc-dispatch.ts`.

## Open Questions To Resolve Before Coding

1. **Chunking granularity** — raw `data` event verbatim (SOL-AG-TG-002's sketch, shown above) vs.
   line-buffered like `handleGitExecStream` (see "Correction" note above). Raw-chunk is simpler
   and matches the solution doc; line-buffering matches the one real precedent this task is
   supposed to copy. Decide and document in a code comment — do not silently pick one.
2. **Timeout behavior** — does `handleAgentExecPromptStream` keep `handleAgentExecPrompt`'s
   `DEFAULT_TIMEOUT_MS`/`MAX_TIMEOUT_MS`/`SIGKILL`-on-timeout logic (recommended, for parity with
   the non-streaming sibling), or rely purely on connection lifecycle like `git.execStream` does?
   If kept, what `stream.end` `exitCode` value represents "killed by timeout" (`handleAgentExecPrompt`
   uses `exitCode: null` in its final object — `stream.end`'s shape only has a `number` `exitCode`
   field per the established convention, so `null` needs a sentinel, e.g. `-1`, same as
   `handleGitExecStream`'s `code ?? 0` / this sketch's `code ?? -1` fallback for a killed/errored
   process).
3. Confirm no other file constructs `PrintModeExecResult` or otherwise imports
   `agent-print-mode-exec.ts` in a way that would be affected by adding a second exported
   function to the file — a plain `grep -rn "agent-print-mode-exec"` across `agent/src` before
   merging is sufficient here; this file has few consumers (currently only
   `agent-rpc-dispatch-agent-exec.ts` imports from it via dynamic `import()`).

## Tests To Add

File: `agent/src/relay/agent-print-mode-exec.test.ts` (existing suite for this module)

- `handleAgentExecPromptStream` sends one `stream.chunk` frame per stdout/stderr `data` event (or
  per line, depending on Open Question 1's resolution), then exactly one `stream.end` frame with
  the correct `exitCode`, all sharing the request `id` — follow the real precedent test already in
  this codebase, `agent-ephemeral-vm-handler.test.ts`'s `describe('handleVmProvision', ...)` block
  (`agent-ephemeral-vm-handler.test.ts:208-248`), which uses a local `MockWs` class and
  `orca-dev-agent-transport`'s `createWireState`/`decodeFrame` to capture and decode every frame a
  handler sends, then asserts on `frames[n].result.type`/`.line`/`.exitCode` in order. There is no
  existing `handleGitExecStream` test to copy instead — `__tests__/agent-git-handler.test.ts` does
  not cover `git.execStream` at all today, so `handleVmProvision`'s test is the only real streaming
  test in this codebase to model this task's tests on.
- Validation failure (missing `prompt`/`worktreePath`, unsupported model) sends a single error
  frame via `sendFrame`, never a `stream.chunk`/`stream.end`.
- `handleAgentExecPrompt` (non-streaming) behavior is provably unchanged — existing tests in
  `agent-print-mode-exec.test.ts` pass with zero modification.

## Verify

```bash
cd /opt/repos/orca/agent && npx tsc --noEmit
cd /opt/repos/orca/agent && npx vitest run src/relay/agent-print-mode-exec.test.ts
```

Full package suite (per `agent/package.json`'s `"test": "vitest run"` script):

```bash
cd /opt/repos/orca/agent && pnpm test
```

---

## Acceptance Criteria

- [ ] `handleAgentExecPromptStream` added to `agent-print-mode-exec.ts`; `handleAgentExecPrompt`'s
      code and exported signature are unchanged.
- [ ] `case 'agent.execPromptStream'` added to `agent-rpc-dispatch-agent-exec.ts`, following the
      exact `void handleXxxStream(...)` + literal `{ type: 'stream.started' }` return shape used
      by `git.execStream`/`vm.provision`/`agent.spawn` in this codebase today.
- [ ] Chunking-granularity and timeout-behavior open questions (above) are explicitly decided and
      documented in code comments, not left implicit.
- [ ] Existing `agent.execPrompt` callers' tests pass unmodified.
- [ ] New tests (above) pass.
