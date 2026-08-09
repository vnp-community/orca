// src/relay/__tests__/agent-spawner.test.ts
// Unit tests for agent-spawner.ts.
// Strategy:
//   - SubAgentSpawner, resolveAgentSpec, buildAgentEnv: pure unit tests (no PTY)
//   - handleAgentKill: pure unit (in-process PTY_REGISTRY check)
//   - handleAgentSpawn: validation-only tests (stop before actual node-pty spawn)
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { tmpdir } from 'node:os'
import type WebSocket from 'ws'
import type { WireState } from '../agent-wire'
import {
  SubAgentSpawner,
  resolveAgentSpec,
  buildAgentEnv,
  handleAgentKill,
  handleAgentSpawn,
  handleAgentSendInput,
} from '../agent-spawner'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

// ── Mock agent-credential-store (CR-TRACE-016 parentSpanId threading test) ───
const readDecryptedKeyMock = vi.hoisted(() => vi.fn(async (): Promise<string | null> => 'decrypted-blob'))
vi.mock('../agent-credential-store', () => ({ readDecryptedKey: readDecryptedKeyMock }))

// ── Fake node-pty (CR-TRACE-002 agentOrch tracing tests) ─────────────────────
// Mirrors the FakePty helper already established in pty-agent-bridge.test.ts.
type FakeAgentPty = {
  onData: (cb: (data: string) => void) => void
  onExit: (cb: (e: { exitCode: number }) => void) => void
  kill: ReturnType<typeof vi.fn>
  write: ReturnType<typeof vi.fn>
  emitData: (data: string) => void
  emitExit: (exitCode: number) => void
}
function makeFakeAgentPty(): FakeAgentPty {
  let dataCb: ((data: string) => void) | null = null
  let exitCb: ((e: { exitCode: number }) => void) | null = null
  return {
    onData: (cb) => { dataCb = cb },
    onExit: (cb) => { exitCb = cb },
    kill: vi.fn(),
    write: vi.fn(),
    emitData: (data) => dataCb?.(data),
    emitExit: (exitCode) => exitCb?.({ exitCode }),
  }
}
let lastSpawnedAgentPty: FakeAgentPty | null = null
const agentSpawnMock = vi.fn((..._args: unknown[]) => {
  lastSpawnedAgentPty = makeFakeAgentPty()
  return lastSpawnedAgentPty
})
vi.mock('node-pty', () => ({ spawn: agentSpawnMock }))

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

