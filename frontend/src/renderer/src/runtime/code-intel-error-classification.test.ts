import { describe, expect, it, vi } from 'vitest'

vi.mock('@/store', () => ({ useAppStore: { getState: () => ({}) } }))
vi.mock('../lib/code-intel-worktree-selector', () => ({ resolveCodeIntelSelector: vi.fn() }))

import { classifyCodeIntelError, LocalCodeIntelError } from './code-intel-client'

const fail = (code: string, message: string, data?: unknown) =>
  ({ ok: false, error: { code, message, ...(data ? { data } : {}) } }) as const

describe('classifyCodeIntelError', () => {
  it('splits the CODEINTEL_ prefix out of error.message (code is "internal")', () => {
    const r = classifyCodeIntelError(
      fail('internal', 'CODEINTEL_TIMEOUT: read timed out | {"retryAfterMs":5000,"inProgress":true}')
    )
    expect(r).toMatchObject({
      kind: 'timeout',
      code: 'CODEINTEL_TIMEOUT',
      message: 'read timed out',
      retryable: true,
      data: { retryAfterMs: 5000, inProgress: true }
    })
  })

  it.each([
    ['CODEINTEL_DISABLED: x', 'disabled'],
    ['CODEINTEL_QUALITY_GATE_DISABLED: x', 'quality-disabled'],
    ['CODEINTEL_NOT_AUTHORIZED: x', 'forbidden'],
    ['CODEINTEL_VERSION_CONFLICT: x | {"currentVersion":3}', 'conflict'],
    ['CODEINTEL_REINDEX_IN_PROGRESS: x | {"jobId":"j"}', 'reindex-in-progress'],
    ['CODEINTEL_REINDEX_COOLDOWN: x | {"retryAfterSeconds":300}', 'rate-limited'],
    ['CODEINTEL_RESPONSE_TOO_LARGE: x', 'too-large'],
    ['CODEINTEL_AMBIGUOUS_SYMBOL: x', 'ambiguous'],
    ['CODEINTEL_WORKTREE_NOT_FOUND: x', 'no-binding'],
    ['CODEINTEL_FUTURE: x', 'unknown']
  ])('%s -> %s', (message, kind) => {
    expect(classifyCodeIntelError(fail('internal', message)).kind).toBe(kind)
  })

  it('maps raw RPC codes when no prefix is present', () => {
    expect(classifyCodeIntelError(fail('method_not_found', 'nope')).kind).toBe('unsupported')
    expect(classifyCodeIntelError(fail('forbidden', 'nope')).kind).toBe('forbidden')
    expect(classifyCodeIntelError(fail('connection_refused', 'down')).kind).toBe('offline')
    expect(classifyCodeIntelError(fail('timeout', 't')).kind).toBe('offline')
  })

  it('ignores error.code === "internal" (not a semantic code)', () => {
    const r = classifyCodeIntelError(fail('internal', 'plain failure'))
    expect(r).toMatchObject({ kind: 'unknown', code: null, message: 'plain failure' })
  })

  it('retryable only for offline, rate-limited and timeout', () => {
    expect(classifyCodeIntelError(fail('connection_refused', 'down')).retryable).toBe(true)
    expect(classifyCodeIntelError(fail('internal', 'CODEINTEL_RATE_LIMITED: x')).retryable).toBe(true)
    expect(classifyCodeIntelError(fail('internal', 'CODEINTEL_DISABLED: x')).retryable).toBe(false)
  })

  it('classifies a thrown Error by its message prefix', () => {
    expect(classifyCodeIntelError(new Error('CODEINTEL_TOOL_FAILED: boom')).kind).toBe('tool-failed')
    expect(classifyCodeIntelError(new Error('socket hang up')).kind).toBe('unknown')
  })

  it('keeps the kind of a local rejection', () => {
    expect(classifyCodeIntelError(new LocalCodeIntelError('validation', 'bad')).kind).toBe('validation')
  })

  it('falls back to unknown for garbage', () => {
    expect(classifyCodeIntelError(undefined).kind).toBe('unknown')
    expect(classifyCodeIntelError({ ok: true }).kind).toBe('unknown')
  })
})
