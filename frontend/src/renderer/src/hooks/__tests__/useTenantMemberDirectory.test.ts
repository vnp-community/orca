// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: { getState: () => ({ settings: {} }) }
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { describeMemberLabel, useTenantMemberDirectory } from '../useTenantMemberDirectory'

describe('useTenantMemberDirectory', () => {
  beforeEach(() => {
    vi.mocked(callRuntimeRpc).mockReset()
  })

  it('fetches the directory via auth.listTenantMemberDirectory on mount', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { id: 'u1', name: 'Alice', email: 'alice@example.com' }
    ])
    const { result } = renderHook(() => useTenantMemberDirectory())

    expect(callRuntimeRpc).toHaveBeenCalledWith(
      'mock-target',
      'auth.listTenantMemberDirectory',
      null
    )
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.directory).toEqual([
      { id: 'u1', name: 'Alice', email: 'alice@example.com' }
    ])
  })

  it('falls back to an empty directory when the RPC rejects', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue(new Error('offline'))
    const { result } = renderHook(() => useTenantMemberDirectory())

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.directory).toEqual([])
  })
})

describe('describeMemberLabel', () => {
  const directory = [{ id: 'u1', name: 'Alice', email: 'alice@example.com' }]

  it('resolves a known userId to "name (email)"', () => {
    expect(describeMemberLabel('u1', directory)).toBe('Alice (alice@example.com)')
  })

  it('falls back to the raw userId when it is not in the directory', () => {
    expect(describeMemberLabel('u-unknown', directory)).toBe('u-unknown')
  })
})
