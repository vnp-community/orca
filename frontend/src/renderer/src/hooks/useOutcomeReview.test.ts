// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useOutcomeReview } from './useOutcomeReview'
import { emitRequestEvent } from '../lib/request-event-bus'

const review = (version: number) => ({
  reviewId: 'v1',
  requestId: 'r1',
  phase: 'design',
  state: 'ready',
  headKey: 'design',
  version,
  createdAt: '2026-10-20T10:00:00Z',
  updatedAt: '2026-10-20T10:00:00Z',
  model: { schemaVersion: 1, phase: 'design', items: [], diagrams: [] }
})
const run = (status: string) => ({
  runId: 'u1',
  reviewId: 'v1',
  kind: 'context',
  status,
  scope: 'design',
  startedAt: '2026-10-20T10:00:00Z'
})

type Script = Record<string, unknown>
function script(map: Script): void {
  callRequestRpc.mockImplementation(async (method: string) => {
    const v = map[method]
    if (v === undefined) {
      return { ok: false, error: { kind: 'unknown', message: `no script for ${method}` } }
    }
    return v && typeof v === 'object' && 'ok' in (v as object) ? v : { ok: true, value: v }
  })
}

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useOutcomeReview', () => {
  it('reads the review and its runs, and never asks for one to be built', async () => {
    script({
      'request.review.get': { found: true, review: review(2) },
      'request.review.runs': { runs: [run('succeeded')] }
    })
    const { result } = renderHook(() => useOutcomeReview('r1'))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current.review?.version).toBe(2)
    expect(result.current.runs).toHaveLength(1)
    expect(result.current.activeRun).toBeNull()
    const methods = callRequestRpc.mock.calls.map((c) => c[0])
    expect(methods).not.toContain('request.review.generate')
    expect(methods).not.toContain('request.review.estimate')
  })

  it('reopening reads only: with no review it estimates and does not generate', async () => {
    script({
      'request.review.get': { found: false },
      'request.review.estimate': {
        phase: 'design',
        executionFinished: false,
        reviewExists: false,
        currentInputsDigest: 'd'
      }
    })
    const { result } = renderHook(() => useOutcomeReview('r1'))
    await waitFor(() => expect(result.current.status).toBe('none'))
    expect(result.current.estimate?.phase).toBe('design')
    expect(callRequestRpc.mock.calls.map((c) => c[0])).not.toContain('request.review.generate')
  })

  it('does not read while disabled or without a Request', () => {
    renderHook(() => useOutcomeReview('r1', { enabled: false }))
    renderHook(() => useOutcomeReview(null))
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('maps the documented refusals to a status, and an older gateway to unsupported', async () => {
    for (const [error, status] of [
      [{ kind: 'unknown', backendCode: 'REQUEST_REVIEW_DISABLED', message: 'x' }, 'disabled'],
      [
        { kind: 'invalid_state', backendCode: 'REQUEST_REVIEW_NOT_READY', message: 'x' },
        'not_ready'
      ],
      [{ kind: 'unsupported', message: 'x' }, 'unsupported'],
      [{ kind: 'network', message: 'x' }, 'error']
    ] as const) {
      callRequestRpc.mockReset()
      callRequestRpc.mockResolvedValue({ ok: false, error })
      const { result, unmount } = renderHook(() => useOutcomeReview('r1'))
      await waitFor(() => expect(result.current.status).toBe(status))
      unmount()
    }
  })

  it('refreshes on a review event of this Request only', async () => {
    script({
      'request.review.get': { found: true, review: review(1) },
      'request.review.runs': { runs: [] }
    })
    const { result } = renderHook(() => useOutcomeReview('r1'))
    await waitFor(() => expect(result.current.review?.version).toBe(1))
    const before = callRequestRpc.mock.calls.length
    act(() => emitRequestEvent({ requestId: 'other', eventType: 'review.changed', occurredAt: '' }))
    act(() =>
      emitRequestEvent({ requestId: 'r1', eventType: 'request.status_changed', occurredAt: '' })
    )
    expect(callRequestRpc.mock.calls.length).toBe(before)
    script({
      'request.review.get': { found: true, review: review(2) },
      'request.review.runs': { runs: [] }
    })
    act(() => emitRequestEvent({ requestId: 'r1', eventType: 'review.changed', occurredAt: '' }))
    await waitFor(() => expect(result.current.review?.version).toBe(2))
  })

  it('polls only while a run is going', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    script({
      'request.review.get': { found: true, review: review(1) },
      'request.review.runs': { runs: [run('running')] }
    })
    const { result } = renderHook(() => useOutcomeReview('r1', { pollMs: 1000 }))
    await vi.waitFor(() => expect(result.current.activeRun?.runId).toBe('u1'))
    const before = callRequestRpc.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    expect(callRequestRpc.mock.calls.length).toBeGreaterThanOrEqual(before + 4)
    script({
      'request.review.get': { found: true, review: review(1) },
      'request.review.runs': { runs: [run('succeeded')] }
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100)
    })
    await vi.waitFor(() => expect(result.current.activeRun).toBeNull())
    const settled = callRequestRpc.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000)
    })
    expect(callRequestRpc.mock.calls.length).toBe(settled)
  })

  it('generate sends only what was asked and reads again afterwards', async () => {
    script({
      'request.review.get': { found: false },
      'request.review.estimate': { phase: 'design', currentInputsDigest: 'd' },
      'request.review.generate': {
        review: review(1),
        created: true,
        reused: false,
        contextCached: false,
        drawingsCached: 0,
        drawingsPending: 0,
        run: run('running')
      }
    })
    const { result } = renderHook(() => useOutcomeReview('r1'))
    await waitFor(() => expect(result.current.status).toBe('none'))
    let out: Awaited<ReturnType<typeof result.current.generate>> | undefined
    await act(async () => {
      out = await result.current.generate({ runContext: false })
    })
    expect(out?.ok).toBe(true)
    const call = callRequestRpc.mock.calls.find((c) => c[0] === 'request.review.generate')
    expect(call?.[1]).toEqual({ id: 'r1', runContext: false })
    const gets = callRequestRpc.mock.calls.filter((c) => c[0] === 'request.review.get').length
    expect(gets).toBeGreaterThanOrEqual(2)
  })

  it('reports a refused generate with its code and leaves the screen as it was', async () => {
    script({
      'request.review.get': { found: false },
      'request.review.estimate': { phase: 'design', currentInputsDigest: 'd' },
      'request.review.generate': {
        ok: false,
        error: { kind: 'invalid_state', backendCode: 'REQUEST_REVIEW_SKILL_MISSING', message: 'm' }
      }
    })
    const { result } = renderHook(() => useOutcomeReview('r1'))
    await waitFor(() => expect(result.current.status).toBe('none'))
    let out: Awaited<ReturnType<typeof result.current.generate>> | undefined
    await act(async () => {
      out = await result.current.generate()
    })
    expect(out).toMatchObject({ ok: false, error: { code: 'REQUEST_REVIEW_SKILL_MISSING' } })
    expect(result.current.status).toBe('none')
    expect(result.current.generating).toBe(false)
  })
})
