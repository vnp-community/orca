// @vitest-environment happy-dom
import { renderHook, act, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
const subscribeRequestEvents = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a),
  subscribeRequestEvents: (...a: unknown[]) => subscribeRequestEvents(...a)
}))
vi.mock('./useRequestFlowSupport', () => ({ useRequestFlowSupport: () => undefined }))

import { useAppStore } from '../store'
import { subscribeRequestBus } from '../lib/request-event-bus'
import {
  useRequestEvents,
  REQUEST_LIST_POLL_MS,
  PENDING_APPROVAL_POLL_MS,
  REQUEST_POLL_EVENT_TYPE
} from './useRequestEvents'

beforeEach(() => {
  vi.useFakeTimers()
  callRequestRpc.mockReset()
  callRequestRpc.mockResolvedValue({ ok: true, value: { approvals: [{}, {}], totalCount: 7 } })
  subscribeRequestEvents.mockReset()
  subscribeRequestEvents.mockReturnValue(vi.fn())
  useAppStore.setState({ requestFlowSupport: 'supported', pendingApprovalCount: 0, activeView: 'requests' })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useRequestEvents', () => {
  it('does nothing while support is not confirmed', () => {
    useAppStore.setState({ requestFlowSupport: 'unknown' })
    renderHook(() => useRequestEvents())
    expect(subscribeRequestEvents).not.toHaveBeenCalled()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })

  it('opens one stream and updates the pending approval count', async () => {
    renderHook(() => useRequestEvents())
    await act(async () => {})
    expect(subscribeRequestEvents).toHaveBeenCalledTimes(1)
    expect(useAppStore.getState().pendingApprovalCount).toBe(7)
  })

  it('forwards stream events to the bus and unsubscribes on unmount', () => {
    const off = vi.fn()
    subscribeRequestEvents.mockReturnValue(off)
    const seen = vi.fn()
    const unsubBus = subscribeRequestBus(seen)
    const { unmount } = renderHook(() => useRequestEvents())
    const { onEvent } = subscribeRequestEvents.mock.calls[0][0]
    onEvent({ requestId: 'r1', eventType: 'request.status_changed', occurredAt: 'now' })
    expect(seen).toHaveBeenCalledTimes(1)
    unmount()
    expect(off).toHaveBeenCalled()
    unsubBus()
  })

  it('falls back to polling on the request page and not when the tab is hidden', () => {
    const seen = vi.fn()
    const unsubBus = subscribeRequestBus(seen)
    renderHook(() => useRequestEvents())
    subscribeRequestEvents.mock.calls[0][0].onFallback()
    vi.advanceTimersByTime(REQUEST_LIST_POLL_MS)
    expect(seen).toHaveBeenCalledWith(expect.objectContaining({ eventType: REQUEST_POLL_EVENT_TYPE }))

    seen.mockClear()
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    vi.advanceTimersByTime(REQUEST_LIST_POLL_MS)
    expect(seen).not.toHaveBeenCalled()
    callRequestRpc.mockClear()
    vi.advanceTimersByTime(PENDING_APPROVAL_POLL_MS)
    expect(callRequestRpc).not.toHaveBeenCalled()
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    unsubBus()
  })
})
