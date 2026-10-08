// @vitest-environment happy-dom
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { emitRequestEvent } from '../lib/request-event-bus'
import { clearGraphLensCache, useGraphLens } from './useGraphLens'
import { emptyGraphPayload } from '../../../shared/graph-wire-parsers'

const raw = { nodes: [{ id: 'a', label: 'A', risk: 'high' }], edges: [] }
const req = { id: 'r1' }

beforeEach(() => {
  callRequestRpc.mockReset()
  clearGraphLensCache()
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useGraphLens', () => {
  it('calls impact.graph with maxNodes and parses the payload', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: raw })
    const { result } = renderHook(() =>
      useGraphLens({ request: req, lens: 'impact', subjectType: 'plan', subjectId: 'p1' })
    )
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(callRequestRpc).toHaveBeenCalledWith('impact.graph', {
      requestId: 'r1', subjectType: 'plan', subjectId: 'p1', lens: 'impact', maxNodes: 500
    })
    expect(result.current.payload?.nodes[0].risk).toBe('high')
  })

  it('does not call the RPC for client lenses', () => {
    const built = emptyGraphPayload('plan')
    const { result } = renderHook(() => useGraphLens({ request: req, lens: 'plan', buildClient: () => built }))
    expect(callRequestRpc).not.toHaveBeenCalled()
    expect(result.current.status).toBe('ready')
    expect(result.current.payload).toBe(built)
  })

  it('serves a second mount from cache', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: raw })
    const first = renderHook(() => useGraphLens({ request: req, lens: 'data', subjectId: 's' }))
    await waitFor(() => expect(first.result.current.status).toBe('ready'))
    first.unmount()
    const second = renderHook(() => useGraphLens({ request: req, lens: 'data', subjectId: 's' }))
    expect(second.result.current.status).toBe('ready')
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('refetches on impact.assessed for the same request only', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: raw })
    const { result } = renderHook(() => useGraphLens({ request: req, lens: 'impact', subjectId: 's' }))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    act(() => emitRequestEvent({ requestId: 'other', eventType: 'impact.assessed', occurredAt: '' }))
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    act(() => emitRequestEvent({ requestId: 'r1', eventType: 'orca.request.impact.assessed', occurredAt: '' }))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
  })

  it('goes idle on unsupported and error on forbidden', async () => {
    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'unsupported', code: 'x', message: 'm' } })
    const a = renderHook(() => useGraphLens({ request: req, lens: 'impact', subjectId: 'u' }))
    await waitFor(() => expect(a.result.current.error?.kind).toBe('unsupported'))
    expect(a.result.current.status).toBe('idle')
    expect(a.result.current.payload).toBeNull()

    callRequestRpc.mockResolvedValueOnce({ ok: false, error: { kind: 'forbidden', code: 'x', message: 'm' } })
    const b = renderHook(() => useGraphLens({ request: req, lens: 'impact', subjectId: 'f' }))
    await waitFor(() => expect(b.result.current.status).toBe('error'))
  })

  it('retries while the assessment is pending', async () => {
    vi.useFakeTimers()
    callRequestRpc
      .mockResolvedValueOnce({ ok: false, error: { kind: 'unknown', code: 'c', message: 'REQUEST_RISK_ASSESSMENT_PENDING: wait' } })
      .mockResolvedValueOnce({ ok: true, value: raw })
    const { result } = renderHook(() => useGraphLens({ request: req, lens: 'impact', subjectId: 'pend' }))
    await act(async () => { await vi.advanceTimersByTimeAsync(10) })
    expect(result.current.status).toBe('loading')
    await act(async () => { await vi.advanceTimersByTimeAsync(3100) })
    expect(result.current.status).toBe('ready')
    expect(callRequestRpc).toHaveBeenCalledTimes(2)
  })

  it('does not set state after unmount', async () => {
    let resolve: (v: unknown) => void = () => {}
    callRequestRpc.mockReturnValue(new Promise((r) => { resolve = r }))
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { unmount } = renderHook(() => useGraphLens({ request: req, lens: 'impact', subjectId: 'un' }))
    unmount()
    await act(async () => resolve({ ok: true, value: raw }))
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })
})
