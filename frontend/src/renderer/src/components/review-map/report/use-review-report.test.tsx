// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reviewReportWire } from './review-report-model.fixture'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: false }
const call = vi.fn()

vi.mock('@/store', () => ({ useAppStore: { getState: () => ({}) } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => 'env-1' }))
vi.mock('../../../hooks/useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../../../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ call }) }))

import { useReviewReport } from './use-review-report'

const opts = { worktreeId: 'wt', projectId: 'p', base: 'main' }

beforeEach(() => {
  vi.useFakeTimers()
  call.mockReset()
  flags.quality = true
})
afterEach(() => vi.useRealTimers())

describe('useReviewReport', () => {
  it('is unavailable and makes no RPC when the quality flag is off', async () => {
    flags.quality = false
    const { result } = renderHook(() => useReviewReport(opts))
    expect(result.current.available).toBe(false)
    expect(await result.current.fetchReport()).toEqual({ ok: false, reason: 'unavailable' })
    expect(call).not.toHaveBeenCalled()
  })

  it('is unavailable without a projectId', async () => {
    const { result } = renderHook(() => useReviewReport({ ...opts, projectId: null }))
    expect(result.current.available).toBe(false)
  })

  it('calls quality.report with project/worktree/base and parses the model', async () => {
    call.mockResolvedValue({ ok: true, result: { model: reviewReportWire(), warnings: [] } })
    const { result } = renderHook(() => useReviewReport(opts))
    let fetched: Awaited<ReturnType<typeof result.current.fetchReport>> | undefined
    await act(async () => {
      fetched = await result.current.fetchReport()
    })
    expect(call.mock.calls[0][1]).toBe('codeIntel.quality.report')
    expect(call.mock.calls[0][2]).toEqual({ projectId: 'p', worktreeId: 'wt', base: 'main' })
    expect(call.mock.calls[0][3]).toMatchObject({ environmentId: 'env-1' })
    expect(fetched).toMatchObject({ ok: true, model: { subject: { branch: 'feature/login' } } })
  })

  it('treats disabled kinds as silently unavailable', async () => {
    call.mockResolvedValue({ ok: false, error: { kind: 'disabled', message: 'x', data: null } })
    const { result } = renderHook(() => useReviewReport(opts))
    await act(async () => {
      expect(await result.current.fetchReport()).toEqual({ ok: false, reason: 'unavailable' })
    })
  })

  it('retries inProgress timeouts after retryAfterMs, then succeeds', async () => {
    call
      .mockResolvedValueOnce({ ok: false, error: { kind: 'unknown', message: '... {"inProgress":true}', data: { retryAfterMs: 3000 } } })
      .mockResolvedValueOnce({ ok: true, result: { model: reviewReportWire() } })
    const { result } = renderHook(() => useReviewReport(opts))
    let fetched: unknown
    await act(async () => {
      const pending = result.current.fetchReport().then((r) => (fetched = r))
      await vi.advanceTimersByTimeAsync(3100)
      await pending
    })
    expect(call).toHaveBeenCalledTimes(2)
    expect(fetched).toMatchObject({ ok: true })
  })

  it('gives up with an error for other failures', async () => {
    call.mockResolvedValue({ ok: false, error: { kind: 'unknown', message: 'boom', data: null } })
    const { result } = renderHook(() => useReviewReport(opts))
    await act(async () => {
      expect(await result.current.fetchReport()).toEqual({ ok: false, reason: 'error' })
    })
  })

  it('ignores a second click while a request is in flight', async () => {
    let resolve: (v: unknown) => void = () => {}
    call.mockReturnValue(new Promise((r) => (resolve = r)))
    const { result } = renderHook(() => useReviewReport(opts))
    let first: Promise<unknown> = Promise.resolve()
    await act(async () => {
      first = result.current.fetchReport()
      const second = await result.current.fetchReport()
      expect(second).toEqual({ ok: false, reason: 'aborted' })
    })
    expect(call).toHaveBeenCalledTimes(1)
    await act(async () => {
      resolve({ ok: true, result: { model: reviewReportWire() } })
      await first
    })
  })
})
