# TASK-ORCH-04: Add handleAgentSendInput + agent.sendInput RPC Case

**Task ID:** TASK-ORCH-04  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** ORCH-001  
**Estimated effort:** Medium (new function + dispatch case)  
**Dependencies:** TASK-ORCH-01 (PTY_REGISTRY structure)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**Files to modify:**
1. `src/relay/agent-spawner.ts` — add `handleAgentSendInput` export
2. `src/relay/agent-rpc-dispatch.ts` — add `case 'agent.sendInput'`

**Problem:** No RPC method exists to send input to a running agent PTY. Without this:
- Cannot send Ctrl+C (`\x03`) for graceful stop
- Cannot pass user prompts to running agent
- Interactive agent use cases completely blocked

---

## Implementation

### Part 1: `src/relay/agent-spawner.ts`

Add the following export function **after** `handleAgentKill`:

```typescript
// ── handleAgentSendInput ──────────────────────────────────────────────────────
// ORCH-001: Write arbitrary data to a running agent PTY's stdin.
// Primary use cases:
//   - Graceful stop: send '\x03' (Ctrl+C)
//   - Multi-turn prompts: send user message to agent stdin
//   - Confirmation prompts: send 'y\n' or 'n\n'

export async function handleAgentSendInput(
  id:      string | number | null,
  params:  Record<string, unknown>,
  _config: AgentConfig,
  log:     AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''

  if (!ptyId) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' },
    }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` },
    }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.sendInput failed: ${msg}`)
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.ServerError, message: msg },
    }
  }
}
```

### Part 2: `src/relay/agent-rpc-dispatch.ts`

Add after `case 'agent.kill'` in the switch statement:

```typescript
// ── agent.sendInput ───────────────────────────────────────────────────────────
case 'agent.sendInput': {
  try {
    const { handleAgentSendInput } = await import('./agent-spawner')
    return (await handleAgentSendInput(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `agent.sendInput unavailable: ${msg}`)
  }
}
```

---

## Wire Protocol

```
# Graceful stop:
Client → Agent: { jsonrpc: "2.0", id: 5, method: "agent.sendInput", params: { ptyId: "pty-u1-t1-xxx", data: "\x03" } }
Agent  → Client: { jsonrpc: "2.0", id: 5, result: { ok: true } }

# Multi-turn prompt:
Client → Agent: { jsonrpc: "2.0", id: 6, method: "agent.sendInput", params: { ptyId: "pty-u1-t1-xxx", data: "What is 2+2?\n" } }
Agent  → Client: { jsonrpc: "2.0", id: 6, result: { ok: true } }
```

---

## Unit Tests to Add

File: `src/relay/__tests__/agent-spawner.test.ts`

```typescript
describe('handleAgentSendInput', () => {
  const config = { workDir: '/tmp' } as AgentConfig
  const log = { info: vi.fn(), warn: vi.fn(), error: vi.fn() } as AgentLogger

  it('returns error when ptyId missing', async () => {
    const r = await handleAgentSendInput(1, {}, config, log) as any
    expect(r.error.code).toBeDefined()
    expect(r.error.message).toContain('Missing ptyId')
  })

  it('returns error when PTY not found', async () => {
    const r = await handleAgentSendInput(2, { ptyId: 'ghost', data: 'x' }, config, log) as any
    expect(r.error.message).toContain('ghost')
  })

  it('writes data to PTY and returns ok=true', async () => {
    const mockWrite = vi.fn()
    const mockPty = { write: mockWrite, kill: vi.fn() }
    // Inject into PTY_REGISTRY directly via test helper or export
    PTY_REGISTRY.set('test-pty', { pty: mockPty as any, taskId: 'tid', userId: 'uid' })
    const r = await handleAgentSendInput(3, { ptyId: 'test-pty', data: '\x03' }, config, log) as any
    expect(mockWrite).toHaveBeenCalledWith('\x03')
    expect(r.result.ok).toBe(true)
    PTY_REGISTRY.delete('test-pty')
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "agent-spawner|agent-rpc-dispatch"
npx vitest run src/relay/__tests__/agent-spawner.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-rpc-dispatch.ts: case 'agent.sendInput' handler. Lấy ptyId từ params, lookup PTY_REGISTRY, gọi pty.write(input).  
**Tests:** Integration verified via dispatch routing.  
