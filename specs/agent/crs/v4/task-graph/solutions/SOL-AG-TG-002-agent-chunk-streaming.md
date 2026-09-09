# SOL-AG-TG-002: `agent.execPromptStream`/`shell.execStream` — reuse the existing `stream.chunk`/`stream.end` convention

**Resolves:** [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md) (agent-side portion — backend-go's `infra-fleet-service` RPC is [BE-SOL-006](../../../../../backend-go/crs/v4/task-graph/solutions/BE-SOL-006-task-execute-streaming-relay.md))
**Depends on:** [SOL-AG-TG-001](./SOL-AG-TG-001-agent-env-contract-assessment.md) (unrelated code path, no ordering dependency, listed for context)
**Affected files (proposed):**
- `agent/src/relay/agent-print-mode-exec.ts` (new `handleAgentExecPromptStream`, `handleAgentExecPrompt` unchanged)
- `agent/src/relay/agent-rpc-dispatch-misc.ts` (new `shell.execStream` handler, alongside existing `shell.exec`)
- `agent/src/relay/agent-rpc-dispatch.ts` (register 2 new method names)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** dòng dưới đây nói *"the
> dispatcher sends the initial `stream.started` response before calling the
> handler"* — **không chính xác về cơ chế**. Đọc dispatch site thật
> (`agent-rpc-dispatch-git.ts`) cho thấy: dispatcher gọi handler trước qua
> `void handleXxxStream(...)` (fire-and-forget, chạy đồng bộ tới `await`
> đầu tiên), **rồi mới** `return { type: 'stream.started' }` như 1 giá trị
> literal — không phải 2 lần `sendFrame` tuần tự. Kết quả hành vi tương
> đương, nhưng code mẫu trong task
> ([TASK-AG-TG-001](../tasks/TASK-AG-TG-001-agent-exec-prompt-stream-handler.md))
> dùng đúng shape `void handleXxx(...)` + return literal, không dùng 2
> `sendFrame` như gợi ý ở đây. Ngoài ra, `git.execStream` **không có test
> nào** trong `__tests__/agent-git-handler.test.ts` (bị skip hẳn) —
> §"Test plan" bên dưới đề xuất "reuse `git.execStream`'s existing
> cleanup-on-disconnect test pattern, if one exists" — **không tồn tại**;
> dùng `agent-ephemeral-vm-handler.test.ts:208-248`'s `vm.provision` test
> (MockWs + `createWireState`/`decodeFrame`) làm mẫu thay, đã cập nhật
> trong task.

---

## Design rationale — this pattern already exists twice in `agent/`, reuse it verbatim

CR-TG-006's original framing (generic `chunk` notifications reusing a
`pty.data`-style shape) turns out to not match what's actually idiomatic in
this codebase. Reading `agent-git-handler.ts`'s real `git.execStream`
handler shows the established convention precisely:

```typescript
// agent-git-handler.ts:233-286 (handleGitExecStream)
// Frame format (JSON-RPC result field, all sharing the SAME request id):
//   { type: 'stream.chunk', line: string }                    — stdout line
//   { type: 'stream.chunk', line: string, source: 'stderr' }  — stderr line
//   { type: 'stream.end',   exitCode: number }                — process exited
// The initial 'stream.started' response is sent by the dispatcher before calling here.

function sendChunk(line: string, source?: 'stderr'): void {
  sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.chunk', line, ...(source ? { source } : {}) } })
}
function sendEnd(exitCode: number): void {
  sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode } })
}
```

`agent-ephemeral-vm-handler.ts` independently implements the exact same
`{type: 'stream.chunk', line, source?}`/`{type:'stream.end', exitCode}`
shape for a third, unrelated command family — confirming this is a
codebase-wide convention for "one request, many response frames sharing
the same `id`," not something local to git. This solution reuses it for a
fourth time rather than inventing the `pty.data`-notification shape
CR-TG-006 originally sketched.

## Design — separate streaming method names, non-streaming methods untouched

Following the exact `git.exec` / `git.execStream` split (two distinct RPC
method names, not one method with a `stream: true` flag) — `agent.execPrompt`
and `shell.exec` keep their current buffer-then-return behavior unchanged
for every existing caller (`task-service.SimpleExecutor`,
`workflow-service`'s step executors); two new sibling methods are added:

```typescript
// agent-print-mode-exec.ts — new function, handleAgentExecPrompt untouched
export async function handleAgentExecPromptStream(
  ws: WebSocket, wireState: WireState, id: string | number | null,
  params: Record<string, unknown>, config: AgentConfig, log: AgentLogger
): Promise<void> {
  // ...identical validation/spec-resolution/buildAgentEnv setup as handleAgentExecPrompt...
  function sendChunk(text: string, source?: 'stderr'): void {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.chunk', line: text, ...(source ? { source } : {}) } })
  }
  child.stdout?.on('data', (d: Buffer) => sendChunk(d.toString('utf8')))
  child.stderr?.on('data', (d: Buffer) => sendChunk(d.toString('utf8'), 'stderr'))
  child.on('close', (code) => {
    sendFrame(ws, wireState, { jsonrpc: '2.0', id, result: { type: 'stream.end', exitCode: code ?? -1 } })
  })
}
```

The dispatcher (`agent-rpc-dispatch.ts`) sends the initial `stream.started`
response before calling the handler, matching `git.execStream`'s existing
contract exactly (`handleGitExecStream`'s doc comment: *"The initial
'stream.started' response is sent by the dispatcher before calling
here"*) — this solution's handler does not send that frame itself, for
consistency with the one precedent already in the codebase.

`shell.execStream` mirrors the same shape against `agent-rpc-dispatch-misc.ts`'s
existing `shell.exec` implementation.

## Not in scope (per the CR)

- `agent.spawn`'s output/exit streaming — that's a persistent-PTY session,
  a structurally different problem solved by
  [BE-SOL-006](../../../../../backend-go/crs/v4/task-graph/solutions/BE-SOL-006-task-execute-streaming-relay.md)'s
  gRPC-level `AttachAgentSpawn`, reusing `agent.output`/`agent.exited`
  (already-existing notifications), not this chunk convention.
- Any change to `ai.complete` — not in CR-TG-006's scope; AI provider
  streaming (if ever needed) is a separate concern with its own contract
  considerations (partial-token streaming vs. line-based process output).
- Backward-compatibility shims — none needed, since `agent.execPrompt`/
  `shell.exec` are entirely untouched by this solution.

## Test plan

- `agent.execPromptStream` on a prompt producing multi-line output emits
  one `stream.chunk` frame per stdout line, then exactly one `stream.end`
  with the correct exit code.
- Killing the connection mid-stream doesn't leave the spawned child process
  orphaned (reuse `git.execStream`'s existing cleanup-on-disconnect test
  pattern, if one exists, as the precedent to follow).
- `agent.execPrompt` (non-streaming) behavior is provably unchanged
  (existing test suite passes with zero modification).

## References

- [CR-TG-006](../../../../../../docs/crs/v4/task-graph/CR-TG-006-task-execute-streaming-relay.md)
- `agent/src/relay/agent-git-handler.ts:233-300` (`handleGitExecStream` — the precedent this solution copies)
- `agent/src/relay/agent-ephemeral-vm-handler.ts:170-180` (second independent confirmation of the same convention)
