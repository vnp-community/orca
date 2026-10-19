// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useRequestActivity } from './useRequestActivity'
import { emitRequestEvent } from '../lib/request-event-bus'

const entry = (id: string, kind = 'task.failed') => ({
  id,
  kind,
  occurredAt: '2026-10-15T10:00:00Z',
  actorKind: 'agent',
  severity: 'error',
  summary: id,
  details: {}
})

beforeEach(() => {
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({
    ok: true,
    value: { entries: [entry('a'), entry('b')], nextPageToken: 'tok' }
  })
})
afterEach(cleanup)

describe('useRequestActivity', () => {
  it('reads the first page', async () => {
    const { result } = renderHook(() => useRequestActivity('r1'))
    await waitFor(() => expect(result.current.entries).toHaveLength(2))
    expect(callRequestRpc).toHaveBeenCalledWith('request.activity', { id: 'r1', pageSize: 30 })
    expect(result.current.hasMore).toBe(true)
  })

  it('loads the next page by token without repeating a line', async () => {
    const { result } = renderHook(() => useRequestActivity('r1'))
    await waitFor(() => expect(result.current.entries).toHaveLength(2))
    callRequestRpc.mockResolvedValue({ ok: true, value: { entries: [entry('b'), entry('c')] } })
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.entries.map((e) => e.id)).toEqual(['a', 'b', 'c']))
    expect(callRequestRpc).toHaveBeenLastCalledWith('request.activity', {
      id: 'r1',
      pageSize: 30,
      pageToken: 'tok'
    })
    expect(result.current.hasMore).toBe(false)
  })

  it('starts over when the filter changes and sends the filter', async () => {
    const { result, rerender } = renderHook(
      ({ kinds }) =>
        useRequestActivity('r1', {
          filter: { kinds, minSeverity: 'attention', actorKind: 'agent' }
        }),
      {
        initialProps: { kinds: ['task.failed'] }
      }
    )
    await waitFor(() => expect(result.current.entries).toHaveLength(2))
    expect(callRequestRpc).toHaveBeenLastCalledWith('request.activity', {
      id: 'r1',
      pageSize: 30,
      kinds: ['task.failed'],
      minSeverity: 'attention',
      actorKind: 'agent'
    })
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { entries: [entry('z', 'approval.decided')] }
    })
    rerender({ kinds: ['approval.decided'] })
    await waitFor(() => expect(result.current.entries.map((e) => e.id)).toEqual(['z']))
  })

  it('does not send severity info (the default) and reads nothing while disabled', async () => {
    renderHook(() => useRequestActivity('r1', { enabled: false }))
    expect(callRequestRpc).not.toHaveBeenCalled()
    const { result } = renderHook(() =>
      useRequestActivity('r1', { filter: { minSeverity: 'info' } })
    )
    await waitFor(() => expect(result.current.entries).toHaveLength(2))
    expect(callRequestRpc.mock.calls[0][1]).toEqual({ id: 'r1', pageSize: 30 })
  })

  it('refreshes the first page on an event of the request', async () => {
    const { result } = renderHook(() => useRequestActivity('r1'))
    await waitFor(() => expect(result.current.entries).toHaveLength(2))
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { entries: [entry('new'), entry('a'), entry('b')] }
    })
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.request.status_changed',
        occurredAt: ''
      })
    )
    await waitFor(() => expect(result.current.entries.map((e) => e.id)).toEqual(['new', 'a', 'b']))
  })

  it('is unsupported on an older gateway and shows a real failure as an error', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported' } })
    const old = renderHook(() => useRequestActivity('r1'))
    await waitFor(() => expect(old.result.current.unsupported).toBe(true))
    expect(old.result.current.error).toBeNull()
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'network' } })
    const broken = renderHook(() => useRequestActivity('r1'))
    await waitFor(() => expect(broken.result.current.error).toBe('network'))
  })
})
