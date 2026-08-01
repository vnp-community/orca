// src/relay/__tests__/agent-spawner.test.ts
// Unit tests for agent-spawner.ts.
// Strategy:
//   - SubAgentSpawner, resolveAgentSpec, buildAgentEnv: pure unit tests (no PTY)
//   - handleAgentKill: pure unit (in-process PTY_REGISTRY check)
//   - handleAgentSpawn: validation-only tests (stop before actual node-pty spawn)
import { describe, it, expect, vi } from 'vitest'
import { tmpdir } from 'node:os'
import type WebSocket from 'ws'
import type { WireState } from '../agent-wire'
import {
  SubAgentSpawner,
  resolveAgentSpec,
  buildAgentEnv,
  handleAgentKill,
  handleAgentSpawn,
} from '../agent-spawner'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const MOCK_CONFIG: AgentConfig = {
  mode:                  'direct-websocket',
  orcaUrl:               '',
  agentToken:            '',
  agentPort:             6799,
  devServerId:           'test-server',
  logLevel:              'info',
  workDir:               tmpdir(),
  toolPath:              '/usr/local/bin:/usr/bin:/bin',
  toolEnv:               { PATH: '/usr/local/bin:/usr/bin:/bin' },
  credentialDir:         tmpdir(),
  tlsRejectUnauthorized: true,
}

const MOCK_LOG: AgentLogger = {
  info:  vi.fn(),
  warn:  vi.fn(),
  error: vi.fn(),
  debug: vi.fn(),
}

const MOCK_WS = {
  readyState: 1,
  send:       vi.fn(),
} as unknown as WebSocket

const MOCK_WIRE = {
  sendSeq: 0,
  recvSeq: 0,
  frameKey: Buffer.alloc(32),
} as unknown as WireState

// ─── SubAgentSpawner ─────────────────────────────────────────────────────────
describe('SubAgentSpawner', () => {
  it('starts in idle state', () => {
    const sm = new SubAgentSpawner()
    expect(sm.getState()).toBe('idle')
  })

  it('idle → spawning', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    expect(sm.getState()).toBe('spawning')
  })

  it('spawning → running', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('running')
    expect(sm.getState()).toBe('running')
  })

  it('running → stopping', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('running')
    sm.transition('stopping')
    expect(sm.getState()).toBe('stopping')
  })

  it('stopping → stopped', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('running')
    sm.transition('stopping')
    sm.transition('stopped')
    expect(sm.getState()).toBe('stopped')
  })

  it('spawning → error (e.g. binary not found)', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('error')
    expect(sm.getState()).toBe('error')
  })

  it('running → error', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('running')
    sm.transition('error')
    expect(sm.getState()).toBe('error')
  })

  it('throws on invalid transition idle → running', () => {
    const sm = new SubAgentSpawner()
    expect(() => sm.transition('running')).toThrow(/invalid transition/)
  })

  it('error → idle (reset)', () => {
    const sm = new SubAgentSpawner()
    sm.transition('spawning')
    sm.transition('error')
    sm.transition('idle')
    expect(sm.getState()).toBe('idle')
  })
})

