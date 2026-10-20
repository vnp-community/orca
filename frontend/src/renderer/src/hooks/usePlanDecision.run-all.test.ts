// @vitest-environment happy-dom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { usePlanDecision } from './usePlanDecision'
import type { Approval } from '../../../shared/request-types'

const approval = {
  id: 'ap1',
  requestId: 'r1',
  subjectType: 'plan',
  subjectId: 'plan1',
  subjectDigest: 'dg',
  status: 'pending',
  version: 2,
  createdAt: '2026-10-01',
  updatedAt: '2026-10-01'
} as Approval

const settled = vi.fn()

beforeEach(() => {
  callRequestRpc.mockReset()
  settled.mockReset()
})
afterEach(cleanup)

describe('usePlanDecision.approve with a run-all option', () => {
  it('keeps the exact arguments of a plain approval', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: {} })
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    let outcome: Awaited<ReturnType<typeof result.current.approve>> | undefined
    await act(async () => {
      outcome = await result.current.approve(approval)
    })
    expect(callRequestRpc).toHaveBeenCalledWith('approval.approve', {
      id: 'ap1',
      approvalId: 'ap1',
      expectedVersion: 2,
      expectedDigest: 'dg',
      comment: undefined
    })
    expect(outcome).toEqual({ ok: true })
  })

  it('sends executionMode only when asked, next to the risk fields', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { executionMode: 'continuous' } })
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    await act(async () => {
      await result.current.approve(
        approval,
        undefined,
        { viewedImpactDigest: 'imp', acceptedFindingIds: ['f1'] },
        { executionMode: 'continuous' }
      )
    })
    expect(callRequestRpc.mock.calls[0][1]).toMatchObject({
      id: 'ap1',
      viewedImpactDigest: 'imp',
      acceptedFindingIds: ['f1'],
      executionMode: 'continuous'
    })
  })

  it('reports runAllApplied true when the server pinned continuous', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { executionMode: 'continuous' } })
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    let outcome: Awaited<ReturnType<typeof result.current.approve>> | undefined
    await act(async () => {
      outcome = await result.current.approve(approval, undefined, undefined, {
        executionMode: 'continuous'
      })
    })
    expect(outcome).toEqual({ ok: true, runAllApplied: true })
    expect(settled).toHaveBeenCalledTimes(1)
  })

  it.each([{}, { executionMode: 'step_by_step' }, null, 'junk'])(
    'reports runAllApplied false for a gateway that answered %j',
    async (value) => {
      callRequestRpc.mockResolvedValue({ ok: true, value })
      const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
      let outcome: Awaited<ReturnType<typeof result.current.approve>> | undefined
      await act(async () => {
        outcome = await result.current.approve(approval, undefined, undefined, {
          executionMode: 'continuous'
        })
      })
      expect(outcome).toEqual({ ok: true, runAllApplied: false })
    }
  )

  it('leaves runAllApplied out when run-all was not asked for, even if the reply carries a mode', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { executionMode: 'continuous' } })
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    let outcome: Awaited<ReturnType<typeof result.current.approve>> | undefined
    await act(async () => {
      outcome = await result.current.approve(approval)
    })
    expect(outcome).toEqual({ ok: true })
  })

  it('blocks a second call while the first is in flight, whichever button sent it', async () => {
    let release: (v: unknown) => void = () => {}
    callRequestRpc.mockReturnValue(new Promise((resolve) => (release = resolve)))
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    let first: Promise<unknown> | undefined
    let second: Awaited<ReturnType<typeof result.current.approve>> | undefined
    await act(async () => {
      first = result.current.approve(approval, undefined, undefined, {
        executionMode: 'continuous'
      })
      second = await result.current.approve(approval)
    })
    expect(second?.ok).toBe(false)
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    await act(async () => {
      release({ ok: true, value: { executionMode: 'continuous' } })
      await first
    })
  })

  it('exposes ALREADY_PINNED as the failure and warns about programming errors with the code only', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    callRequestRpc.mockResolvedValueOnce({
      ok: false,
      error: {
        kind: 'unknown',
        code: 'runtime_error',
        message: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED: pinned',
        backendCode: 'REQUEST_EXECUTION_MODE_ALREADY_PINNED'
      }
    })
    const { result } = renderHook(() => usePlanDecision({ id: 'r1' }, settled))
    await act(async () => {
      await result.current.approve(approval, undefined, undefined, { executionMode: 'continuous' })
    })
    expect(result.current.failure?.error.backendCode).toBe('REQUEST_EXECUTION_MODE_ALREADY_PINNED')
    expect(warn).not.toHaveBeenCalled()

    callRequestRpc.mockResolvedValueOnce({
      ok: false,
      error: {
        kind: 'unknown',
        code: 'runtime_error',
        message: 'REQUEST_EXECUTION_MODE_INVALID: secret request text',
        backendCode: 'REQUEST_EXECUTION_MODE_INVALID'
      }
    })
    await act(async () => {
      await result.current.approve(approval, undefined, undefined, { executionMode: 'continuous' })
    })
    expect(warn).toHaveBeenCalledTimes(1)
    expect(JSON.stringify(warn.mock.calls)).not.toContain('secret request text')
    warn.mockRestore()
  })
})
