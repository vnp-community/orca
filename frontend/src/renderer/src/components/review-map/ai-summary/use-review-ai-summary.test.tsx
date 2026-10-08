// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { manifestWire, summaryWire } from './ai-summary.fixture'
import { resetAiSummaryConsent } from './ai-summary-consent-state'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: true }
const call = vi.fn()

vi.mock('@/store', () => ({ useAppStore: { getState: () => ({}) } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('../../../hooks/useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../../../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call }) }))

import { useReviewAiSummary } from './use-review-ai-summary'

const args = { projectId: 'p', worktreeId: 'wt', locale: 'en' }
const dry = (extra: Record<string, unknown> = {}) => ({ ok: true, result: { manifest: manifestWire(), summary: null, ...extra } })
const full = () => ({ ok: true, result: { summary: summaryWire(), manifest: manifestWire(), cache: { hit: false } } })
const err = (kind: string, code: string | null = null, message = 'x', data: Record<string, unknown> | null = null) => ({
  ok: false,
  error: { kind, code, message, data }
})

beforeEach(() => {
  vi.useFakeTimers()
  call.mockReset()
  flags.ai = true
  resetAiSummaryConsent()
})
afterEach(() => vi.useRealTimers())

describe('useReviewAiSummary', () => {
  it('is hidden with zero RPCs when the AI flag is off', async () => {
    flags.ai = false
    const { result } = renderHook(() => useReviewAiSummary(args))
    expect(result.current.state).toBe('hidden')
    await act(async () => {
      await result.current.preview('metadata')
      await result.current.confirmAndGenerate()
    })
    expect(call).not.toHaveBeenCalled()
  })

  it('is hidden without a projectId', () => {
    const { result } = renderHook(() => useReviewAiSummary({ ...args, projectId: null }))
    expect(result.current.state).toBe('hidden')
  })

  it('preview is a dry run that never generates and waits for consent', async () => {
    call.mockResolvedValue(dry())
    const { result } = renderHook(() => useReviewAiSummary(args))
    expect(result.current.state).toBe('idle')
    await act(async () => {
      await result.current.preview('metadata')
    })
    expect(call).toHaveBeenCalledTimes(1)
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.summary')
    expect(call.mock.calls[0][2]).toMatchObject({ dryRun: true, level: 'metadata', locale: 'en' })
    expect(result.current.state).toBe('awaiting-consent')
    expect(result.current.manifest?.redactions).toBe(2)
    expect(result.current.view).toBeNull()
  })

  it('generates only after confirmation, with dryRun:false', async () => {
    call.mockResolvedValueOnce(dry()).mockResolvedValueOnce(full())
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
    })
    await act(async () => {
      await result.current.confirmAndGenerate({ forceRefresh: true })
    })
    expect(call.mock.calls[1][2]).toMatchObject({ dryRun: false, forceRefresh: true })
    expect(result.current.state).toBe('ready')
    expect(result.current.view?.summary?.model).toBe('claude-x')
  })

  it('skips the preview for a level already confirmed this session, but asks again for another level', async () => {
    call.mockResolvedValueOnce(dry()).mockResolvedValueOnce(full()).mockResolvedValueOnce(full())
    const { result } = renderHook(() => useReviewAiSummary({ ...args, maxLevel: 'diff' }))
    await act(async () => {
      await result.current.preview('metadata')
      await result.current.confirmAndGenerate()
    })
    await act(async () => {
      await result.current.preview('metadata')
    })
    expect(call.mock.calls[2][2]).toMatchObject({ dryRun: false })
    call.mockResolvedValueOnce(dry({ manifest: manifestWire({ level: 'diff' }) }))
    await act(async () => {
      await result.current.preview('diff')
    })
    expect(call.mock.calls[3][2]).toMatchObject({ dryRun: true, level: 'diff' })
    expect(result.current.state).toBe('awaiting-consent')
  })

  it('never sends diff data when the tenant only allows metadata', async () => {
    call.mockResolvedValue(dry())
    const { result } = renderHook(() => useReviewAiSummary(args))
    expect(result.current.allowedLevels).toEqual(['metadata'])
    await act(async () => {
      await result.current.preview('diff')
    })
    expect(call.mock.calls[0][2].level).toBe('metadata')
  })

  it('retries inProgress timeouts every retryAfterMs and then succeeds', async () => {
    call
      .mockResolvedValueOnce(dry())
      .mockResolvedValueOnce(err('timeout', 'CODEINTEL_TIMEOUT', 'inProgress', { retryAfterMs: 3000 }))
      .mockResolvedValueOnce(full())
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
    })
    let pending: Promise<void> = Promise.resolve()
    await act(async () => {
      pending = result.current.confirmAndGenerate()
      await vi.advanceTimersByTimeAsync(3100)
      await pending
    })
    expect(call).toHaveBeenCalledTimes(3)
    expect(result.current.state).toBe('ready')
  })

  it('gives up with a timeout error after 90 s of retries', async () => {
    call.mockResolvedValueOnce(dry())
    call.mockResolvedValue(err('timeout', 'CODEINTEL_TIMEOUT', 'inProgress', { retryAfterMs: 3000 }))
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
    })
    await act(async () => {
      const pending = result.current.confirmAndGenerate()
      await vi.advanceTimersByTimeAsync(95_000)
      await pending
    })
    expect(result.current.state).toBe('error')
    expect(result.current.error).toBe('timeout')
  })

  it('cancel drops a late result', async () => {
    let resolve: (v: unknown) => void = () => {}
    call.mockResolvedValueOnce(dry()).mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
    })
    await act(async () => {
      void result.current.confirmAndGenerate()
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(result.current.state).toBe('generating')
    await act(async () => {
      result.current.cancel()
      resolve(full())
      await vi.advanceTimersByTimeAsync(0)
    })
    expect(result.current.state).toBe('idle')
    expect(result.current.view).toBeNull()
  })

  it.each([
    ['ai-disabled', null, 'hidden'],
    ['quality-disabled', null, 'hidden'],
    ['forbidden', null, 'forbidden'],
    ['ai-error', 'CODEINTEL_AI_NO_RELAY', 'no-relay'],
    ['ai-error', 'CODEINTEL_AI_BAD_OUTPUT', 'bad-output'],
    ['rate-limited', null, 'rate-limited'],
    ['tool-failed', null, 'unknown']
  ])('maps %s/%s', async (kind, code, expected) => {
    call.mockResolvedValue(err(kind, code, 'x', kind === 'rate-limited' ? { retryAfterSeconds: 30 } : null))
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
    })
    if (expected === 'hidden') {
      expect(result.current.state).toBe('hidden')
    } else {
      expect(result.current.state).toBe('error')
      expect(result.current.error).toBe(expected)
    }
    if (kind === 'rate-limited') {expect(result.current.retryAfterSeconds).toBe(30)}
  })

  it('treats a generation response without a summary as bad output, never showing raw text', async () => {
    call.mockResolvedValueOnce(dry()).mockResolvedValueOnce({ ok: true, result: { summary: { raw: 'oops' } } })
    const { result } = renderHook(() => useReviewAiSummary(args))
    await act(async () => {
      await result.current.preview('metadata')
      await result.current.confirmAndGenerate()
    })
    expect(result.current.error).toBe('bad-output')
    expect(result.current.view).toBeNull()
  })
})
