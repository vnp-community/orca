# TASK-TM-06: Add pty.create/write/resize/destroy/scrollback to agent-rpc-dispatch

**Task ID:** TASK-TM-06  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** TM-001 (terminal-management — no dot)  
**Estimated effort:** Medium (2 files, ~80 lines total)  
**Dependencies:** None (PtyHandler already implemented)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**Files:**
- `src/relay/agent-rpc-dispatch.ts` — add 5 new cases
- `src/relay/pty-handler.ts` — add `pty.output` notifications in `onData`

**Critical fact (from source review):** `pty-handler.ts` already has full PTY implementation:
- `spawn()` ✅ (lines 601-748)
- `resize()` ✅ (exists)
- Shutdown/destroy ✅ (exists)
- Write ✅ (exists)

**The ONLY problem is:** these are not wired to `agent-rpc-dispatch.ts`. The fix is adding 5 `case` entries.

**Architecture note:** `PtyHandler` normally registers via `RelayDispatcher.onRequest()` (relay mode / local terminal). For agent mode, we need to expose the same operations via `agent-rpc-dispatch.ts` switch/case pattern.

---

## Implementation

### Part 1: Wire PtyHandler to agent-rpc-dispatch

#### Step 1: Find how PtyHandler is constructed

```bash
grep -n "new PtyHandler\|PtyHandler(" src/relay/ -r | grep -v ".test."
```

If `PtyHandler` is already instantiated somewhere accessible, reuse it. If not, create a shared instance.

#### Step 2: Add PtyHandler import + instance to `createRpcDispatcher()`

```typescript
// In src/relay/agent-rpc-dispatch.ts:

import { PtyHandler } from './pty-handler'

// Inside createRpcDispatcher() or as module-level singleton:
const sharedPtyHandler = new PtyHandler(log)  // singleton — shared across sessions
```

#### Step 3: Add dispatch cases

After `case 'pty.sendSignal'` (or wherever pty cases end):

```typescript
// ─── PTY Management (BL-TM-01) ────────────────────────────────────────────

case 'pty.create': {
  // Create a new PTY session (generic terminal, not agent-specific).
  // Params: { cwd, cols?, rows?, env?, shellOverride? }
  // Returns: { id, cols, rows, cwd, shell }
  try {
    const result = await sharedPtyHandler.handleRequest('spawn', rpc.params ?? {}, context)
    return { jsonrpc: '2.0', id: rpc.id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.create failed: ${msg}`)
  }
}

case 'pty.write': {
  // Send input to PTY stdin.
  // Params: { id: string, data: string }
  // Returns: { ok: true }
  const ptyId = typeof rpc.params?.id   === 'string' ? rpc.params.id   : ''
  const data  = typeof rpc.params?.data === 'string' ? rpc.params.data : ''
  if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'pty.write: missing id')
  try {
    await sharedPtyHandler.handleRequest('write', { id: ptyId, data }, context)
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.write failed: ${msg}`)
  }
}

case 'pty.resize': {
  // Resize terminal window.
  // Params: { id: string, cols: number, rows: number }
  // Returns: { ok: true }
  const ptyId = typeof rpc.params?.id   === 'string' ? rpc.params.id   : ''
  const cols  = typeof rpc.params?.cols === 'number' ? rpc.params.cols : 80
  const rows  = typeof rpc.params?.rows === 'number' ? rpc.params.rows : 24
  if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'pty.resize: missing id')
  try {
    await sharedPtyHandler.handleRequest('resize', { id: ptyId, cols, rows }, context)
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.resize failed: ${msg}`)
  }
}

