// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { usePlanProposalOfRun } from './usePlanProposalOfRun'

const ok = (value: unknown) => ({ ok: true as const, value })

type Props = {
  id: string
  runId?: string
  revision?: number
  refreshKey?: string
  enabled?: boolean
}
function setup(initial: Props) {
  return renderHook(
    (p: Props) =>
      usePlanProposalOfRun(
        p.id,
        { runId: p.runId, revision: p.revision, refreshKey: p.refreshKey },
        p.enabled
      ),
    { initialProps: initial }
  )
}

beforeEach(() => callRequestRpc.mockReset())
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usePlanProposalOfRun', () => {
  it('reads the proposal of the run and the revision once', async () => {
    callRequestRpc.mockResolvedValue(
      ok({ runId: 'run-2', status: 'succeeded', proposal: { title: 'T' } })
    )
    const { result, rerender } = setup({ id: 'r1', runId: 'run-2', revision: 2 })
    expect(result.current.isLoading).toBe(true)
    await waitFor(() => expect(result.current.view?.proposal?.title).toBe('T'))
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', {
      id: 'r1',
      runId: 'run-2',
      revision: 2
    })
    rerender({ id: 'r1', runId: 'run-2', revision: 2 })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(result.current.isLoading).toBe(false)
    expect(result.current.running).toBe(false)
  })

  it('asks for the latest run when it has neither run nor revision (an older gateway)', async () => {
    callRequestRpc.mockResolvedValue(ok({ status: 'succeeded', proposal: { title: 'T' } }))
    setup({ id: 'r1' })
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', { id: 'r1' })
  })

  it('reports a run that is still writing as running without polling', async () => {
    vi.useFakeTimers()
    callRequestRpc.mockResolvedValue(ok({ status: 'running', proposal: null }))
    const { result } = setup({ id: 'r1', runId: 'run-3' })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(result.current.running).toBe(true)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('reports a failed run with its code', async () => {
    callRequestRpc.mockResolvedValue(
      ok({ status: 'failed', errorCode: 'REQUEST_PLAN_REVISION_NO_CHANGE' })
    )
    const { result } = setup({ id: 'r1', runId: 'run-4' })
    await waitFor(() => expect(result.current.view?.status).toBe('failed'))
    expect(result.current.view?.errorCode).toBe('REQUEST_PLAN_REVISION_NO_CHANGE')
  })

  it('reports a failed read and refetch asks again', async () => {
    callRequestRpc.mockResolvedValueOnce({
      ok: false,
      error: { kind: 'network', code: 'NETWORK_ERROR', message: 'offline' }
    })
    const { result } = setup({ id: 'r1', runId: 'run-5' })
    await waitFor(() => expect(result.current.error).toBe('network'))
    expect(result.current.view).toBeNull()
    callRequestRpc.mockResolvedValueOnce(ok({ status: 'succeeded', proposal: { title: 'T' } }))
    result.current.refetch()
    await waitFor(() => expect(result.current.view?.proposal?.title).toBe('T'))
    expect(result.current.error).toBeNull()
  })

  it('keeps the last answer of the same version across a refresh but never shows another version', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ status: 'succeeded', proposal: { title: 'one' } }))
    const { result, rerender } = setup({ id: 'r1', runId: 'a', refreshKey: 'k1' })
    await waitFor(() => expect(result.current.view?.proposal?.title).toBe('one'))
    callRequestRpc.mockImplementationOnce(() => new Promise(() => undefined))
    rerender({ id: 'r1', runId: 'a', refreshKey: 'k2' })
    expect(result.current.view?.proposal?.title).toBe('one')
    expect(result.current.isLoading).toBe(true)
    callRequestRpc.mockImplementationOnce(() => new Promise(() => undefined))
    rerender({ id: 'r1', runId: 'b', refreshKey: 'k2' })
    expect(result.current.view).toBeNull()
  })

  it('does not call while disabled', async () => {
    setup({ id: 'r1', runId: 'a', enabled: false })
    await Promise.resolve()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })
})
