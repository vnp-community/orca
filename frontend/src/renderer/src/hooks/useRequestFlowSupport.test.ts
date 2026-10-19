// @vitest-environment happy-dom
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, it, expect, vi, beforeEach } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import { useAppStore } from '../store'
import { supportsBaseRef } from '../store/slices/request'
import { useRequestFlowSupport, _clearRequestFlowSupportCache } from './useRequestFlowSupport'

// Why: without globals RTL does not unmount between tests, and a leftover hook would probe again when settings change.
afterEach(cleanup)

beforeEach(() => {
  callRequestRpc.mockReset()
  _clearRequestFlowSupportCache()
  useAppStore.setState({
    requestFlowSupport: 'unknown',
    requestFlowFeatures: new Set(),
    settings: null
  })
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

describe('useRequestFlowSupport features (CR-REQ-040)', () => {
  const baseRefOn = () => supportsBaseRef(useAppStore.getState())

  it('stores the features of a gateway that sends them and enables supportsBaseRef', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { enabled: true, features: ['request.baseRef', 'request.other'] }
    })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(baseRefOn()).toBe(true))
    expect([...useAppStore.getState().requestFlowFeatures]).toEqual([
      'request.baseRef',
      'request.other'
    ])
    expect(useAppStore.getState().requestFlowSupport).toBe('supported')
  })

  it('treats a gateway without a features field (older than CR-REQ-039) as having none', async () => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { enabled: true } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('supported'))
    expect(useAppStore.getState().requestFlowFeatures.size).toBe(0)
    expect(baseRefOn()).toBe(false)
  })

  it('is not fooled by a gateway that lists other features only', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { enabled: true, features: ['request.x'] }
    })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('supported'))
    expect(baseRefOn()).toBe(false)
  })

  it.each([
    ['a string', 'request.baseRef'],
    ['an object', { 'request.baseRef': true }],
    ['null', null]
  ])('ignores features that are %s and drops non-string entries', async (_name, features) => {
    callRequestRpc.mockResolvedValue({ ok: true, value: { enabled: true, features } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('supported'))
    expect(baseRefOn()).toBe(false)
  })

  it('keeps only the string entries of a mixed list', async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { enabled: true, features: [1, null, 'request.baseRef'] }
    })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(baseRefOn()).toBe(true))
    expect(useAppStore.getState().requestFlowFeatures.size).toBe(1)
  })

  it('clears features when the request-service is disabled or unsupported', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set(['request.baseRef']) })
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { enabled: false, features: ['request.baseRef'] }
    })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('unsupported'))
    expect(baseRefOn()).toBe(false)
  })

  it('keeps no features after an unsupported error', async () => {
    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'unsupported' } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('unsupported'))
    expect(useAppStore.getState().requestFlowFeatures.size).toBe(0)
  })

  it('does not drop the last known features on a network error', async () => {
    callRequestRpc.mockResolvedValueOnce({
      ok: true,
      value: { enabled: true, features: ['request.baseRef'] }
    })
    const first = renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(baseRefOn()).toBe(true))
    first.unmount()

    callRequestRpc.mockResolvedValue({ ok: false, error: { kind: 'network' } })
    renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    expect(baseRefOn()).toBe(true)
  })

  it("never shows one gateway's features on another: switching target clears them until the probe answers", async () => {
    callRequestRpc.mockResolvedValue({
      ok: true,
      value: { enabled: true, features: ['request.baseRef'] }
    })
    const view = renderHook(() => useRequestFlowSupport())
    await waitFor(() => expect(baseRefOn()).toBe(true))

    let answer: (value: unknown) => void = () => {}
    callRequestRpc.mockReturnValue(new Promise((resolve) => (answer = resolve)))
    useAppStore.setState({
      settings: { activeRuntimeEnvironmentId: 'env-old-gateway' } as never
    })
    view.rerender()
    await waitFor(() => expect(callRequestRpc).toHaveBeenCalledTimes(2))
    expect(baseRefOn()).toBe(false)

    answer({ ok: true, value: { enabled: true } })
    await waitFor(() => expect(useAppStore.getState().requestFlowSupport).toBe('supported'))
    expect(baseRefOn()).toBe(false)
  })
})