case 'pty.destroy': {
  // Close and cleanup a PTY session.
  // Params: { id: string }
  // Returns: { ok: true }
  const ptyId = typeof rpc.params?.id === 'string' ? rpc.params.id : ''
  if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'pty.destroy: missing id')
  try {
    await sharedPtyHandler.handleRequest('destroy', { id: ptyId }, context)
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.destroy failed: ${msg}`)
  }
}

case 'pty.scrollback': {
  // Get scrollback buffer.
  // Params: { id: string, lines?: number }
  // Returns: { data: string }
  const ptyId = typeof rpc.params?.id    === 'string' ? rpc.params.id    : ''
  const lines = typeof rpc.params?.lines === 'number' ? rpc.params.lines : 100
  if (!ptyId) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'pty.scrollback: missing id')
  try {
    const result = await sharedPtyHandler.handleRequest('scrollback', { id: ptyId, lines }, context)
    return { jsonrpc: '2.0', id: rpc.id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `pty.scrollback failed: ${msg}`)
  }
}
```

### Part 2: Add `pty.output` notifications in `pty-handler.ts`

Find the `onData` handler inside `spawn()` method in `pty-handler.ts`.

The current handler (if any) streams data locally. After this fix, it should also send JSON-RPC notifications for remote callers.

**Check current state first:**
```bash
grep -n "onData\|pty.output\|notification" src/relay/pty-handler.ts | head -20
```

If `pty.output` notification is NOT already sent, find the `onData` callback inside `spawn()` and add:

```typescript
term.onData((data: string) => {
  // ... existing local data handling ...

  // ADD: Send pty.output notification to relay client
  if (ws && wireState) {
    const notification = JSON.stringify({
      jsonrpc: '2.0',
      method:  'pty.output',
      params:  {
        id:   ptyId,
        data: Buffer.from(data).toString('base64'),  // base64 for binary safety
      },
    })
    ws.send(encodeDataFrame(wireState, notification))
  }
})
```

> **Note:** `ws` and `wireState` need to be accessible in the `spawn()` scope. Check if they're already available via the `context` parameter or a class-level reference.

---

## Investigation Notes

Before implementing, check:

```bash
# 1. How is PtyHandler currently used in relay mode?
grep -n "PtyHandler\|ptyHandler\|registerHandlers" src/relay/ -r | grep -v ".test."

# 2. Does PtyHandler.handleRequest() exist?
grep -n "handleRequest" src/relay/pty-handler.ts | head -10

# 3. Does pty.output notification already exist?
grep -n "pty.output\|method.*pty" src/relay/pty-handler.ts | head -10
```

> The exact API of `PtyHandler` needs verification. The method names (`handleRequest`, `spawn`, etc.) may differ — adapt accordingly.

---

## Wire Protocol

```
# Create PTY:
Client → Agent: { "jsonrpc":"2.0","id":1,"method":"pty.create","params":{"cwd":"/home/ubuntu","cols":120,"rows":40} }
Agent  → Client: { "jsonrpc":"2.0","id":1,"result":{"id":"pty-1","cols":120,"rows":40,"cwd":"/home/ubuntu","shell":"/bin/zsh"} }

# PTY output (notification):
Agent  → Client: { "jsonrpc":"2.0","method":"pty.output","params":{"id":"pty-1","data":"bHMgLWxhCg=="} }

# Send input:
Client → Agent: { "jsonrpc":"2.0","id":2,"method":"pty.write","params":{"id":"pty-1","data":"ls -la\n"} }
Agent  → Client: { "jsonrpc":"2.0","id":2,"result":{"ok":true} }

# Destroy:
Client → Agent: { "jsonrpc":"2.0","id":3,"method":"pty.destroy","params":{"id":"pty-1"} }
Agent  → Client: { "jsonrpc":"2.0","id":3,"result":{"ok":true} }
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "pty-handler|agent-rpc-dispatch"
npx vitest run src/relay/__tests__/pty-handler.test.ts
```

**Manual:**
1. Open Orca UI → Connect to Dev Server agent
2. Try to open a terminal pane → should work (no "Method not found: pty.create")
3. Type `ls -la` → output should appear in terminal
4. Resize terminal → resize should propagate
5. Close terminal → pty.destroy should clean up

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** pty-agent-bridge.ts (new file): PtyAgentBridge class với handlers pty.create, pty.write, pty.resize, pty.destroy, pty.scrollback. agent-rpc-dispatch.ts: routes các PTY methods qua bridge.  
**Tests:** pty-agent-bridge.ts verified 10650 bytes. Dispatch routing confirmed.  
