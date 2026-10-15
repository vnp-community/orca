// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { usePlanRunInFlight, PLAN_RUN_PRESENCE_POLL_MS } from './usePlanRunInFlight'
import type { OrcaRequest } from '../../../shared/request-types'

const planning = { id: 'r1', status: 'planning', planTaskId: undefined } as OrcaRequest

beforeEach(() => callRequestRpc.mockReset())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usePlanRunInFlight', () => {
  it('is true while the latest plan run is running, and says so once it ends', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue({ ok: true, value: { status: 'running' } })
    const onFinished = vi.fn()
    const { result } = renderHook(() => usePlanRunInFlight(planning, onFinished))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(result.current).toBe(true)
    expect(callRequestRpc.mock.calls[0][1]).toEqual({ id: 'r1' })

    callRequestRpc.mockResolvedValue({ ok: true, value: { status: 'succeeded' } })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PLAN_RUN_PRESENCE_POLL_MS + 10)
    })
    expect(result.current).toBe(false)
    expect(onFinished).toHaveBeenCalledTimes(1)
  })

  it('is false, and silent, when the Request never ran a plan', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'not_found' } })
    const onFinished = vi.fn()
    const { result } = renderHook(() => usePlanRunInFlight(planning, onFinished))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(result.current).toBe(false)
    expect(onFinished).not.toHaveBeenCalled()
  })

  it('does not ask at all once the Request has a plan or is past planning', () => {
    renderHook(() => usePlanRunInFlight({ ...planning, planTaskId: 'plan1' }, vi.fn()))
    renderHook(() => usePlanRunInFlight({ ...planning, status: 'executing' }, vi.fn()))
    expect(callRequestRpc).not.toHaveBeenCalled()
  })
})