// toolPath: '' → toolPathDirs.length === 0 → binaryExists check short-circuits
// true regardless of spec.binary, letting these tests reach the (mocked)
// node-pty.spawn() call instead of failing at the pre-spawn existence check.
const SPAWNABLE_CONFIG: AgentConfig = { ...MOCK_CONFIG, toolPath: '' }

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

  // CR-TRACE-016: correlates the fallback credential-read span with the
  // agent:spawn span that triggered it, via a plain parentSpanId param.
  it('threads parentSpanId through to readDecryptedKey (mocked)', async () => {
    readDecryptedKeyMock.mockClear()
    const specWithApiKeyEnvVar = { binary: 'claude', buildArgs: () => [], apiKeyEnvVar: 'ANTHROPIC_API_KEY' }
    // BUG-AG-HLD-002: buildAgentEnv() now throws when a credential exists but
    // no resolvedApiKey was provided (readDecryptedKeyMock resolves a truthy
    // blob) — see the dedicated "fails fast" describe block below. This test
    // only cares that parentSpanId reaches readDecryptedKey, not the outcome.
    await buildAgentEnv(baseReq, specWithApiKeyEnvVar, MOCK_CONFIG, null, MOCK_LOG, 'parent-span-xyz').catch(() => {})
    expect(readDecryptedKeyMock).toHaveBeenCalledWith(
      baseReq.accountId, MOCK_CONFIG, expect.anything(), 'parent-span-xyz'
    )
  })

  // BUG-AG-HLD-002: buildAgentEnv() must fail fast instead of injecting the
  // Layer-1 (still browser-encrypted) ciphertext as if it were a plaintext key.
  describe('fails fast instead of injecting ciphertext (BUG-AG-HLD-002)', () => {
    const specWithApiKeyEnvVar = { binary: 'claude', buildArgs: () => [], apiKeyEnvVar: 'ANTHROPIC_API_KEY' }

    it('throws "no plaintext resolvedApiKey was provided" when a credential exists but resolvedApiKey is absent', async () => {
      readDecryptedKeyMock.mockResolvedValueOnce('layer1-ciphertext-blob')
      await expect(
        buildAgentEnv(baseReq, specWithApiKeyEnvVar, MOCK_CONFIG, null)
      ).rejects.toThrow('no plaintext resolvedApiKey was provided')
    })

    it('throws "no credential found" when nothing is stored either', async () => {
      readDecryptedKeyMock.mockResolvedValueOnce(null)
      await expect(
        buildAgentEnv(baseReq, specWithApiKeyEnvVar, MOCK_CONFIG, null)
      ).rejects.toThrow(`no credential found for accountId=${baseReq.accountId}`)
    })

    it('never assigns the Layer-1 blob value to ANTHROPIC_API_KEY/OPENAI_API_KEY/GEMINI_API_KEY', async () => {
      readDecryptedKeyMock.mockResolvedValueOnce('layer1-ciphertext-blob')
      let env: Record<string, string> | undefined
      try {
        env = await buildAgentEnv(baseReq, specWithApiKeyEnvVar, MOCK_CONFIG, null)
      } catch {
        // expected — fail-fast throws instead of returning an env object
      }
      expect(env).toBeUndefined()
    })

    it('still injects resolvedApiKey as plaintext when provided (unaffected by this fix)', async () => {
      const env = await buildAgentEnv(baseReq, specWithApiKeyEnvVar, MOCK_CONFIG, 'sk-real-plaintext-key')
      expect(env.ANTHROPIC_API_KEY).toBe('sk-real-plaintext-key')
    })
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

// ─── handleAgentSpawn — agentOrch tracing (CR-TRACE-002) ─────────────────────
describe('handleAgentSpawn — agentOrch tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
    lastSpawnedAgentPty = null
  })
  afterEach(() => unregister())

  it('emits agentOrch:spawn when resumeId is absent (BL-AG-01)', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)
    expect(events.some(e => e.flow === 'agentOrch:spawn' && e.level === 'start')).toBe(true)
    expect(events.some(e => e.flow === 'agentOrch:resume')).toBe(false)
  })

  it('emits agentOrch:resume instead of agentOrch:spawn when resumeId is present (BL-AG-03)', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(), resumeId: 'sess-abc',
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)
    expect(events.some(e => e.flow === 'agentOrch:resume' && e.level === 'start')).toBe(true)
    expect(events.some(e => e.flow === 'agentOrch:spawn')).toBe(false)
  })

  it('does not emit a new span per pty.onData frame (BL-AG-05) — only one "first-output" step', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)

    for (let i = 0; i < 50; i++) lastSpawnedAgentPty!.emitData(`chunk-${i}\n`)

    const firstOutputSteps = events.filter(
      e => e.flow === 'agentOrch:spawn' && e.level === 'step' && e.label === 'first-output'
    )
    expect(firstOutputSteps).toHaveLength(1)
  })

  it('resumes span id from params._trace.id', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(), _trace: { id: 'resumed-spawn-1' },
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)
    const start = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'start')
    expect(start?.id).toBe('resumed-spawn-1')
  })

  it('emits ok() with exitCode on pty exit', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)
    lastSpawnedAgentPty!.emitExit(0)
    const ok = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'ok')
    expect(ok?.fields.exitCode).toBe(0)
  })
})

