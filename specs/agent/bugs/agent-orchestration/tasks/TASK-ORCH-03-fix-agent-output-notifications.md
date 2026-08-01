# TASK-ORCH-03: Fix agent.spawn Output — JSON-RPC Notifications (not Responses)

**Task ID:** TASK-ORCH-03  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** ORCH-006  
**Estimated effort:** Small (change 2 event handlers)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

File: `src/relay/agent-spawner.ts`

**Current code (L163-180) — WRONG:**
```typescript
pty.onData((data) => {
  // ❌ Sends response with same `id` repeatedly — violates JSON-RPC 2.0
  const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.output', ptyId, data } })
  ws.send(encodeDataFrame(wireState, frame))
})

pty.onExit(({ exitCode }) => {
  // ❌ Same id problem
  const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.exit', ptyId, code: exitCode } })
  ws.send(encodeDataFrame(wireState, frame))
})
```

**Problem:** JSON-RPC 2.0 spec: a request with an `id` must receive **exactly one response**. Sending multiple `{ id, result }` frames violates the protocol. Orca Server cannot distinguish streaming data from the actual spawn response.

**Fix:** Use JSON-RPC **notifications** (no `id` field) for streaming output. Initial spawn response with `{ type: 'spawn.accepted' }` is handled by the dispatch layer.

---

## Implementation

### Replace `pty.onData` handler

```typescript
// NEW — JSON-RPC 2.0 notification (no id):
pty.onData((data) => {
  const notification = JSON.stringify({
    jsonrpc: '2.0',
    method:  'agent.output',
    params:  {
      ptyId,
      data: Buffer.from(data).toString('base64'),  // base64 for binary-safe transport
    },
  })
  ws.send(encodeDataFrame(wireState, notification))
})
```

### Replace `pty.onExit` handler

```typescript
// NEW — JSON-RPC 2.0 notification (no id):
pty.onExit(({ exitCode }) => {
  PTY_REGISTRY.delete(ptyId)
  spawner.transition('stopping')
  spawner.transition('stopped')
  if (exitCode === 0) {
    span.ok({ ptyId, exitCode })
  } else {
    span.fail(`exit code ${exitCode}`, { ptyId, exitCode })
  }
  const notification = JSON.stringify({
    jsonrpc: '2.0',
    method:  'agent.exited',
    params:  { ptyId, exitCode },
  })
  ws.send(encodeDataFrame(wireState, notification))
  log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`)
})
```

---

## Protocol Contract (for Orca Server side)

After this fix, the wire protocol becomes:

```
Client → Agent:   { jsonrpc: "2.0", id: 1, method: "agent.spawn", params: {...} }
Agent  → Client:  { jsonrpc: "2.0", id: 1, result: { type: "spawn.accepted", ptyId: "pty-..." } }
Agent  → Client:  { jsonrpc: "2.0", method: "agent.output", params: { ptyId: "...", data: "<base64>" } }
Agent  → Client:  { jsonrpc: "2.0", method: "agent.output", params: { ptyId: "...", data: "<base64>" } }
...
Agent  → Client:  { jsonrpc: "2.0", method: "agent.exited", params: { ptyId: "...", exitCode: 0 } }
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep agent-spawner
```

**Manual verification:**
1. Send `agent.spawn` → expect ONE response with `{ type: 'spawn.accepted' }`
2. Agent output → expect notifications with `method: 'agent.output'` (no `id`)
3. Agent exit → expect notification with `method: 'agent.exited'` (no `id`)

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-spawner.ts: pty.onData gửi notification (method: agent.output, no id). pty.onExit gửi notification (method: agent.exited). Không vi phạm JSON-RPC 2.0.  
**Tests:** agent-spawner.test.ts: handleAgentSpawn validation tests pass.  
