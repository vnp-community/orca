// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { useRequestSummaries } from './useRequestSummaries'

const wire = (id: string) => ({
  id, projectId: 'p', number: 1, title: `T-${id}`, type: 'bug', status: 'submitted',
  createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z'
})

beforeEach(() => {
  callRequestRpc.mockReset()
  useAppStore.setState({ requestsById: {} })
})

describe('useRequestSummaries', () => {
  it('caps concurrency at 4 and fills the store', async () => {
    let active = 0
    let peak = 0
    callRequestRpc.mockImplementation(async (_m: string, p: { id: string }) => {
      active++
      peak = Math.max(peak, active)
      await new Promise((r) => setTimeout(r, 5))
      active--
      return { ok: true, value: { request: wire(p.id) } }
    })
    const ids = Array.from({ length: 10 }, (_, i) => `r${i}`)
    const { result } = renderHook(() => useRequestSummaries(ids))
    await waitFor(() => expect(Object.keys(result.current.byId)).toHaveLength(10))
    expect(peak).toBeLessThanOrEqual(4)
    expect(peak).toBeGreaterThan(1)
    expect(callRequestRpc).toHaveBeenCalledTimes(10)
  })

  it('does not refetch ids already cached', async () => {
    useAppStore.setState({ requestsById: { r1: wire('r1') as never } })
    callRequestRpc.mockResolvedValue({ ok: true, value: { request: wire('r2') } })
    const { result } = renderHook(() => useRequestSummaries(['r1', 'r2', 'r2']))
    await waitFor(() => expect(result.current.byId.r2).toBeDefined())
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('isolates failures and flags not-found', async () => {
    callRequestRpc.mockImplementation(async (_m: string, p: { id: string }) =>
      p.id === 'bad'
        ? { ok: false, error: { kind: 'not_found', code: 'internal', message: 'REQUEST_NOT_FOUND: gone' } }
        : { ok: true, value: { request: wire(p.id) } })
    const { result } = renderHook(() => useRequestSummaries(['ok', 'bad']))
    await waitFor(() => expect(result.current.failedIds.has('bad')).toBe(true))
    expect(result.current.notFoundIds.has('bad')).toBe(true)
    await waitFor(() => expect(result.current.byId.ok).toBeDefined())
  })
})