// ─── handleAgentKill — agentOrch:stop ─────────────────────────────────────────
describe('handleAgentKill — agentOrch:stop', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('emits agentOrch:stop span with ok() when pty found and killed', async () => {
    await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE)
    const spawnOk = events.find(e => e.flow === 'agentOrch:spawn' && e.level === 'ok')
    events = []
    const ptyId = (await handleAgentSpawn(2, {
      model: 'claude', taskId: 'task-2', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { result: { ptyId: string } }).result.ptyId
    events = []

    await handleAgentKill(3, { ptyId, signal: 'SIGTERM' }, SPAWNABLE_CONFIG, MOCK_LOG)
    const ok = events.find(e => e.flow === 'agentOrch:stop' && e.level === 'ok')
    expect(ok?.fields.ptyId).toBe(ptyId)
    expect(ok?.fields.signal).toBe('SIGTERM')
    void spawnOk
  })

  it('emits ok() with note=already dead when ptyId not in registry', async () => {
    await handleAgentKill(1, { ptyId: 'pty-nonexistent-xyz' }, MOCK_CONFIG, MOCK_LOG)
    const ok = events.find(e => e.flow === 'agentOrch:stop' && e.level === 'ok')
    expect(ok?.fields.note).toBe('already dead')
  })
})

// ─── handleAgentSendInput — agentOrch:stop (Ctrl+C only) ─────────────────────
describe('handleAgentSendInput — agentOrch:stop (Ctrl+C only)', () => {
  let events: TraceEvent[]
  let unregister: () => void
  let ptyId: string

  beforeEach(async () => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
    const resp = await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { result: { ptyId: string } }
    ptyId = resp.result.ptyId
    events = []
  })
  afterEach(() => unregister())

  it('emits agentOrch:stop span when data === "\\x03"', async () => {
    await handleAgentSendInput(2, { ptyId, data: '\x03' }, MOCK_CONFIG, MOCK_LOG)
    expect(events.some(e => e.flow === 'agentOrch:stop' && e.level === 'ok')).toBe(true)
  })

  it('does NOT emit any span for arbitrary interactive keystrokes', async () => {
    await handleAgentSendInput(2, { ptyId, data: 'a' }, MOCK_CONFIG, MOCK_LOG)
    expect(events.filter(e => e.flow === 'agentOrch:stop')).toHaveLength(0)
  })
})

// ─── handleAgentSendInput — agent:spawn tracing (CR-TRACE-005) ───────────────
// Distinct from agentOrch:stop above — this span (spawnerTracer, reused, not a
// new tracer) fires on EVERY call, not just Ctrl+C, so BL-CR-02/03 remote
// feedback into a PTY is traceable even for non-Ctrl+C data.
describe('handleAgentSendInput — agent:spawn tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void
  let ptyId: string

  beforeEach(async () => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
    const resp = await handleAgentSpawn(1, {
      model: 'claude', taskId: 'task-1', userId: 'user-1', cwd: tmpdir(),
    }, SPAWNABLE_CONFIG, MOCK_LOG, MOCK_WS, MOCK_WIRE) as { result: { ptyId: string } }
    ptyId = resp.result.ptyId
    events = []
  })
  afterEach(() => unregister())

  it('span.fail("missing ptyId") when ptyId is empty', async () => {
    await handleAgentSendInput(2, { data: 'x' }, MOCK_CONFIG, MOCK_LOG)
    const fail = events.find(e => e.flow === 'agent:spawn' && e.level === 'fail')
    expect(fail?.fields.err).toBe('missing ptyId')
  })

  it('span.fail("pty-not-found") when ptyId is not in PTY_REGISTRY', async () => {
    await handleAgentSendInput(2, { ptyId: 'pty-does-not-exist', data: 'x' }, MOCK_CONFIG, MOCK_LOG)
    const fail = events.find(e => e.flow === 'agent:spawn' && e.level === 'fail')
    expect(fail?.fields.err).toBe('pty-not-found')
  })

  it('span.ok({ptyId, bytes}) on successful write — never contains the data content', async () => {
    await handleAgentSendInput(2, { ptyId, data: 'secret keystroke content' }, MOCK_CONFIG, MOCK_LOG)
    const ok = events.find(e => e.flow === 'agent:spawn' && e.level === 'ok')
    expect(ok?.fields.ptyId).toBe(ptyId)
    expect(ok?.fields.bytes).toBe('secret keystroke content'.length)
    expect(JSON.stringify(events)).not.toContain('secret keystroke content')
  })
})
