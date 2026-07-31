# T19 — Tạo agent-spawner.ts (SubAgentSpawner — relay tier) [NEW FILE]

**Phase:** 3 (parallel với T15, T18)  
**Effort:** ~2 hours  
**Depends on:** — (độc lập)  
**Solution ref:** [conflict-analysis-tdd-v6.md §NEW-A](../../../../../conflict-analysis-tdd-v6.md)  
**Agent spec ref:** [specs/agent/crs/v1/tdd-v6/solutions/CR-AG-12-agent-spawner.md](../../../../../agent/crs/v1/tdd-v6/solutions/CR-AG-12-agent-spawner.md)  
**⚠️ Conflict Resolution:** NEW-A — tên `SubAgentSpawner` để phân biệt với `ProfileAwareAgentSpawner` (Orca Server tier)

---

## ⚠️ QUAN TRỌNG — Phân biệt 2 tier

```
src/main/project/ProfileAwareAgentSpawner.ts  ← Orca Server tier (4.6KB, ĐÃ TỒN TẠI)
src/relay/agent-spawner.ts                     ← Dev Server/Relay tier (CHƯA TỒN TẠI — TẠO MỚI)

Hai file này HOÀN TOÀN KHÁC NHAU — không conflict, không overlap.
agent-spawner.ts export class SubAgentSpawner (tên khác để phân biệt rõ ràng).
```

## Mục tiêu

Tạo `src/relay/agent-spawner.ts` cho Dev Server tier:
- Class `SubAgentSpawner` — quản lý lifecycle PTY process cho sub-agent
- `handleAgentSpawn()` — RPC handler (fire-and-forget, streaming)
- `handleAgentKill()` — RPC handler
- PTY Registry (in-process singleton)

**Đây là file Agent tier owns** — Agent CR-AG-12 spec chỉ đạo.

---

## Files Cần Đọc Trước

1. `src/relay/agent-credential-store.ts` — pattern `handleReadCredential()`
2. `src/relay/agent-config.ts` — `AgentConfig` interface
3. `src/relay/agent-logger.ts` — `AgentLogger` interface
4. `src/relay/agent-wire.ts` — `encodeDataFrame()`, `WireState`
5. `src/shared/agent-wire-protocol.ts` — `AgentErrorCode`
6. `src/relay/agent-rpc-dispatch.ts` — xem cases `agent.spawn`, `agent.kill` (TASK-07 DONE, line 177-198)

---

## File: `src/relay/agent-spawner.ts` [NEW]

