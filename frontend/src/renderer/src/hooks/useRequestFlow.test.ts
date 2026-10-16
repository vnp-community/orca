// @vitest-environment happy-dom
import { renderHook, waitFor, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { resetRequestFlowCache, useRequestFlow } from './useRequestFlow'

beforeEach(() => {
  callRequestRpc.mockReset()
  resetRequestFlowCache()
})
afterEach(cleanup)

const wire = {
  type: 'bug',
  analysisKind: 'diagnosis',
  analysisGate: 'solution',
  planKind: 'plan',
  hasPhases: true,
  startGate: 'plan',
  executionGates: [],
  completesAfterAnalysis: false,
  humanConfirmRequired: false,
  openSpecProfile: 'light'
}

describe('useRequestFlow', () => {
  it('answers from the fallback table at once, then from the backend', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: wire })
    const { result } = renderHook(() => useRequestFlow('bug', 'L'))
    expect(result.current?.source).toBe('fallback')
    expect(result.current?.hasPhases).toBe(true)
    await waitFor(() => expect(result.current?.source).toBe('backend'))
    expect(callRequestRpc).toHaveBeenCalledWith('request.flow', { type: 'bug', size: 'L' })
  })

  it('asks the gateway once for the same type and size', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: wire })
    const first = renderHook(() => useRequestFlow('bug', 'L'))
    await waitFor(() => expect(first.result.current?.source).toBe('backend'))
    const second = renderHook(() => useRequestFlow('bug', 'L'))
    expect(second.result.current?.source).toBe('backend')
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
  })

  it('keeps the fallback when the gateway has no such channel', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported' } })
    const { result } = renderHook(() => useRequestFlow('change_request', 'M'))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(result.current).toMatchObject({
      source: 'fallback',
      planKind: 'plan',
      hasPhases: true,
      executionGates: ['phase']
    })
  })

  it('keeps the fallback when the payload is not a flow', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { nonsense: true } })
    const { result } = renderHook(() => useRequestFlow('task', 'S'))
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(result.current?.source).toBe('fallback')
  })

  it('is null for an unknown or missing type and never calls the gateway', () => {
    expect(renderHook(() => useRequestFlow('unknown')).result.current).toBeNull()
    expect(renderHook(() => useRequestFlow(undefined)).result.current).toBeNull()
    expect(callRequestRpc).not.toHaveBeenCalled()
  })
})
