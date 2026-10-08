// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { emitRequestEvent } from '../lib/request-event-bus'
import { useBacklog } from './useBacklog'

const reqPage = (ids: string[], nextPageToken = '') => ({
  ok: true, value: { requestRows: ids.map((requestId) => ({ requestId, title: requestId })), nextPageToken }
})
const groupPage = (reqIds: string[], nextPageToken = '') => ({
  ok: true, value: { groups: reqIds.map((requestId) => ({ requestId, planTaskId: `pl-${requestId}`, tasks: [{ taskId: `t-${requestId}` }] })), nextPageToken }
})
const err = (message: string, kind = 'unknown') => ({ ok: false, error: { kind, code: 'internal', message } })

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestFlowSupport: 'supported' })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useBacklog', () => {
  it('uses the per-view channel and page size, with no tenant parameters', async () => {
    callRequestRpc.mockImplementation(async (m: string) => (m === 'backlog.requests' ? reqPage(['r1']) : groupPage(['r1'])))
    const a = renderHook(() => useBacklog('requests', { projectId: 'p' }))
    const b = renderHook(() => useBacklog('tasks', { planTaskId: 'pl' }))
    const c = renderHook(() => useBacklog('execute', { phaseTaskId: 'ph', planTaskId: 'ignored' }))
    await waitFor(() => expect(c.result.current.loadedOnce).toBe(true))
    await waitFor(() => expect(a.result.current.loadedOnce && b.result.current.loadedOnce).toBe(true))
    expect(callRequestRpc).toHaveBeenCalledWith('backlog.requests', { pageSize: 50, projectId: 'p' })
    expect(callRequestRpc).toHaveBeenCalledWith('backlog.tasks', { pageSize: 20, planTaskId: 'pl' })
    expect(callRequestRpc).toHaveBeenCalledWith('backlog.execute', { pageSize: 20, phaseTaskId: 'ph' })
  })

  it('does not call the server when inactive', async () => {
    renderHook(() => useBacklog('tasks', {}, { active: false }))
    await act(async () => {})
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('loadMore sends the token and removes duplicates', async () => {
    callRequestRpc.mockResolvedValueOnce(reqPage(['a', 'b'], 'tok')).mockResolvedValueOnce(reqPage(['b', 'c']))
    const { result } = renderHook(() => useBacklog('requests'))
    await waitFor(() => expect(result.current.items).toHaveLength(2))
    expect(result.current.countLabel()).toEqual({ count: 2, plus: true })
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.items.map((i) => i.requestId)).toEqual(['a', 'b', 'c']))
    expect(callRequestRpc.mock.calls[1][1]).toMatchObject({ pageToken: 'tok' })
    expect(result.current.hasMore).toBe(false)
  })

  it('follows empty pages that still have a token, at most 3 extra times', async () => {
    callRequestRpc.mockResolvedValue(groupPage([], 'more'))
    const { result } = renderHook(() => useBacklog('tasks'))
    await waitFor(() => expect(result.current.loadedOnce).toBe(true))
    expect(callRequestRpc).toHaveBeenCalledTimes(4)
    callRequestRpc.mockReset()
    callRequestRpc.mockResolvedValueOnce(groupPage([], 'more')).mockResolvedValueOnce(groupPage(['r9']))
    const second = renderHook(() => useBacklog('execute'))
    await waitFor(() => expect(second.result.current.items).toHaveLength(1))
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('isolates errors per view instance', async () => {
    callRequestRpc.mockImplementation(async (m: string) =>
      m === 'backlog.tasks' ? err('REQUEST_BACKLOG_TASK_SERVICE_UNAVAILABLE: down', 'unavailable') : reqPage(['r1']))
    const req = renderHook(() => useBacklog('requests'))
    const task = renderHook(() => useBacklog('tasks'))
    await waitFor(() => expect(task.result.current.error?.kind).toBe('network'))
    await waitFor(() => expect(req.result.current.items).toHaveLength(1))
    expect(req.result.current.error).toBeNull()
  })

  it('maps unsupported to the store flag and keeps old items when a reload fails', async () => {
    callRequestRpc.mockResolvedValueOnce(reqPage(['a']))
    const { result } = renderHook(() => useBacklog('requests'))
    await waitFor(() => expect(result.current.items).toHaveLength(1))
    callRequestRpc.mockResolvedValueOnce(err('x', 'network'))
    act(() => result.current.refetch())
    await waitFor(() => expect(result.current.error?.kind).toBe('network'))
    expect(result.current.items).toHaveLength(1)
    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'unsupported', code: 'method_not_found', message: 'x' } })
    act(() => result.current.refetch())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('unsupported'))
  })

  it('restarts from page one once after a bad page token', async () => {
    callRequestRpc
      .mockResolvedValueOnce(reqPage(['a'], 'stale'))
      .mockResolvedValueOnce(err('REQUEST_BACKLOG_BAD_PAGE_TOKEN: bad'))
      .mockResolvedValue(reqPage(['a', 'z']))
    const { result } = renderHook(() => useBacklog('requests'))
    await waitFor(() => expect(result.current.hasMore).toBe(true))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.items).toHaveLength(2))
    expect(callRequestRpc.mock.calls[2][1]).not.toHaveProperty('pageToken')
  })

  it('refetches on listed events after a debounce, ignores others', async () => {
    callRequestRpc.mockResolvedValue(reqPage(['a']))
    const { result } = renderHook(() => useBacklog('requests'))
    await waitFor(() => expect(result.current.loadedOnce).toBe(true))
    vi.useFakeTimers()
    act(() => emitRequestEvent({ requestId: 'r', eventType: 'orca.request.solution.proposed', occurredAt: '' }))
    act(() => { vi.advanceTimersByTime(600) })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    act(() => emitRequestEvent({ requestId: 'r', eventType: 'orca.request.phase.completed', occurredAt: '' }))
    act(() => { vi.advanceTimersByTime(600) })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('polls while mounted and clears the timer on unmount', async () => {
    callRequestRpc.mockResolvedValue(reqPage(['a']))
    vi.useFakeTimers()
    const { unmount } = renderHook(() => useBacklog('requests'))
    await act(async () => { vi.advanceTimersByTime(30_000) })
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
