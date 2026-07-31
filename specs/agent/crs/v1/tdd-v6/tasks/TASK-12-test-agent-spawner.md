# TASK-12: Write Tests — agent-spawner

> ✅ **STATUS: DONE** — Completed 2026-07-30T18:13
> 📝 **Result:** 46/46 tests pass — NEW file created: `AgentStateMachine` (10), `resolveAgentSpec` (13), `buildAgentEnv` (13), `handleAgentKill` (4), `handleAgentSpawn` validation (6).
**Phase:** 5
**File:** `src/relay/__tests__/agent-spawner.test.ts` (NEW FILE)
**Operation:** CREATE
**Depends on:** TASK-06 phải hoàn thành

> **Note về node-pty:** Các tests liên quan đến PTY spawn thực tế cần native addon.
> Tests trong file này tập trung vào **pure units** (state machine, resolver, env builder)
> và **validation** (không cần node-pty thực tế).

---

## Test File

Tạo `src/relay/__tests__/agent-spawner.test.ts`:

```typescript
// src/relay/__tests__/agent-spawner.test.ts

import { describe, it, expect, vi } from 'vitest'
import { tmpdir } from 'node:os'

import {
  AgentStateMachine,
  resolveAgentSpec,
  buildAgentEnv,
  handleAgentKill,
  handleAgentSpawn,
} from '../agent-spawner'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const MOCK_CONFIG = {
  devServerId:   'test-server',
  agentToken:    '',
  workDir:       tmpdir(),
  credentialDir: tmpdir(),
  toolPath:      '/usr/local/bin:/usr/bin:/bin',
  toolEnv:       { PATH: '/usr/local/bin:/usr/bin:/bin' },
} as unknown as AgentConfig

const MOCK_LOG: AgentLogger = {
  info:  () => {},
  warn:  () => {},
  error: () => {},
  debug: () => {},
}

// ─── AgentStateMachine ────────────────────────────────────────────────────────

describe('AgentStateMachine', () => {
  it('starts in idle state', () => {
    const sm = new AgentStateMachine()
    expect(sm.current()).toBe('idle')
  })

  it('idle → running on first_output', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    expect(sm.current()).toBe('running')
  })

  it('running → waiting_for_input on osc_prompt_open', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('osc_prompt_open')
    expect(sm.current()).toBe('waiting_for_input')
  })

  it('waiting_for_input → running on osc_prompt_close', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('osc_prompt_open')
    sm.transition('osc_prompt_close')
    expect(sm.current()).toBe('running')
  })

  it('running → completed on exit_ok', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('exit_ok')
    expect(sm.current()).toBe('completed')
  })

  it('running → error on exit_err', () => {
    const sm = new AgentStateMachine()
    sm.transition('first_output')
    sm.transition('exit_err')
    expect(sm.current()).toBe('error')
  })

  it('idle → completed (direct, no output)', () => {
    const sm = new AgentStateMachine()
    sm.transition('exit_ok')
    expect(sm.current()).toBe('completed')
  })

  it('does not transition from idle on osc_prompt_open', () => {
    const sm = new AgentStateMachine()
    sm.transition('osc_prompt_open')
    // Still idle — osc_prompt_open requires running state
    expect(sm.current()).toBe('idle')
  })

  it('transition returns new state', () => {
    const sm = new AgentStateMachine()
    const next = sm.transition('first_output')
    expect(next).toBe('running')
  })
})

// ─── resolveAgentSpec ─────────────────────────────────────────────────────────

describe('resolveAgentSpec', () => {
  it('resolves exact "claude" → claude binary', () => {
    const spec = resolveAgentSpec('claude')
    expect(spec?.binary).toBe('claude')
    expect(spec?.apiKeyEnv).toBe('ANTHROPIC_API_KEY')
  })

  it('resolves "claude-opus-4-5" via prefix → claude binary', () => {
    const spec = resolveAgentSpec('claude-opus-4-5')
    expect(spec?.binary).toBe('claude')
    expect(spec?.apiKeyEnv).toBe('ANTHROPIC_API_KEY')
  })

  it('resolves "claude-haiku-3-5" via prefix → claude binary', () => {
    const spec = resolveAgentSpec('claude-haiku-3-5')
    expect(spec?.binary).toBe('claude')
  })

  it('resolves "codex" → codex binary', () => {
    const spec = resolveAgentSpec('codex')
    expect(spec?.binary).toBe('codex')
    expect(spec?.apiKeyEnv).toBe('OPENAI_API_KEY')
  })

  it('resolves "gemini" → gemini binary', () => {
    const spec = resolveAgentSpec('gemini')
    expect(spec?.binary).toBe('gemini')
    expect(spec?.apiKeyEnv).toBe('GOOGLE_API_KEY')
  })

  it('resolves "gemini-2.0-flash" via prefix → gemini binary', () => {
    const spec = resolveAgentSpec('gemini-2.0-flash')
    expect(spec?.binary).toBe('gemini')
  })

  it('resolves "ollama" → ollama binary (local inference)', () => {
    const spec = resolveAgentSpec('ollama')
    expect(spec?.binary).toBe('ollama')
    expect(spec?.localInference).toBe(true)
    expect(spec?.apiKeyEnv).toBeNull()
  })

  it('resolves "ollama-llama3" via ollama prefix → ollama binary', () => {
    const spec = resolveAgentSpec('ollama-llama3')
    expect(spec?.binary).toBe('ollama')
    expect(spec?.localInference).toBe(true)
  })

  it('resolves "opencode" → opencode binary', () => {
    const spec = resolveAgentSpec('opencode')
    expect(spec?.binary).toBe('opencode')
    expect(spec?.apiKeyEnv).toBeNull()
  })

  it('returns undefined for unknown model', () => {
    expect(resolveAgentSpec('unknown-model-xyz-abc')).toBeUndefined()
  })

  it('returns undefined for empty string', () => {
    expect(resolveAgentSpec('')).toBeUndefined()
  })
})

// ─── buildAgentEnv ────────────────────────────────────────────────────────────

describe('buildAgentEnv', () => {
  const baseReq = {
    model:       'claude',
    trustPreset: 'standard',
    cwd:         tmpdir(),
    taskId:      'task-123',
    userId:      'user-456',
    projectId:   'proj-789',
    accountId:   'acc-001',
  }

  const claudeSpec = { binary: 'claude', buildArgs: () => [], apiKeyEnv: 'ANTHROPIC_API_KEY' }
  const ollamaSpec = { binary: 'ollama', buildArgs: () => [], apiKeyEnv: null, localInference: true as const }

  it('sets per-user GH_CONFIG_DIR', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'user-alice' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.GH_CONFIG_DIR).toContain('user-alice')
    expect(env.GH_CONFIG_DIR).toContain('.config/gh/')
  })

  it('sets per-user GLAB_CONFIG_DIR', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'user-bob' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.GLAB_CONFIG_DIR).toContain('user-bob')
    expect(env.GLAB_CONFIG_DIR).toContain('.config/glab-cli/')
  })

  it('sets ORCA_TASK_ID from request', async () => {
    const env = await buildAgentEnv({ ...baseReq, taskId: 'task-abc' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_TASK_ID).toBe('task-abc')
  })

  it('sets ORCA_PROJECT_ID from request', async () => {
    const env = await buildAgentEnv({ ...baseReq, projectId: 'proj-xyz' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_PROJECT_ID).toBe('proj-xyz')
  })

  it('sets ORCA_USER_ID from request', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'user-test' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_USER_ID).toBe('user-test')
  })

  it('injects API key when spec.apiKeyEnv is set and apiKey provided', async () => {
    const env = await buildAgentEnv({ ...baseReq }, claudeSpec, MOCK_CONFIG, 'sk-ant-test-key-123')
    expect(env.ANTHROPIC_API_KEY).toBe('sk-ant-test-key-123')
  })

  it('does NOT inject API key when apiKey is null', async () => {
    const env = await buildAgentEnv({ ...baseReq }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ANTHROPIC_API_KEY).toBeUndefined()
  })

  it('sets OLLAMA_HOST for local inference spec', async () => {
    const env = await buildAgentEnv({ ...baseReq }, ollamaSpec, MOCK_CONFIG, null)
    expect(env.OLLAMA_HOST).toBeDefined()
    expect(env.OPENAI_BASE_URL).toBeDefined()
  })

  it('does NOT set OLLAMA_HOST for non-local-inference spec', async () => {
    const env = await buildAgentEnv({ ...baseReq }, claudeSpec, MOCK_CONFIG, null)
    expect(env.OLLAMA_HOST).toBeUndefined()
  })

  it('merges extraEnv into result', async () => {
    const env = await buildAgentEnv(
      { ...baseReq, extraEnv: { MY_CUSTOM_VAR: 'custom-value-123' } },
      claudeSpec, MOCK_CONFIG, null
    )
    expect(env.MY_CUSTOM_VAR).toBe('custom-value-123')
  })

  it('different users get different GH_CONFIG_DIR', async () => {
    const env1 = await buildAgentEnv({ ...baseReq, userId: 'alice' }, claudeSpec, MOCK_CONFIG, null)
    const env2 = await buildAgentEnv({ ...baseReq, userId: 'bob' }, claudeSpec, MOCK_CONFIG, null)
    expect(env1.GH_CONFIG_DIR).not.toBe(env2.GH_CONFIG_DIR)
  })
})

// ─── handleAgentKill ──────────────────────────────────────────────────────────

describe('handleAgentKill', () => {
  it('returns InvalidParams for missing ptyId', async () => {
    const res = await handleAgentKill(1, {}, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns ok:true for non-existent ptyId (idempotent)', async () => {
    const res = await handleAgentKill(1, { ptyId: 'pty-nonexistent-xyz' }, MOCK_CONFIG, MOCK_LOG) as {
      result?: { ok: boolean; note?: string }
    }
    expect(res.result?.ok).toBe(true)
  })
})

// ─── handleAgentSpawn — validation ───────────────────────────────────────────

describe('handleAgentSpawn — validation', () => {
  // Note: these tests do NOT spawn real PTY — they test validation before spawn

  const MOCK_WS = {
    readyState: 1,
    send: vi.fn(),
  } as unknown as import('ws').default

  const MOCK_WIRE = {
    sendSeq: 0,
    recvSeq: 0,
    frameKey: Buffer.alloc(32),
  } as unknown as import('../agent-wire').WireState

  it('returns InvalidParams for missing model', async () => {
    const res = await handleAgentSpawn(1, {
      taskId: 'task-1', userId: 'user-1', cwd: tmpdir()
      // model missing
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for missing taskId', async () => {
    const res = await handleAgentSpawn(1, {
      model: 'claude', userId: 'user-1', cwd: tmpdir()
      // taskId missing
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for unknown model', async () => {
    const res = await handleAgentSpawn(1, {
      model: 'totally-unknown-ai-xyz', taskId: 'task-1', userId: 'user-1', cwd: tmpdir()
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns ServerError when binary not in toolPath', async () => {
    // Ollama may not be installed — test with a random binary
    const res = await handleAgentSpawn(1, {
      model:   'opencode',      // opencode binary unlikely to be in test env
      taskId:  'task-1',
      userId:  'user-1',
      cwd:     tmpdir(),
      accountId: '',
    }, {
      ...MOCK_CONFIG,
      toolPath: '/nonexistent-path-for-test',  // empty PATH
    } as unknown as AgentConfig, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { error?: { code: number } }
    expect(res.error?.code).toBeDefined()  // ServerError or similar
  })
})
```

---

## Verify

```bash
pnpm test src/relay/__tests__/agent-spawner.test.ts
# Expected: ≥ 30 tests pass
# Note: handleAgentSpawn spawn tests skip if node-pty not available
```

---

## Done criteria

- [ ] `AgentStateMachine` — 9 tests (đủ 5 states, transitions, returns state)
- [ ] `resolveAgentSpec` — 11 tests (claude, codex, gemini, ollama, opencode, unknown, empty)
- [ ] `buildAgentEnv` — 11 tests (GH/GLAB per user, ORCA vars, API key inject, extraEnv, OLLAMA_HOST)
- [ ] `handleAgentKill` — 2 tests (missing ptyId, idempotent)
- [ ] `handleAgentSpawn` validation — 4 tests (missing params, unknown model, bad toolPath)
- [ ] Tất cả ≥ 30 tests pass (state machine + resolver là pure units → 100% pass)
