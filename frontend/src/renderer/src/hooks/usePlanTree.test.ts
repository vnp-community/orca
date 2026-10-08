// @vitest-environment happy-dom
import { renderHook, waitFor, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRuntimeRpc = vi.fn()
vi.mock('../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (...a: unknown[]) => callRuntimeRpc(...a),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { usePlanTree, PLAN_TREE_MAX_PAGES, PLAN_TREE_POLL_MS } from './usePlanTree'
import { emitRequestEvent } from '../lib/request-event-bus'
import type { OrcaRequest } from '../../../shared/request-types'

const baseReq = {
  id: 'r1',
  projectId: 'p1',
  planTaskId: 'plan1',
  status: 'planning'
} as OrcaRequest

function t(id: string, type: string, parentId?: string) {
  return { id, type, parentId, title: id, status: 'todo', projectId: 'p1' }
}

beforeEach(() => {
  callRuntimeRpc.mockReset()
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: { approvals: [] } })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('usePlanTree', () => {
  it('concatenates pages and builds the subtree', async () => {
    callRuntimeRpc
      .mockResolvedValueOnce({
        tasks: [t('plan1', 'plan'), t('ph1', 'phase', 'plan1')],
        nextPageToken: 'n1'
      })
      .mockResolvedValueOnce({ tasks: [t('w1', 'task', 'ph1')], nextPageToken: '' })
    const { result } = renderHook(() => usePlanTree(baseReq))
    await waitFor(() => expect(result.current.tree?.plan?.id).toBe('plan1'))
    expect(result.current.tree?.tasksByPhase['ph1'].map((x) => x.id)).toEqual(['w1'])
    expect(callRuntimeRpc.mock.calls[1][2]).toMatchObject({
      projectId: 'p1',
      pageToken: 'n1',
      pageSize: 200
    })
    expect(result.current.truncated).toBe(false)
  })

  it('flags truncated after the max page count', async () => {
    callRuntimeRpc.mockResolvedValue({ tasks: [t('plan1', 'plan')], nextPageToken: 'more' })
    const { result } = renderHook(() => usePlanTree(baseReq))
    await waitFor(() => expect(result.current.truncated).toBe(true))
    expect(callRuntimeRpc).toHaveBeenCalledTimes(PLAN_TREE_MAX_PAGES)
  })

  it('does not call task.list without planTaskId', async () => {
    const { result } = renderHook(() => usePlanTree({ ...baseReq, planTaskId: undefined }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(callRuntimeRpc).not.toHaveBeenCalled()
    expect(result.current.tree).toBeNull()
  })

  it('maps a forbidden task.list error to its kind', async () => {
    callRuntimeRpc.mockRejectedValue({ code: 'forbidden', message: 'REQUEST_FORBIDDEN: no' })
    const { result } = renderHook(() => usePlanTree(baseReq))
    await waitFor(() => expect(result.current.error).toBe('forbidden'))
  })

  it('refetches on a plan.generated bus event for this request only', async () => {
    callRuntimeRpc.mockResolvedValue({ tasks: [t('plan1', 'plan')] })
    renderHook(() => usePlanTree(baseReq))
    await waitFor(() => expect(callRuntimeRpc).toHaveBeenCalledTimes(1))
    act(() =>
      emitRequestEvent({
        requestId: 'other',
        eventType: 'orca.request.plan.generated',
        occurredAt: ''
      })
    )
    act(() =>
      emitRequestEvent({
        requestId: 'r1',
        eventType: 'orca.request.plan.generated',
        occurredAt: ''
      })
    )
    await waitFor(() => expect(callRuntimeRpc).toHaveBeenCalledTimes(2))
  })

  it('polls while executing, skips when hidden, stops on unmount', async () => {
    vi.useFakeTimers()
    callRuntimeRpc.mockResolvedValue({ tasks: [t('plan1', 'plan')] })
    const { unmount } = renderHook(() => usePlanTree({ ...baseReq, status: 'executing' }))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0)
    })
    const initial = callRuntimeRpc.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PLAN_TREE_POLL_MS)
    })
    expect(callRuntimeRpc.mock.calls.length).toBe(initial + 1)

    const vis = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PLAN_TREE_POLL_MS)
    })
    expect(callRuntimeRpc.mock.calls.length).toBe(initial + 1)
    vis.mockRestore()

    unmount()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PLAN_TREE_POLL_MS * 2)
    })
    expect(callRuntimeRpc.mock.calls.length).toBe(initial + 1)
  })
})
