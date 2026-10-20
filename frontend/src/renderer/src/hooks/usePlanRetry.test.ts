// @vitest-environment happy-dom
import { renderHook, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { usePlanRetry } from './usePlanRetry'
import type { PlanRunView } from '../../../shared/request-plan-auto'

const run = (enabled: boolean | null): PlanRunView => ({
  runId: 'run1',
  status: 'failed',
  errorCode: 'REQUEST_PLAN_AI_INVALID_JSON',
  errorMessage: '',
  engine: 'native',
  autoPlan:
    enabled === null
      ? null
      : { enabled, state: 'gave_up', origin: 'event', attempt: 2, maxAttempts: 2, reasonCode: 'X' }
})

function setup(view: PlanRunView | null, manual = vi.fn().mockResolvedValue({ ok: true })) {
  const refresh = vi.fn()
  const hook = renderHook(() =>
    usePlanRetry({ request: { id: 'r1' }, run: view, regenerateManually: manual, refresh })
  )
  return { ...hook, manual, refresh }
}

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestFlowFeatures: new Set(['request.planAuto']) })
})
afterEach(cleanup)

describe('usePlanRetry', () => {
  it('restarts through the server when the automatic mode is on, then reads the state again', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { autoPlan: { state: 'pending' } } })
    const { result, manual, refresh } = setup(run(true))
    await act(() => result.current.retry())
    expect(callRequestRpc).toHaveBeenCalledWith(
      'request.retryPlan',
      { id: 'r1' },
      { idempotent: true }
    )
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(manual).not.toHaveBeenCalled()
    expect(result.current.error).toBeNull()
    expect(result.current.retrying).toBe(false)
  })

  it('falls back to the manual flow when the server says the automatic mode is off', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'feature_off',
        code: 'runtime_error',
        backendCode: 'REQUEST_PLAN_AUTO_DISABLED',
        message: 'REQUEST_PLAN_AUTO_DISABLED: off'
      }
    })
    const { result, manual, refresh } = setup(run(true))
    await act(() => result.current.retry())
    expect(manual).toHaveBeenCalledTimes(1)
    expect(refresh).not.toHaveBeenCalled()
    expect(result.current.error).toBeNull()
  })

  it('recognises the code from the message when backendCode is absent', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: { kind: 'unknown', code: 'runtime_error', message: 'REQUEST_PLAN_AUTO_DISABLED: off' }
    })
    const { result, manual } = setup(run(true))
    await act(() => result.current.retry())
    expect(manual).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['the project switch is off', run(false)],
    ['the gateway sent no block', run(null)],
    ['there is no run', null]
  ])('goes straight to the manual flow when %s', async (_label, view) => {
    const { result, manual } = setup(view)
    await act(() => result.current.retry())
    expect(manual).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('goes straight to the manual flow on a gateway without the flag', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    const { result, manual } = setup(run(true))
    await act(() => result.current.retry())
    expect(manual).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('shows any other server failure and does not start a second run by hand', async () => {
    callRequestRpc.mockResolvedValue({
      ok: false,
      error: {
        kind: 'invalid_state',
        code: 'runtime_error',
        backendCode: 'REQUEST_STATE_STALE',
        message: 'REQUEST_STATE_STALE: no'
      }
    })
    const { result, manual } = setup(run(true))
    await act(() => result.current.retry())
    expect(manual).not.toHaveBeenCalled()
    expect(result.current.error).toBeTruthy()
  })

  it('shows the failure of the manual flow', async () => {
    const manual = vi.fn().mockResolvedValue({
      ok: false,
      error: { kind: 'unavailable', code: 'runtime_error', message: 'boom' }
    })
    const { result } = setup(run(false), manual)
    await act(() => result.current.retry())
    expect(result.current.error).toBeTruthy()
  })

  it('stays quiet when a manual generate is already running', async () => {
    const manual = vi.fn().mockResolvedValue({
      ok: false,
      error: { kind: 'unknown', code: 'IN_FLIGHT', message: 'In flight' }
    })
    const { result } = setup(run(false), manual)
    await act(() => result.current.retry())
    expect(result.current.error).toBeNull()
  })

  it('clears the previous error when tried again', async () => {
    callRequestRpc.mockResolvedValueOnce({
      ok: false,
      error: { kind: 'unavailable', code: 'runtime_error', message: 'boom' }
    })
    const { result } = setup(run(true))
    await act(() => result.current.retry())
    expect(result.current.error).toBeTruthy()
    callRequestRpc.mockResolvedValueOnce({ ok: true, value: {} })
    await act(() => result.current.retry())
    expect(result.current.error).toBeNull()
  })

  it('makes one call when pressed twice while the first is in flight', async () => {
    let release: (v: unknown) => void = () => undefined
    callRequestRpc.mockImplementation(() => new Promise((r) => (release = r)))
    const { result } = setup(run(true))
    let first: Promise<void> = Promise.resolve()
    act(() => {
      first = result.current.retry()
      void result.current.retry()
    })
    expect(result.current.retrying).toBe(true)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    await act(async () => {
      release({ ok: true, value: {} })
      await first
    })
    expect(result.current.retrying).toBe(false)
  })
})
