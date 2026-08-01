# TASK-ORCH-05: Fix handleAgentKill Signal + Add cleanupAllPtys

**Task ID:** TASK-ORCH-05  
**Priority:** 🔴 CRITICAL  
**Bugs fixed:** ORCH-002, ORCH-011  
**Estimated effort:** Small  
**Dependencies:** TASK-ORCH-01  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/agent-spawner.ts`

### Bug ORCH-002 — Kill signal hardcoded to SIGTERM

**Current code (L215):**
```typescript
entry.pty.kill('SIGTERM')  // ❌ ignores params.signal
```

**Problem:** `agent.kill` is supposed to accept `signal: 'SIGKILL'` for force-kill, but hardcodes SIGTERM. Hung processes cannot be force-killed by the client.

### Bug ORCH-011 — Orphaned PTYs after WS disconnect

**Current code (agent-session.ts):**
```typescript
stop(): void {
  if (keepaliveTimer !== null) {
    clearInterval(keepaliveTimer)
    keepaliveTimer = null
  }
  // ← PTY_REGISTRY never cleaned up!
}
```

**Problem:** When Orca Server disconnects (user closes laptop, network drop), all spawned agent PTYs keep running on the Dev Server consuming CPU/memory/tokens indefinitely.

---

## Implementation

### Part 1: Fix `handleAgentKill` in `src/relay/agent-spawner.ts`

```typescript
export async function handleAgentKill(
  id:      string | number | null,
  params:  Record<string, unknown>,
  _config: AgentConfig,
  log:     AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  // ORCH-002: Validate and use the caller's requested signal
  const rawSignal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  const signal: 'SIGTERM' | 'SIGKILL' = rawSignal === 'SIGKILL' ? 'SIGKILL' : 'SIGTERM'
  const span = spawnerTracer.start({ method: 'agent.kill', ptyId: ptyId || '(empty)', signal })

  if (!ptyId) {
    span.fail('missing ptyId', {})
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, note: 'already dead' })
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } }
  }

  // ORCH-002: Use validated signal
  if (process.platform === 'win32') {
    entry.pty.kill()  // Windows: no POSIX signals
  } else {
    entry.pty.kill(signal)
  }
  PTY_REGISTRY.delete(ptyId)
  span.ok({ ptyId, signal })
  log.info(`agent.kill: ptyId=${ptyId} ${signal} sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

### Part 2: Add `cleanupAllPtys` export to `src/relay/agent-spawner.ts`

Add after `handleAgentKill`:

```typescript
// ── cleanupAllPtys ───────────────────────────────────────────────────────────
// ORCH-011: Called by agent-session.ts stop() when WS session closes.
// Kills all active PTYs to prevent orphaned processes on the Dev Server.

export function cleanupAllPtys(log: AgentLogger): void {
  if (PTY_REGISTRY.size === 0) return
  log.info(`session.stop: cleaning up ${PTY_REGISTRY.size} orphaned PTY(s)`)
  for (const [ptyId, entry] of PTY_REGISTRY.entries()) {
    try {
      if (process.platform === 'win32') {
        entry.pty.kill()
      } else {
        entry.pty.kill('SIGTERM')
      }
      log.info(`session.stop: killed PTY ${ptyId}`)
    } catch (err) {
      log.warn(`session.stop: failed to kill PTY ${ptyId}: ${err}`)
    }
  }
  PTY_REGISTRY.clear()
}
```

### Part 3: Wire `cleanupAllPtys` into `src/relay/agent-session.ts`

1. Add import at top of file:
```typescript
import { cleanupAllPtys } from './agent-spawner'
```

2. Update `stop()` method:
```typescript
stop(): void {
  if (keepaliveTimer !== null) {
    clearInterval(keepaliveTimer)
    keepaliveTimer = null
  }
  // ORCH-011: Kill orphaned PTYs on disconnect
  cleanupAllPtys(log)
},
```

---

## Wire Protocol

```
# Force-kill:
Client → Agent: { jsonrpc: "2.0", id: 7, method: "agent.kill", params: { ptyId: "pty-u1-t1-xxx", signal: "SIGKILL" } }
Agent  → Client: { jsonrpc: "2.0", id: 7, result: { ok: true } }

# Graceful kill (default):
Client → Agent: { jsonrpc: "2.0", id: 8, method: "agent.kill", params: { ptyId: "pty-u1-t1-xxx" } }
Agent  → Client: { jsonrpc: "2.0", id: 8, result: { ok: true } }
```

---

## Unit Tests to Add

```typescript
describe('handleAgentKill', () => {
  it('sends SIGKILL when params.signal = SIGKILL', async () => {
    const mockKill = vi.fn()
    PTY_REGISTRY.set('pty-k1', { pty: { kill: mockKill, write: vi.fn() } as any, taskId: 't', userId: 'u' })
    await handleAgentKill(1, { ptyId: 'pty-k1', signal: 'SIGKILL' }, config, log)
    expect(mockKill).toHaveBeenCalledWith('SIGKILL')
  })

  it('defaults to SIGTERM for unknown signal value', async () => {
    const mockKill = vi.fn()
    PTY_REGISTRY.set('pty-k2', { pty: { kill: mockKill, write: vi.fn() } as any, taskId: 't', userId: 'u' })
    await handleAgentKill(2, { ptyId: 'pty-k2', signal: 'INVALID' }, config, log)
    expect(mockKill).toHaveBeenCalledWith('SIGTERM')
  })
})

describe('cleanupAllPtys', () => {
  it('kills all PTYs in registry', () => {
    const kill1 = vi.fn(), kill2 = vi.fn()
    PTY_REGISTRY.set('p1', { pty: { kill: kill1 } as any, taskId: 't1', userId: 'u' })
    PTY_REGISTRY.set('p2', { pty: { kill: kill2 } as any, taskId: 't2', userId: 'u' })
    cleanupAllPtys(mockLog)
    expect(kill1).toHaveBeenCalled()
    expect(kill2).toHaveBeenCalled()
    expect(PTY_REGISTRY.size).toBe(0)
  })

  it('is a no-op when registry is empty', () => {
    PTY_REGISTRY.clear()
    expect(() => cleanupAllPtys(mockLog)).not.toThrow()
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "agent-spawner|agent-session"
npx vitest run src/relay/__tests__/agent-spawner.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-spawner.ts: handleAgentKill đọc params.signal, validate 'SIGTERM'|'SIGKILL', gọi entry.pty.kill(signal). PTY_REGISTRY.delete(ptyId) sau kill. pty.onExit cũng delete.  
**Tests:** agent-spawner.test.ts: handleAgentKill tests pass — idempotent, missing ptyId, signal params.  
