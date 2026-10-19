// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { RequestStatus } from '../../../shared/request-types'
import {
  POST_ANSWER_REFRESH_INTERVAL_MS,
  POST_ANSWER_REFRESH_MAX_TRIES,
  useRequestPostAnswerRefresh
} from './useRequestPostAnswerRefresh'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

function setup(initial: RequestStatus) {
  const refetch = vi.fn()
  const hook = renderHook(({ status }) => useRequestPostAnswerRefresh(status, refetch), {
    initialProps: { status: initial }
  })
  return { refetch, hook }
}

describe('useRequestPostAnswerRefresh', () => {
  it('does nothing until armed', async () => {
    const { refetch } = setup('awaiting_information')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POST_ANSWER_REFRESH_INTERVAL_MS * 5)
    })
    expect(refetch).not.toHaveBeenCalled()
  })

  it('re-reads on a timer while the Request is still working, then stops once it waits for a person', async () => {
    const { refetch, hook } = setup('awaiting_information')
    act(() => hook.result.current())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POST_ANSWER_REFRESH_INTERVAL_MS)
    })
    expect(refetch).toHaveBeenCalledTimes(1)
    hook.rerender({ status: 'analyzing' })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POST_ANSWER_REFRESH_INTERVAL_MS)
    })
    expect(refetch).toHaveBeenCalledTimes(2)
    hook.rerender({ status: 'awaiting_analysis_approval' })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POST_ANSWER_REFRESH_INTERVAL_MS * 5)
    })
    expect(refetch).toHaveBeenCalledTimes(2)
  })

  it('gives up after a bounded number of tries', async () => {
    const { refetch, hook } = setup('analyzing')
    act(() => hook.result.current())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(
        POST_ANSWER_REFRESH_INTERVAL_MS * (POST_ANSWER_REFRESH_MAX_TRIES + 10)
      )
    })
    expect(refetch).toHaveBeenCalledTimes(POST_ANSWER_REFRESH_MAX_TRIES)
  })
})
