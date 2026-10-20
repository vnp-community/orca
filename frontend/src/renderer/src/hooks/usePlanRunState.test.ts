// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import {
  usePlanRunState,
  PLAN_AUTO_START_GRACE_MS,
  PLAN_RUN_STATE_POLL_MS
} from './usePlanRunState'

type Req = { id: string; status: string }
const planning: Req = { id: 'r1', status: 'planning' }
const ok = (value: unknown) => ({ ok: true, value })
const advance = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })

beforeEach(() => callRequestRpc.mockReset())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usePlanRunState', () => {
  it('asks at once, then every poll, and is busy while the run runs', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ runId: 'a', status: 'running' }))
    const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
    await advance(10)
    expect(result.current.busy).toBe(true)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(callRequestRpc.mock.calls[0][0]).toBe('request.planProposal')
    expect(callRequestRpc.mock.calls[0][1]).toEqual({ id: 'r1' })
    await advance(PLAN_RUN_STATE_POLL_MS)
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('polls although the Request already has a Plan (a rewrite)', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'running' }))
    const withPlan = { ...planning, planTaskId: 'plan1' }
    const { result } = renderHook(() => usePlanRunState(withPlan as never, vi.fn()))
    await advance(10)
    expect(result.current.busy).toBe(true)
  })

  it.each(['pending', 'running', 'committing'])(
    'is busy for a %s job with no run',
    async (state) => {
      vi.useFakeTimers()
      callRequestRpc.mockResolvedValue(ok({ autoPlan: { enabled: true, state, attempt: 1 } }))
      const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
      await advance(10)
      expect(result.current.busy).toBe(true)
      expect(result.current.view?.autoPlan?.state).toBe(state)
    }
  )

  it.each(['gave_up', 'skipped', 'done', ''])('is not busy for a %j job', async (state) => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'failed', autoPlan: { enabled: true, state } }))
    const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
    await advance(10)
    expect(result.current.busy).toBe(false)
  })

  it('says it finished exactly once, when the work stops', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'running' }))
    const onFinished = vi.fn()
    const { result } = renderHook(() => usePlanRunState(planning as never, onFinished))
    await advance(10)
    callRequestRpc.mockResolvedValue(ok({ status: 'succeeded' }))
    await advance(PLAN_RUN_STATE_POLL_MS + 10)
    expect(result.current.busy).toBe(false)
    expect(onFinished).toHaveBeenCalledTimes(1)
    await advance(PLAN_RUN_STATE_POLL_MS * 3)
    expect(onFinished).toHaveBeenCalledTimes(1)
  })

  it('does not call it finished between two attempts of the same job', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(
      ok({ status: 'running', autoPlan: { enabled: true, state: 'running' } })
    )
    const onFinished = vi.fn()
    renderHook(() => usePlanRunState(planning as never, onFinished))
    await advance(10)
    callRequestRpc.mockResolvedValue(
      ok({ status: 'failed', autoPlan: { enabled: true, state: 'pending', attempt: 2 } })
    )
    await advance(PLAN_RUN_STATE_POLL_MS + 10)
    expect(onFinished).not.toHaveBeenCalled()
  })

  it('treats "run not found" as no run: not busy, silent', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'not_found' } })
    const onFinished = vi.fn()
    const { result } = renderHook(() => usePlanRunState(planning as never, onFinished))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(result.current.busy).toBe(false)
    expect(result.current.view).toBeNull()
    expect(onFinished).not.toHaveBeenCalled()
  })

  it('does not ask at all while the Request is not planning, and stops when it leaves planning', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'running' }))
    const { rerender, result } = renderHook(
      ({ request }) => usePlanRunState(request as never, vi.fn()),
      {
        initialProps: { request: { id: 'r1', status: 'executing' } as Req }
      }
    )
    await advance(PLAN_RUN_STATE_POLL_MS * 2)
    expect(callRequestRpc).not.toHaveBeenCalled()

    rerender({ request: planning })
    await advance(10)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(result.current.busy).toBe(true)

    rerender({ request: { id: 'r1', status: 'awaiting_plan_approval' } })
    expect(result.current.busy).toBe(false)
    expect(result.current.view).toBeNull()
    await advance(PLAN_RUN_STATE_POLL_MS * 3)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('stops polling when unmounted', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'running' }))
    const { unmount } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
    await advance(10)
    unmount()
    await advance(PLAN_RUN_STATE_POLL_MS * 3)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('refresh reads at once, and once more after a read already in flight', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({}))
    const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
    await advance(10)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    callRequestRpc.mockResolvedValue(ok({ status: 'running' }))
    act(() => result.current.refresh())
    await advance(10)
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    expect(result.current.busy).toBe(true)

    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementationOnce(() => new Promise((r) => (release = r)))
    act(() => result.current.refresh())
    act(() => result.current.refresh())
    expect(callRequestRpc).toHaveBeenCalledTimes(3)
    await act(async () => {
      release(ok({ status: 'succeeded' }))
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(callRequestRpc).toHaveBeenCalledTimes(4)
  })

  describe('start grace period', () => {
    const idle = ok({ autoPlan: { enabled: true } })

    it('elapses only after the first sighting of "on, nothing started"', async () => {
      vi.useFakeTimers()
      callRequestRpc.mockResolvedValue(idle)
      const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
      await advance(10)
      expect(result.current.graceElapsed).toBe(false)
      await advance(PLAN_AUTO_START_GRACE_MS - 1000)
      expect(result.current.graceElapsed).toBe(false)
      await advance(1500)
      expect(result.current.graceElapsed).toBe(true)
    })

    it('does not count time from later polls, only from the first sighting', async () => {
      vi.useFakeTimers()
      callRequestRpc.mockResolvedValue(idle)
      const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
      await advance(PLAN_AUTO_START_GRACE_MS - 500)
      expect(result.current.graceElapsed).toBe(false)
      await advance(1000)
      expect(result.current.graceElapsed).toBe(true)
    })

    it('starts over when work shows up, and never counts when automatic mode is off', async () => {
      vi.useFakeTimers()
      callRequestRpc.mockResolvedValue(idle)
      const { result } = renderHook(() => usePlanRunState(planning as never, vi.fn()))
      await advance(10 + PLAN_RUN_STATE_POLL_MS)
      callRequestRpc.mockResolvedValue(ok({ autoPlan: { enabled: true, state: 'running' } }))
      await advance(PLAN_RUN_STATE_POLL_MS)
      await advance(PLAN_AUTO_START_GRACE_MS)
      expect(result.current.graceElapsed).toBe(false)

      callRequestRpc.mockResolvedValue(ok({ autoPlan: { enabled: false } }))
      await advance(PLAN_RUN_STATE_POLL_MS + PLAN_AUTO_START_GRACE_MS)
      expect(result.current.graceElapsed).toBe(false)
    })
  })
})
