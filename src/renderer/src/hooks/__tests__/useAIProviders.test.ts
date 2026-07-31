// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore: any = {
  aiAccounts:          [],
  isLoadingAIAccounts: false,
  setAIAccounts:       vi.fn((a: any[]) => { mockStore.aiAccounts = a }),
  updateAIAccountStatus: vi.fn(),
  removeAIAccount:     vi.fn(),
  setLoadingAIAccounts: vi.fn((v: boolean) => { mockStore.isLoadingAIAccounts = v }),
  getState:            () => mockStore,
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: any) => fn ? fn(mockStore) : mockStore,
    { getState: () => mockStore }
  ),
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

describe('useAIProviders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.aiAccounts = []
    mockStore.isLoadingAIAccounts = false
  })

  it('fetches accounts on mount', async () => {
    mockRpc.mockResolvedValueOnce([{ id: 'acc1', provider: 'anthropic' }])
    const { useAIProviders } = await import('../useAIProviders')
    renderHook(() => useAIProviders())
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'aiProvider.list', { devServerId: undefined })
    expect(mockStore.setAIAccounts).toHaveBeenCalledWith([{ id: 'acc1', provider: 'anthropic' }])
  })

  it('devServerId filter applied to fetch', async () => {
    mockRpc.mockResolvedValueOnce([])
    const { useAIProviders } = await import('../useAIProviders')
    renderHook(() => useAIProviders('srv1'))
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'aiProvider.list', { devServerId: 'srv1' })
  })

  it('testConnection ok → status active', async () => {
    mockRpc.mockResolvedValueOnce([])    // mount fetch
    mockRpc.mockResolvedValueOnce({ ok: true, latencyMs: 120 })
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.testConnection('acc1') })
    expect(mockStore.updateAIAccountStatus).toHaveBeenCalledWith('acc1', 'healthy')
  })

  it('testConnection fail → status invalid', async () => {
    mockRpc.mockResolvedValueOnce([])
    mockRpc.mockResolvedValueOnce({ ok: false, latencyMs: 0, error: 'Invalid key' })
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.testConnection('acc1') })
    expect(mockStore.updateAIAccountStatus).toHaveBeenCalledWith('acc1', 'invalid')
  })

  it('refresh re-fetches accounts', async () => {
    mockRpc.mockResolvedValue([])
    const { useAIProviders } = await import('../useAIProviders')
    const { result } = renderHook(() => useAIProviders())
    await act(async () => { await result.current.refresh() })
    expect(mockRpc).toHaveBeenCalledTimes(2)  // mount + manual refresh
  })
})
