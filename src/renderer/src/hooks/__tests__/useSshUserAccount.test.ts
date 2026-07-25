// @vitest-environment happy-dom
import { renderHook, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useSshUserAccount } from '../useSshUserAccount'
import * as runtimeRpcClient from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client')

describe('useSshUserAccount', () => {
  beforeEach(() => {
    vi.mocked(runtimeRpcClient.callRuntimeRpc).mockResolvedValue({
      linuxUsername: 'orca-alice',
      provisioned: true
    })
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('Fetches linux username cho serverId', async () => {
    const { result } = renderHook(() => useSshUserAccount('server-1'))
    
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })
    
    expect(result.current.linuxUsername).toBe('orca-alice')
    expect(result.current.provisioned).toBe(true)
    expect(runtimeRpcClient.callRuntimeRpc).toHaveBeenCalledWith(
      { kind: 'local' },
      'ssh.getUserAccount',
      { serverId: 'server-1' }
    )
  })

  it('Returns null linuxUsername while loading', async () => {
    // delay resolution to check loading state
    let resolvePromise: any
    const promise = new Promise(resolve => { resolvePromise = resolve })
    vi.mocked(runtimeRpcClient.callRuntimeRpc).mockReturnValue(promise as any)
    
    const { result } = renderHook(() => useSshUserAccount('server-1'))
    
    expect(result.current.isLoading).toBe(true)
    expect(result.current.linuxUsername).toBeNull()
    
    resolvePromise({ linuxUsername: 'orca-alice', provisioned: true })
    
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })
    expect(result.current.linuxUsername).toBe('orca-alice')
  })

  it('Computes predicted username từ email', () => {
    const { result } = renderHook(() => useSshUserAccount('server-1', { previewFromEmail: 'bob@co.com' }))
    expect(result.current.previewUsername).toBe('orca-bob')
  })
})
