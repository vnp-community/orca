/**
 * sub-agent-spawner.test.ts — Tests cho SubAgentSpawner relay tier (CR-AG-12 / T19)
 *
 * ≥ 17 tests theo spec TDD-AG-12 §10
 *
 * Pure unit tests — không cần PTY thật, không cần WS thật.
 * Test SubAgentSpawner state machine, resolveAgentSpec, buildAgentEnv, handleAgentKill
 */
import { describe, it, expect, vi } from 'vitest'
import {
  SubAgentSpawner,
  resolveAgentSpec,
  buildAgentEnv,
  handleAgentKill,
} from '../agent-spawner'

// ── SubAgentSpawner — State Machine ────────────────────────────────────────────

describe('SubAgentSpawner', () => {
  it('initial state is idle', () => {
    expect(new SubAgentSpawner().getState()).toBe('idle')
  })

  it('idle → spawning OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning')
    expect(s.getState()).toBe('spawning')
  })

  it('idle → running throws (invalid transition)', () => {
    expect(() => new SubAgentSpawner().transition('running')).toThrow()
  })

  it('idle → stopped throws (invalid transition)', () => {
    expect(() => new SubAgentSpawner().transition('stopped')).toThrow()
  })

  it('spawning → running OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning')
    s.transition('running')
    expect(s.getState()).toBe('running')
  })

  it('spawning → error OK', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning')
    s.transition('error')
    expect(s.getState()).toBe('error')
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

  it('stopped → idle OK (reset cycle)', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning'); s.transition('running'); s.transition('stopping'); s.transition('stopped')
    s.transition('idle')
    expect(s.getState()).toBe('idle')
  })

  it('error → idle OK (recovery)', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning'); s.transition('error')
    s.transition('idle')
    expect(s.getState()).toBe('idle')
  })

  it('running → idle throws (skip stopping)', () => {
    const s = new SubAgentSpawner()
    s.transition('spawning'); s.transition('running')
    expect(() => s.transition('idle')).toThrow()
  })
})

// ── resolveAgentSpec ──────────────────────────────────────────────────────────

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

  it('gemini args includes --stream', () => {
    expect(resolveAgentSpec('gemini-2.5').args).toContain('--stream')
  })
})

// ── buildAgentEnv ─────────────────────────────────────────────────────────────

describe('buildAgentEnv', () => {
  it('returns object với ANTHROPIC_API_KEY', async () => {
    const env = await buildAgentEnv('acc-1', 'sk-xxx', '/repo')
    expect(env.ANTHROPIC_API_KEY).toBe('sk-xxx')
  })

  it('ORCA_AGENT_CWD = cwd param', async () => {
    const env = await buildAgentEnv('acc-1', 'sk-xxx', '/my/repo')
    expect(env.ORCA_AGENT_CWD).toBe('/my/repo')
  })

  it('ORCA_ACCOUNT_ID = accountId param', async () => {
    const env = await buildAgentEnv('my-account', 'key', '/cwd')
    expect(env.ORCA_ACCOUNT_ID).toBe('my-account')
  })

  it('all 3 provider keys have same value (shared apiKey)', async () => {
    const env = await buildAgentEnv('acc', 'shared-key', '/cwd')
    expect(env.ANTHROPIC_API_KEY).toBe('shared-key')
    expect(env.OPENAI_API_KEY).toBe('shared-key')
    expect(env.GEMINI_API_KEY).toBe('shared-key')
  })

  it('HOME và PATH are set', async () => {
    const env = await buildAgentEnv('a', 'k', '/c')
    expect(env.HOME).toBeDefined()
    expect(env.PATH).toBeDefined()
  })
})

// ── handleAgentKill ────────────────────────────────────────────────────────────

describe('handleAgentKill', () => {
  const mockConfig = { workDir: '/tmp' } as any
  const mockLog = { info: vi.fn(), error: vi.fn(), warn: vi.fn() } as any

  it('returns ok=true khi pty not found (already dead)', async () => {
    const result = await handleAgentKill(1, { ptyId: 'not-exist' }, mockConfig, mockLog) as any
    expect(result.result.ok).toBe(true)
  })

  it('returns note khi pty not found', async () => {
    const result = await handleAgentKill(1, { ptyId: 'gone-pty' }, mockConfig, mockLog) as any
    expect(result.result).toHaveProperty('note')
  })

  it('returns error khi thiếu ptyId', async () => {
    const result = await handleAgentKill(1, {}, mockConfig, mockLog) as any
    expect(result.error.code).toBeDefined()
  })

  it('returns error.message khi ptyId empty string', async () => {
    const result = await handleAgentKill(1, { ptyId: '' }, mockConfig, mockLog) as any
    expect(result.error.message).toContain('Missing ptyId')
  })

  it('id is forwarded in response (pass-through)', async () => {
    const result = await handleAgentKill(42, { ptyId: 'nonexistent' }, mockConfig, mockLog) as any
    expect(result.id).toBe(42)
  })
})