// ─── resolveAgentSpec ─────────────────────────────────────────────────────────
describe('resolveAgentSpec', () => {
  it('resolves exact "claude" → binary=claude', () => {
    const spec = resolveAgentSpec('claude')
    expect(spec?.binary).toBe('claude')
    expect(spec?.apiKeyEnvVar).toBe('ANTHROPIC_API_KEY')
  })

  it('resolves "claude-opus-4-5" via prefix → claude binary', () => {
    expect(resolveAgentSpec('claude-opus-4-5')?.binary).toBe('claude')
  })

  it('resolves "claude-haiku-3-5" via prefix → claude binary', () => {
    expect(resolveAgentSpec('claude-haiku-3-5')?.binary).toBe('claude')
  })

  it('resolves exact "codex" → binary=codex', () => {
    const spec = resolveAgentSpec('codex')
    expect(spec?.binary).toBe('codex')
    expect(spec?.apiKeyEnvVar).toBe('OPENAI_API_KEY')
  })

  it('resolves exact "gemini" → binary=gemini', () => {
    const spec = resolveAgentSpec('gemini')
    expect(spec?.binary).toBe('gemini')
    expect(spec?.apiKeyEnvVar).toBe('GEMINI_API_KEY')
  })

  it('resolves "gemini-2.0-flash" via prefix → gemini binary', () => {
    expect(resolveAgentSpec('gemini-2.0-flash')?.binary).toBe('gemini')
  })

  it('resolves "opencode" → binary=opencode, no apiKeyEnv', () => {
    const spec = resolveAgentSpec('opencode')
    expect(spec?.binary).toBe('opencode')
    expect(spec?.apiKeyEnvVar).toBeNull()
  })

  it('resolves "ollama" → binary=ollama, localInference=true', () => {
    const spec = resolveAgentSpec('ollama')
    expect(spec?.binary).toBe('ollama')
    expect(spec?.localInference).toBe(true)
    expect(spec?.apiKeyEnvVar).toBeNull()
  })

  it('resolves "ollama-llama3" via ollama catch-all → ollama binary', () => {
    const spec = resolveAgentSpec('ollama-llama3')
    expect(spec?.binary).toBe('ollama')
    expect(spec?.localInference).toBe(true)
  })

  it('returns undefined for unknown model', () => {
    expect(resolveAgentSpec('totally-unknown-xyz-model')).toBeUndefined()
  })

  it('returns undefined for empty string', () => {
    expect(resolveAgentSpec('')).toBeUndefined()
  })

  it('claude spec has buildArgs function', () => {
    const spec = resolveAgentSpec('claude')
    expect(typeof spec?.buildArgs).toBe('function')
  })
})

// ─── buildAgentEnv ────────────────────────────────────────────────────────────
describe('buildAgentEnv', () => {
  const baseReq = {
    model:       'claude',
    trustPreset: 'standard',
    cwd:         tmpdir(),
    taskId:      'task-test-001',
    userId:      'user-test-001',
    projectId:   'proj-test-001',
    accountId:   'acc-test-001',
    extraEnv:    {},
  }

  const claudeSpec = { binary: 'claude', buildArgs: () => [], apiKeyEnv: 'ANTHROPIC_API_KEY' }
  const ollamaSpec = {
    binary: 'ollama', buildArgs: () => [], apiKeyEnv: null as null,
    localInference: true as const,
  }

  it('sets per-user GH_CONFIG_DIR', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'alice' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.GH_CONFIG_DIR).toContain('alice')
    expect(env.GH_CONFIG_DIR).toMatch(/\.config\/gh\/alice\/$/)
  })

  it('sets per-user GLAB_CONFIG_DIR', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'bob' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.GLAB_CONFIG_DIR).toContain('bob')
    expect(env.GLAB_CONFIG_DIR).toMatch(/\.config\/glab-cli\/bob\/$/)
  })

  it('different users get different GH_CONFIG_DIR', async () => {
    const envA = await buildAgentEnv({ ...baseReq, userId: 'alice' }, claudeSpec, MOCK_CONFIG, null)
    const envB = await buildAgentEnv({ ...baseReq, userId: 'bob' }, claudeSpec, MOCK_CONFIG, null)
    expect(envA.GH_CONFIG_DIR).not.toBe(envB.GH_CONFIG_DIR)
  })

  it('sets ORCA_TASK_ID', async () => {
    const env = await buildAgentEnv({ ...baseReq, taskId: 'task-xyz-123' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_TASK_ID).toBe('task-xyz-123')
  })

  it('sets ORCA_PROJECT_ID', async () => {
    const env = await buildAgentEnv({ ...baseReq, projectId: 'proj-abc-456' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_PROJECT_ID).toBe('proj-abc-456')
  })

  it('sets ORCA_USER_ID', async () => {
    const env = await buildAgentEnv({ ...baseReq, userId: 'user-inject' }, claudeSpec, MOCK_CONFIG, null)
    expect(env.ORCA_USER_ID).toBe('user-inject')
  })

  it('injects API key when spec.apiKeyEnv is set and apiKey provided', async () => {
    const env = await buildAgentEnv(baseReq, claudeSpec, MOCK_CONFIG, 'sk-ant-test-key-999')
    expect(env.ANTHROPIC_API_KEY).toBe('sk-ant-test-key-999')
  })

  it('does NOT set API key env when apiKey is null', async () => {
    const env = await buildAgentEnv(baseReq, claudeSpec, MOCK_CONFIG, null)
    expect(env.ANTHROPIC_API_KEY).toBeUndefined()
  })

  it('sets OLLAMA_HOST for local inference spec', async () => {
    const env = await buildAgentEnv(baseReq, ollamaSpec, MOCK_CONFIG, null)
    expect(env.OLLAMA_HOST).toBeDefined()
  })

  it('sets OPENAI_BASE_URL for local inference spec', async () => {
    const env = await buildAgentEnv(baseReq, ollamaSpec, MOCK_CONFIG, null)
    expect(env.OPENAI_BASE_URL).toBeDefined()
  })

  it('does NOT set OLLAMA_HOST for non-local-inference spec', async () => {
    const env = await buildAgentEnv(baseReq, claudeSpec, MOCK_CONFIG, null)
    expect(env.OLLAMA_HOST).toBeUndefined()
  })

  it('merges extraEnv into result', async () => {
    const env = await buildAgentEnv(
      { ...baseReq, extraEnv: { MY_CUSTOM_VAR: 'custom-value-xyz' } },
      claudeSpec, MOCK_CONFIG, null
    )
    expect(env.MY_CUSTOM_VAR).toBe('custom-value-xyz')
  })

  it('sets PATH from config.toolPath', async () => {
    const env = await buildAgentEnv(baseReq, claudeSpec, MOCK_CONFIG, null)
    expect(env.PATH).toContain('/usr/bin')
  })
})

