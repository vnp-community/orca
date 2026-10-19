// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))
import { emitRequestEvent } from '../lib/request-event-bus'

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)
const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string, message = 'm') => ({ ok: false, error: { kind, code: 'x', message } })
import { useClarifications } from './useClarifications'

const clar = {
  id: 'c1',
  request_id: 'r1',
  status: 'open',
  version: 2,
  questions: [{ id: 'q1', seq: 1, kind: 'text', prompt: 'Why?' }]
}

describe('useClarifications', () => {
  it('loads the open clarification and keeps drafts in state only', async () => {
    callRequestRpc.mockResolvedValue(
      ok({ clarifications: [clar, { id: 'c0', status: 'answered' }] })
    )
    const { result } = renderHook(() => useClarifications('r1'))
    await waitFor(() => expect(result.current.open?.id).toBe('c1'))
    expect(result.current.history.map((c) => c.id)).toEqual(['c0'])
    act(() => result.current.setDraftValue('q1', 'because'))
    expect(result.current.hasDraft).toBe(true)
    expect(window.localStorage.length).toBe(0)
    act(() => result.current.clearDraft())
    expect(result.current.hasDraft).toBe(false)
  })

  it('answer posts the payload, returns stillMissing and ignores a double submit', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ clarifications: [clar] }))
    const { result } = renderHook(() => useClarifications('r1'))
    await waitFor(() => expect(result.current.open).not.toBeNull())
    let release: (v: unknown) => void = () => {}
    callRequestRpc.mockImplementationOnce(
      () =>
        new Promise((r) => {
          release = r
        })
    )
    callRequestRpc.mockResolvedValue(ok({ clarifications: [clar] }))
    const payload = { clarificationId: 'c1', answers: [], complete: true, expectedVersion: 2 }
    let first!: Promise<unknown>
    let second: unknown
    await act(async () => {
      first = result.current.answer(payload)
      second = await result.current.answer(payload)
    })
    expect(second).toMatchObject({ ok: false })
    await act(async () => {
      release(ok({ still_missing: true, request_status: 'awaiting_information' }))
      await first
    })
    expect(await first).toEqual({
      ok: true,
      value: { stillMissing: true, requestStatus: 'awaiting_information' }
    })
    expect(callRequestRpc).toHaveBeenCalledWith('clarification.answer', payload)
  })

  it('refetches on clarification events for its request only and ', async () => {
    callRequestRpc.mockResolvedValue(ok({ clarifications: [] }))
    renderHook(() => useClarifications('r1'))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(1))
    act(() =>
      emitRequestEvent({ requestId: 'zz', eventType: 'clarification.requested', occurredAt: '' })
    )
    act(() =>
      emitRequestEvent({ requestId: 'r1', eventType: 'clarification.requested', occurredAt: '' })
    )
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
  })

  it('surfaces an unsupported list as a load failure instead of "no clarifications"', async () => {
    callRequestRpc.mockResolvedValue(fail('unsupported', 'method_not_found'))
    const u = renderHook(() => useClarifications('r9'))
    await waitFor(() => expect(u.result.current.loading).toBe(false))
    expect(u.result.current.error).toBe('unsupported')
    expect(u.result.current.loadFailure).toEqual({ kind: 'unsupported', code: 'x' })
    expect(u.result.current.open).toBeNull()
  })

  it('quotes the backend code from the message and clears the failure on a successful retry', async () => {
    callRequestRpc.mockResolvedValueOnce(fail('forbidden', 'REQUEST_FORBIDDEN: no grant'))
    const { result } = renderHook(() => useClarifications('r1'))
    await waitFor(() =>
      expect(result.current.loadFailure).toEqual({ kind: 'forbidden', code: 'REQUEST_FORBIDDEN' })
    )
    callRequestRpc.mockResolvedValue(ok({ clarifications: [clar] }))
    act(() => result.current.refetch())
    await waitFor(() => expect(result.current.open?.id).toBe('c1'))
    expect(result.current.loadFailure).toBeNull()
    expect(result.current.error).toBeNull()
  })

  it('keeps the open question when a later refetch fails so typed answers survive', async () => {
    callRequestRpc.mockResolvedValueOnce(ok({ clarifications: [clar] }))
    const { result } = renderHook(() => useClarifications('r1'))
    await waitFor(() => expect(result.current.open?.id).toBe('c1'))
    act(() => result.current.setDraftValue('q1', 'half typed'))
    callRequestRpc.mockResolvedValue(fail('network', 'offline'))
    act(() => result.current.refetch())
    await waitFor(() => expect(result.current.loadFailure?.kind).toBe('network'))
    expect(result.current.open?.id).toBe('c1')
    expect(result.current.draft.get('q1')).toBe('half typed')
  })

  it('does not call the backend when the request is not waiting on a person', () => {
    const { result } = renderHook(() => useClarifications(null))
    expect(callRequestRpc).not.toHaveBeenCalled()
    expect(result.current.loadFailure).toBeNull()
  })
})