```typescript
/**
 * agent-spawner.ts — Dev Server tier SubAgent spawner (CR-AG-12)
 *
 * ⚠️ KHÔNG nhầm với src/main/project/ProfileAwareAgentSpawner.ts (Orca Server tier)
 *    Đây là relay-side spawner cho sub-agent process management.
 *
 * Exports:
 *   SubAgentSpawner   — class lifecycle manager (pure, testable)
 *   handleAgentSpawn  — RPC handler (fire-and-forget streaming)
 *   handleAgentKill   — RPC handler
 *   buildAgentEnv     — env builder (testable with mock credStore)
 *   resolveAgentSpec  — model → binary spec (pure, testable)
 *
 * @module relay/agent-spawner
 */
import * as nodePty from 'node-pty'
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { encodeDataFrame } from './agent-wire'
import type { WireState } from './agent-wire'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ── Types ──────────────────────────────────────────────────────────────────────

export type AgentLifecycleState = 'idle' | 'spawning' | 'running' | 'stopping' | 'stopped' | 'error'

export interface AgentSpawnRequest {
  taskId:    string
  userId:    string
  modelId:   string
  accountId: string
  cwd?:      string
}

export interface AgentStatusEvent {
  type:    'spawn.accepted' | 'spawn.started' | 'spawn.output' | 'spawn.exit' | 'spawn.error'
  ptyId?:  string
  taskId?: string
  data?:   string
  code?:   number
  error?:  string
}

// ── PTY Registry (in-process singleton) ──────────────────────────────────────

const PTY_REGISTRY = new Map<string, {
  pty:    nodePty.IPty
  taskId: string
  userId: string
}>()

// ── SubAgentSpawner (pure class — testable) ───────────────────────────────────

export class SubAgentSpawner {
  private state: AgentLifecycleState = 'idle'

  getState(): AgentLifecycleState { return this.state }

  transition(next: AgentLifecycleState): void {
    const VALID: Record<AgentLifecycleState, AgentLifecycleState[]> = {
      idle:     ['spawning'],
      spawning: ['running', 'error'],
      running:  ['stopping', 'error'],
      stopping: ['stopped', 'error'],
      stopped:  ['idle'],
      error:    ['idle'],
    }
    if (!VALID[this.state]?.includes(next)) {
      throw new Error(`SubAgentSpawner: invalid transition ${this.state} → ${next}`)
    }
    this.state = next
  }
}

// ── resolveAgentSpec (pure, testable) ─────────────────────────────────────────

export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}

// ── buildAgentEnv (testable with mock credStore) ──────────────────────────────

export async function buildAgentEnv(
  accountId: string,
  apiKey:    string,
  cwd:       string,
): Promise<Record<string, string>> {
  return {
    ANTHROPIC_API_KEY: apiKey,
    OPENAI_API_KEY:    apiKey,
    GEMINI_API_KEY:    apiKey,
    ORCA_AGENT_CWD:    cwd,
    ORCA_ACCOUNT_ID:   accountId,
    HOME:              process.env.HOME ?? '/tmp',
    PATH:              process.env.PATH ?? '/usr/bin:/bin',
  }
}

// ── handleAgentSpawn (fire-and-forget) ────────────────────────────────────────

export async function handleAgentSpawn(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
  ws:     WebSocket,
  _state: WireState,
): Promise<void> {
  const req: AgentSpawnRequest = {
    taskId:    typeof params.taskId    === 'string' ? params.taskId    : '',
    userId:    typeof params.userId    === 'string' ? params.userId    : '',
    modelId:   typeof params.modelId   === 'string' ? params.modelId   : '',
    accountId: typeof params.accountId === 'string' ? params.accountId : '',
    cwd:       typeof params.cwd       === 'string' ? params.cwd       : config.workDir,
  }

  if (!req.taskId || !req.modelId || !req.accountId) {
    const errFrame = JSON.stringify({
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing taskId/modelId/accountId' },
    })
    ws.send(encodeDataFrame(errFrame))
    return
  }

  const spawner = new SubAgentSpawner()

  try {
    spawner.transition('spawning')

    const spec = resolveAgentSpec(req.modelId)
    const env  = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)

    const ptyId = `${req.taskId}-${Date.now()}`
    const pty = nodePty.spawn(spec.binary, spec.args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd:  req.cwd ?? config.workDir,
      env,
    })

    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId })
    spawner.transition('running')

    log.info(`agent.spawn: ptyId=${ptyId} model=${req.modelId}`)

    pty.onData((data) => {
      const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.output', ptyId, data } })
      ws.send(encodeDataFrame(frame))
    })

    pty.onExit(({ exitCode }) => {
      PTY_REGISTRY.delete(ptyId)
      spawner.transition('stopping')
      spawner.transition('stopped')
      const frame = JSON.stringify({ jsonrpc: '2.0', id, result: { type: 'spawn.exit', ptyId, code: exitCode } })
      ws.send(encodeDataFrame(frame))
      log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`)
    })

  } catch (err: unknown) {
    spawner.transition('error')
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.spawn: error ${msg}`)
    const frame = JSON.stringify({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } })
    ws.send(encodeDataFrame(frame))
  }
}

// ── handleAgentKill ───────────────────────────────────────────────────────────

export async function handleAgentKill(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''

  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } }
  }

  entry.pty.kill('SIGTERM')
  PTY_REGISTRY.delete(ptyId)
  log.info(`agent.kill: ptyId=${ptyId} SIGTERM sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

---

## Tests: `src/relay/__tests__/agent-spawner.test.ts` [NEW]

Theo spec TDD-AG-12 §10 (≥17 tests):

```typescript
import { describe, it, expect, vi } from 'vitest'
import { SubAgentSpawner, resolveAgentSpec, buildAgentEnv, handleAgentKill } from '../agent-spawner'

describe('SubAgentSpawner', () => {
  it('initial state is idle', () => {
    expect(new SubAgentSpawner().getState()).toBe('idle')
  })
  it('idle → spawning OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning')
    expect(s.getState()).toBe('spawning')
  })
  it('idle → running throws', () => {
    expect(() => new SubAgentSpawner().transition('running')).toThrow()
  })
  it('spawning → running OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning')
    s.transition('running')
    expect(s.getState()).toBe('running')
  })
  it('running → stopping OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning'); s.transition('running'); s.transition('stopping')
    expect(s.getState()).toBe('stopping')
  })
  it('stopping → stopped OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning'); s.transition('running'); s.transition('stopping'); s.transition('stopped')
    expect(s.getState()).toBe('stopped')
  })
})

describe('resolveAgentSpec', () => {
  it('claude → binary=claude', () => {
    expect(resolveAgentSpec('claude-3-5-sonnet').binary).toBe('claude')
  })
  it('gemini → binary=gemini', () => {
    expect(resolveAgentSpec('gemini-2.0-flash').binary).toBe('gemini')
  })
  it('unknown modelId throws', () => {
    expect(() => resolveAgentSpec('unknown-model')).toThrow('unknown modelId')
  })
  it('claude args includes stream-json', () => {
    expect(resolveAgentSpec('claude-3').args).toContain('stream-json')
  })
})

describe('buildAgentEnv', () => {
  it('returns object với ANTHROPIC_API_KEY', async () => {
    const env = await buildAgentEnv('acc-1', 'sk-xxx', '/repo')
    expect(env.ANTHROPIC_API_KEY).toBe('sk-xxx')
  })
  it('ORCA_AGENT_CWD = cwd param', async () => {
    const env = await buildAgentEnv('acc-1', 'sk-xxx', '/my/repo')
    expect(env.ORCA_AGENT_CWD).toBe('/my/repo')
  })
})

describe('handleAgentKill', () => {
  const mockConfig = { workDir: '/tmp', toolPath: '/bin/tool' } as any
  const mockLog = { info: vi.fn(), error: vi.fn(), warn: vi.fn() } as any

  it('returns ok=true khi pty not found', async () => {
    const result = await handleAgentKill(1, { ptyId: 'not-exist' }, mockConfig, mockLog) as any
    expect(result.result.ok).toBe(true)
  })
  it('returns error khi thiếu ptyId', async () => {
    const result = await handleAgentKill(1, {}, mockConfig, mockLog) as any
    expect(result.error.code).toBeDefined()
  })
})
```

---

## Bước Verify

```bash
# TypeScript:
npx tsc --noEmit -p config/tsconfig.node.json

# Tests:
pnpm vitest run src/relay/__tests__/agent-spawner.test.ts
# Expected: ≥17 tests pass

# Verify không overlap với Orca Server tier:
grep -rn "ProfileAwareAgentSpawner" src/relay/agent-spawner.ts  # phải 0 results
grep -rn "SubAgentSpawner" src/main/project/                    # phải 0 results
```

---

## Acceptance Criteria

- [x] `src/relay/agent-spawner.ts` tạo mới, export `SubAgentSpawner`, `handleAgentSpawn`, `handleAgentKill` ✅
- [x] `src/relay/__tests__/agent-spawner.test.ts` tạo mới — ≥17 tests pass ✅ (`sub-agent-spawner.test.ts` — 26 tests pass)
- [x] Tên export là `SubAgentSpawner` (KHÔNG phải `ProfileAwareAgentSpawner`) ✅
- [x] `npx tsc --noEmit` → 0 errors ✅
- [x] `agent-rpc-dispatch.ts` cases `agent.spawn`/`agent.kill` (line 381-393) tương thích với file mới ✅