// ─── handleAgentKill — validation ────────────────────────────────────────────
describe('handleAgentKill', () => {
  it('returns InvalidParams (-32602) for missing ptyId', async () => {
    const resp = await handleAgentKill(1, {}, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('ptyId')
  })

  it('returns { ok: true } for non-existent ptyId (idempotent)', async () => {
    const resp = await handleAgentKill(1, { ptyId: 'pty-nonexistent-xyz-123' }, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.result.ok).toBe(true)
  })

  it('returns note about pty not found when idempotent', async () => {
    const resp = await handleAgentKill(1, { ptyId: 'pty-nonexistent-abc-456' }, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.result.note).toContain('not found')
  })

  it('response always has jsonrpc 2.0 format', async () => {
    const resp = await handleAgentKill(1, { ptyId: 'pty-any' }, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.id).toBe(1)
  })
})

// ─── handleAgentSpawn — validation only (no actual PTY spawn) ─────────────────
describe('handleAgentSpawn — validation', () => {
  it('returns InvalidParams for missing model', async () => {
    const resp = await handleAgentSpawn(1, {
      taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('model')
  })

  it('returns InvalidParams for missing taskId', async () => {
    const resp = await handleAgentSpawn(1, {
      model: 'claude', userId: 'user-1', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for missing userId', async () => {
    const resp = await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for missing cwd', async () => {
    const resp = await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1',
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for unknown model', async () => {
    const resp = await handleAgentSpawn(1, {
      model: 'unknown-ai-xyz-abc', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unknown model')
  })

  it('returns ServerError when binary not in toolPath', async () => {
    const restrictedConfig = {
      ...MOCK_CONFIG,
      toolPath: '/nonexistent-test-path-xyz-abc',
    } as AgentConfig
    const resp = await handleAgentSpawn(1, {
      model: 'opencode',
      taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
      accountId: '',
    }, restrictedConfig, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.error).toBeDefined()
  })

  it('response always has jsonrpc 2.0 format', async () => {
    // Even for validation errors
    const resp = await handleAgentSpawn(1, {}, MOCK_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.id).toBe(1)
  })
})
