// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callEnvelope = vi.fn()
vi.mock('../runtime/code-intel-client', () => ({ getCodeIntelClient: () => ({ callEnvelope }) }))

import { useDataFlows, type DataFlowsFilter } from './useDataFlows'

const flow = (id: string) => ({ id, label: id, trigger: { kind: 'grpc', name: 't' }, entryService: 's', entryRpc: 'r', serviceHops: 1, completeness: 'complete' })
const page = (ids: string[], next?: string, total = 0) => ({
  ok: true,
  envelope: { data: { flows: ids.map(flow) }, nextPageToken: next, totalCount: total }
})
const base: DataFlowsFilter = { query: '', triggerKind: null, service: null }

beforeEach(() => { vi.useFakeTimers(); callEnvelope.mockReset() })
afterEach(() => vi.useRealTimers())
const flush = () => act(async () => { await vi.advanceTimersByTimeAsync(0) })

describe('useDataFlows', () => {
  it('loads the first page with limit 100 and pages with pageToken', async () => {
    callEnvelope.mockResolvedValueOnce(page(['a', 'b'], 'tok', 5)).mockResolvedValueOnce(page(['b', 'c']))
    const { result } = renderHook(() => useDataFlows('w', null, base))
    await flush()
    expect(callEnvelope.mock.calls[0][2]).toEqual({ limit: 100 })
    expect(result.current).toMatchObject({ status: 'ready', total: 5, hasMore: true })
    act(() => result.current.loadMore())
    await flush()
    expect(callEnvelope.mock.calls[1][2]).toEqual({ limit: 100, pageToken: 'tok' })
    expect(result.current.flows.map((f) => f.id)).toEqual(['a', 'b', 'c'])
    expect(result.current.hasMore).toBe(false)
  })

  it('debounces the query by 300 ms and sends it to the server', async () => {
    callEnvelope.mockResolvedValue(page(['a']))
    const { rerender } = renderHook((p) => useDataFlows('w', null, p), { initialProps: base })
    await flush()
    callEnvelope.mockClear()
    rerender({ ...base, query: 'ord' })
    rerender({ ...base, query: 'orde' })
    await act(async () => { await vi.advanceTimersByTimeAsync(299) })
    expect(callEnvelope).not.toHaveBeenCalled()
    await act(async () => { await vi.advanceTimersByTimeAsync(2) })
    expect(callEnvelope).toHaveBeenCalledTimes(1)
    expect(callEnvelope.mock.calls[0][2]).toMatchObject({ query: 'orde' })
  })

  it('passes triggerKind and service filters', async () => {
    callEnvelope.mockResolvedValue(page([]))
    renderHook(() => useDataFlows('w', null, { query: '', triggerKind: 'http', service: 'orders' }))
    await flush()
    expect(callEnvelope.mock.calls[0][2]).toEqual({ limit: 100, triggerKind: 'http', service: 'orders' })
  })

  it('aborts the previous request when the filter changes', async () => {
    let first: AbortSignal | undefined
    callEnvelope.mockImplementationOnce((_w, _m, _p, _parse, opts) => { first = opts.signal; return new Promise(() => {}) })
    callEnvelope.mockResolvedValue(page(['x']))
    const { rerender, result } = renderHook((p) => useDataFlows('w', null, p), { initialProps: base })
    await flush()
    rerender({ ...base, service: 'other' })
    await flush()
    expect(first?.aborted).toBe(true)
    expect(result.current.flows.map((f) => f.id)).toEqual(['x'])
  })

  it('exposes errors for a retry', async () => {
    callEnvelope.mockResolvedValueOnce({ ok: false, error: { kind: 'timeout', code: null, message: 'slow', data: null, retryable: true } })
    const { result } = renderHook(() => useDataFlows('w', null, base))
    await flush()
    expect(result.current.status).toBe('error')
    callEnvelope.mockResolvedValueOnce(page(['a']))
    act(() => result.current.refetch())
    await flush()
    expect(result.current.status).toBe('ready')
  })
})
