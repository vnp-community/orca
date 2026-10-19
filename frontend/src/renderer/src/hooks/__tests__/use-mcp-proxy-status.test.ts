// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))

const mockCallRpc = vi.mocked(callRuntimeRpc)

// Minimal mock for ensureRuntimeEnvironmentCompatible (called internally)
vi.mock('../../runtime/runtime-compatibility-cache', () => ({
  ensureRuntimeEnvironmentCompatible: vi.fn().mockResolvedValue(undefined),
  clearRuntimeCompatibilityCacheForTests: vi.fn(),
}))

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { useMCPProxyStatus } = await import('../use-mcp-proxy-status')

describe('useMCPProxyStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('initial status is unconfigured', () => {
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    expect(result.current.statusInfo.status).toBe('unconfigured')
  })

  it('status=unconfigured when configured=false', async () => {
    mockCallRpc.mockResolvedValueOnce({ configured: false })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unconfigured')
  })

  it('status=unreachable when reachable=false', async () => {
    mockCallRpc.mockResolvedValueOnce({
      configured: true,
      url: 'http://proxy:3012',
      reachable: false,
      error: 'connection refused',
    })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unreachable')
    expect(result.current.statusInfo.url).toBe('http://proxy:3012')
  })

  it('status=auth-error when authOk=false', async () => {
    mockCallRpc.mockResolvedValueOnce({
      configured: true,
      url: 'http://proxy:3012',
      reachable: true,
      authOk: false,
    })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('auth-error')
  })

  it('status=connected when reachable=true and authOk=true', async () => {
    mockCallRpc.mockResolvedValueOnce({
      configured: true,
      url: 'http://proxy:3012',
      reachable: true,
      authOk: true,
    })
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('connected')
    expect(result.current.statusInfo.latencyMs).toBeGreaterThanOrEqual(0)
  })

  it('status=checking while call is in flight', async () => {
    let resolve: (v: unknown) => void = () => {}
    mockCallRpc.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    act(() => {
      void result.current.checkStatus()
    })
    expect(result.current.statusInfo.status).toBe('checking')
    await act(async () => { resolve({ configured: false }) })
  })

  it('status=unreachable when callRuntimeRpc throws', async () => {
    mockCallRpc.mockRejectedValueOnce(new Error('network error'))
    const { result } = renderHook(() => useMCPProxyStatus('env-1'))
    await act(() => result.current.checkStatus())
    expect(result.current.statusInfo.status).toBe('unreachable')
    expect(result.current.statusInfo.error).toContain('network error')
  })
})
