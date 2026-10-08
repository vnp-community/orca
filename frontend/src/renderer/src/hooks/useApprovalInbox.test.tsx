// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { emitRequestEvent } from '../lib/request-event-bus'
import { useApprovalInbox } from './useApprovalInbox'

const wire = (id: string, over: Record<string, unknown> = {}) => ({
  id, requestId: `req-${id}`, subjectType: 'phase', subjectId: 's', subjectDigest: `dg-${id}`,
  status: 'pending', version: 2, createdAt: '2026-10-01T00:00:00Z', ...over
})
const opts = { subjectGroup: 'all' as const, overdueOnly: false, active: true }
const page = (ids: string[], nextPageToken = '') => ({
  ok: true, value: { approvals: ids.map((i) => wire(i)), nextPageToken }
})

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestsById: {}, requestFlowSupport: 'supported', pendingApprovalCount: 0 })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useApprovalInbox', () => {
  it('loads the first page, then merges the next without duplicate ids', async () => {
    callRequestRpc.mockResolvedValueOnce(page(['a', 'b'], 'tok')).mockResolvedValueOnce(page(['b', 'c']))
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(2))
    expect(callRequestRpc.mock.calls[0]).toEqual(['approval.listPending', { pageSize: 50 }])
    expect(result.current.hasMore).toBe(true)
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.rows.map((r) => r.id)).toEqual(['a', 'b', 'c']))
    expect(callRequestRpc.mock.calls[1][1]).toMatchObject({ pageToken: 'tok' })
    expect(result.current.hasMore).toBe(false)
  })

  it('sends the subject type filter to the server only for single-type groups and updates the count unfiltered', async () => {
    callRequestRpc.mockResolvedValue(page(['a']))
    const { result } = renderHook(() => useApprovalInbox({ ...opts, subjectGroup: 'phase' }))
    await waitFor(() => expect(result.current.rows).toHaveLength(1))
    expect(callRequestRpc.mock.calls[0][1]).toMatchObject({ subjectType: 'phase' })
    expect(useAppStore.getState().pendingApprovalCount).toBe(0)
    renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(useAppStore.getState().pendingApprovalCount).toBe(1))
  })

  it('approve sends id, expectedVersion and expectedDigest only, and drops the row', async () => {
    callRequestRpc.mockResolvedValueOnce(page(['a'])).mockResolvedValueOnce({ ok: true, value: {} })
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(1))
    let res
    await act(async () => { res = await result.current.approve(result.current.rows[0]) })
    expect(res).toEqual({ outcome: 'ok' })
    expect(callRequestRpc.mock.calls[1]).toEqual([
      'approval.approve', { id: 'a', expectedVersion: 2, expectedDigest: 'dg-a' }
    ])
    expect(result.current.rows).toHaveLength(0)
  })

  it('never decides without a digest, and rejects short comments without an RPC', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { approvals: [wire('a', { subjectDigest: undefined })] } })
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(1))
    const row = result.current.rows[0]
    expect((await result.current.approve(row)).outcome).toBe('validation')
    expect((await result.current.reject({ ...row, subjectDigest: 'x' }, '123456789')).outcome).toBe('validation')
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('maps closed to row removal, changed to a refetch, unsupported to the flag', async () => {
    const err = (message: string, kind = 'conflict') => ({ ok: false, error: { kind, code: 'internal', message } })
    callRequestRpc
      .mockResolvedValueOnce(page(['a', 'b']))
      .mockResolvedValueOnce(err('REQUEST_APPROVAL_ALREADY_DECIDED: x', 'invalid_state'))
      .mockResolvedValueOnce(err('REQUEST_APPROVAL_DIGEST_MISMATCH: x'))
      .mockResolvedValue(page(['b']))
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(2))
    const [a, b] = result.current.rows
    await act(async () => { expect((await result.current.approve(a)).outcome).toBe('closed') })
    expect(result.current.rows.map((r) => r.id)).toEqual(['b'])
    const before = callRequestRpc.mock.calls.length
    await act(async () => { expect((await result.current.approve(b)).outcome).toBe('changed') })
    await waitFor(() => expect(callRequestRpc.mock.calls.length).toBeGreaterThan(before + 1))
    expect(callRequestRpc.mock.calls.at(-1)![0]).toBe('approval.listPending')

    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'unsupported', code: 'method_not_found', message: 'x' } })
    await act(async () => { expect((await result.current.approve(b)).outcome).toBe('unsupported') })
    expect(useAppStore.getState().requestFlowSupport).toBe('unsupported')
  })

  it('refetches after an approval event (debounced) and ignores other events', async () => {
    callRequestRpc.mockResolvedValue(page(['a']))
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(1))
    vi.useFakeTimers()
    act(() => emitRequestEvent({ requestId: 'r', eventType: 'orca.request.request.created', occurredAt: '' }))
    act(() => { vi.advanceTimersByTime(600) })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    act(() => emitRequestEvent({ requestId: 'r', eventType: 'approval.decided', occurredAt: '' }))
    act(() => emitRequestEvent({ requestId: 'r', eventType: 'approval.requested', occurredAt: '' }))
    act(() => { vi.advanceTimersByTime(600) })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('polls every 30 s while visible, and stops on unmount', async () => {
    callRequestRpc.mockResolvedValue(page(['a']))
    vi.useFakeTimers()
    const { unmount } = renderHook(() => useApprovalInbox(opts))
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    await act(async () => { vi.advanceTimersByTime(30_000) })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not call the server when inactive and filters overdue rows on the client', async () => {
    const { result, rerender } = renderHook((o) => useApprovalInbox(o), { initialProps: { ...opts, active: false } as Parameters<typeof useApprovalInbox>[0] })
    expect(callRequestRpc).not.toHaveBeenCalled()
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { approvals: [wire('late', { dueAt: '2026-10-01T00:00:00Z' }), wire('fine', { dueAt: '2027-01-01T00:00:00Z' })] }
    })
    const now = Date.parse('2026-10-07T00:00:00Z')
    rerender({ ...opts, active: true, overdueOnly: true, now })
    await waitFor(() => expect(result.current.rows.map((r) => r.id)).toEqual(['late']))
  })

  it('keeps old rows and exposes the error on a failed reload', async () => {
    callRequestRpc.mockResolvedValueOnce(page(['a']))
    const { result } = renderHook(() => useApprovalInbox(opts))
    await waitFor(() => expect(result.current.rows).toHaveLength(1))
    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'network', code: 'NETWORK_ERROR', message: 'x' } })
    act(() => result.current.refetch())
    await waitFor(() => expect(result.current.error?.kind).toBe('network'))
    expect(result.current.rows).toHaveLength(1)
  })
})
