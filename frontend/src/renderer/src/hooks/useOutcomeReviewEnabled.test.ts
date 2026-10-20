// @vitest-environment happy-dom
import { renderHook, waitFor, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { resetOutcomeReviewFlagCache, useOutcomeReviewEnabled } from './useOutcomeReviewEnabled'

const settings = (enabled: unknown) => ({
  ok: true,
  value: {
    projectId: 'p1',
    engine: 'native',
    openspecMinVersion: '',
    solutionAgent: false,
    executionMode: 'step_by_step',
    outcomeReviewEnabled: enabled,
    version: 1
  }
})

beforeEach(() => {
  callRequestRpc.mockReset()
  resetOutcomeReviewFlagCache()
})
afterEach(cleanup)

describe('useOutcomeReviewEnabled', () => {
  it('is on only when the gateway says so', async () => {
    callRequestRpc.mockResolvedValue(settings(true))
    const { result } = renderHook(() => useOutcomeReviewEnabled('p1'))
    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.enabled).toBe(true))
    expect(callRequestRpc).toHaveBeenCalledWith('request.engineSettings', { projectId: 'p1' })
  })

  it('treats a missing flag (an older gateway), a failed read and no project as off', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { ...settings(true).value, outcomeReviewEnabled: undefined }
    })
    const old = renderHook(() => useOutcomeReviewEnabled('p1'))
    await waitFor(() => expect(old.result.current.loading).toBe(false))
    expect(old.result.current.enabled).toBe(false)

    resetOutcomeReviewFlagCache()
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'forbidden', message: 'x' } })
    const failed = renderHook(() => useOutcomeReviewEnabled('p2'))
    await waitFor(() => expect(failed.result.current.loading).toBe(false))
    expect(failed.result.current.enabled).toBe(false)

    const none = renderHook(() => useOutcomeReviewEnabled(null))
    expect(none.result.current).toEqual({ enabled: false, loading: false })
  })

  it('reads a project once for a while, but retries a failed read', async () => {
    callRequestRpc.mockResolvedValue(settings(true))
    const first = renderHook(() => useOutcomeReviewEnabled('p1'))
    await waitFor(() => expect(first.result.current.enabled).toBe(true))
    first.unmount()
    const again = renderHook(() => useOutcomeReviewEnabled('p1'))
    await waitFor(() => expect(again.result.current.enabled).toBe(true))
    expect(callRequestRpc).toHaveBeenCalledTimes(1)

    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'network', message: 'x' } })
    const failing = renderHook(() => useOutcomeReviewEnabled('p3'))
    await waitFor(() => expect(failing.result.current.loading).toBe(false))
    failing.unmount()
    callRequestRpc.mockResolvedValue(settings(true))
    const retry = renderHook(() => useOutcomeReviewEnabled('p3'))
    await waitFor(() => expect(retry.result.current.enabled).toBe(true))
  })
})
