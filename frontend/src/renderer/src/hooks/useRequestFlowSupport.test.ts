// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { useRequestFlowSupport, _clearRequestFlowSupportCache } from './useRequestFlowSupport'

beforeEach(() => {
  callRequestRpc.mockReset()
  _clearRequestFlowSupportCache()
  useAppStore.setState({ requestFlowSupport: 'unknown' })
})

describe('useRequestFlowSupport', () => {
  it('maps enabled=true to supported', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { enabled: true } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('supported'))
  })

  it('maps enabled=false and unsupported errors to unsupported', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { enabled: false } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('unsupported'))

    _clearRequestFlowSupportCache()
    useAppStore.setState({ requestFlowSupport: 'unknown' })
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported' } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('unsupported'))
  })

  it('keeps unknown on a network error', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'network' } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalled())
    expect(useAppStore.getState().requestFlowSupport).toBe('unknown')
  })
})
