// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reviewReportFixture } from './review-report-model.fixture'
import { REVIEW_REPORT_START_MARKER } from './review-report-markdown'

const state = vi.hoisted(() => ({ available: true, fetchResult: null as unknown }))
const toastError = vi.hoisted(() => vi.fn())

vi.mock('sonner', () => ({ toast: { error: toastError, success: vi.fn() } }))
vi.mock('./use-review-report', () => ({
  useReviewReport: () => ({
    available: state.available,
    loading: false,
    fetchReport: async () => state.fetchResult
  })
}))

import { useInsertReviewReport } from './use-insert-review-report'

const base = { worktreeId: 'wt', projectId: 'p', provider: 'gitlab' as const }

beforeEach(() => {
  state.available = true
  state.fetchResult = { ok: true, model: reviewReportFixture() }
  toastError.mockReset()
})

describe('useInsertReviewReport', () => {
  it('is undefined when the report is unavailable (button hidden)', () => {
    state.available = false
    const { result } = renderHook(() => useInsertReviewReport({ ...base, body: '', setBody: vi.fn() }))
    expect(result.current).toBeUndefined()
  })

  it('merges into the current description and keeps the user text', async () => {
    const setBody = vi.fn()
    const { result } = renderHook(() => useInsertReviewReport({ ...base, body: 'my notes', setBody }))
    await act(async () => {
      await result.current?.()
    })
    const next = setBody.mock.calls[0][0] as string
    expect(next.startsWith('my notes\n\n')).toBe(true)
    expect(next).toContain(REVIEW_REPORT_START_MARKER)
    expect(next).toContain('merge request') // GitLab wording
  })

  it('uses the latest body when it changed while the report was loading', async () => {
    const setBody = vi.fn()
    const { result, rerender } = renderHook((p) => useInsertReviewReport(p), {
      initialProps: { ...base, body: 'old', setBody }
    })
    const insert = result.current!
    rerender({ ...base, body: 'typed meanwhile', setBody })
    await act(async () => {
      await insert()
    })
    expect(setBody.mock.calls[0][0]).toContain('typed meanwhile')
  })

  it('shows an error toast and does not touch the body when the fetch fails', async () => {
    state.fetchResult = { ok: false, reason: 'error' }
    const setBody = vi.fn()
    const { result } = renderHook(() => useInsertReviewReport({ ...base, body: 'x', setBody }))
    await act(async () => {
      await result.current?.()
    })
    expect(setBody).not.toHaveBeenCalled()
    expect(toastError).toHaveBeenCalledTimes(1)
  })

  it('stays silent when the report is unavailable for this worktree', async () => {
    state.fetchResult = { ok: false, reason: 'unavailable' }
    const setBody = vi.fn()
    const { result } = renderHook(() => useInsertReviewReport({ ...base, body: 'x', setBody }))
    await act(async () => {
      await result.current?.()
    })
    expect(setBody).not.toHaveBeenCalled()
    expect(toastError).not.toHaveBeenCalled()
  })
})
