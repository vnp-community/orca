// @vitest-environment happy-dom
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { useRequests } from './useRequests'

const row = (id: string) => ({ id, title: id, type: 'bug', status: 'submitted', projectId: 'p' })

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestFlowSupport: 'supported', requestsById: {} })
})

describe('useRequests', () => {
  it('loads, parses and mirrors into the store', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { requests: [row('a')] } })
    const { result } = renderHook(() => useRequests({ projectId: 'p' }))
    await waitFor(() => expect(result.current.requests).toHaveLength(1))
    expect(useAppStore.getState().requestsById.a).toBeDefined()
    expect(result.current.supported).toBe(true)
  })

  it('appends the next page and de-duplicates by id', async () => {
    callRequestRpc
      .mockResolvedValueOnce({ ok: true, value: { requests: [row('a'), row('b')], nextPageToken: 't1' } })
      .mockResolvedValueOnce({ ok: true, value: { requests: [row('b'), row('c')] } })
    const { result } = renderHook(() => useRequests())
    await waitFor(() => expect(result.current.nextPageToken).toBe('t1'))
    act(() => result.current.nextPage())
    await waitFor(() => expect(result.current.requests.map((r) => r.id)).toEqual(['a', 'b', 'c']))
    expect(callRequestRpc.mock.calls[1][1]).toMatchObject({ pageToken: 't1' })
  })

  it('exposes the error kind and does not call when unsupported', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'forbidden' } })
    const { result } = renderHook(() => useRequests())
    await waitFor(() => expect(result.current.error).toBe('forbidden'))

    callRequestRpc.mockClear()
    useAppStore.setState({ requestFlowSupport: 'unsupported' })
    const second = renderHook(() => useRequests())
    expect(callRequestRpc).not.toHaveBeenCalled()
    expect(second.result.current.supported).toBe(false)
  })
})
