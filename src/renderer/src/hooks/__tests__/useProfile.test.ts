// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useProfile, useProfileActions } from '../useProfile'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

const { mockStore } = vi.hoisted(() => ({
  mockStore: {
    setProfileLoading: vi.fn(),
    setResolved: vi.fn(),
    setUserProfile: vi.fn(),
    resolvedProfile: null,
    userProfile: null,
    profileIsLoading: false,
  }
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    vi.fn(selector => selector(mockStore)),
    { getState: vi.fn().mockReturnValue({ ...mockStore, settings: {} }) }
  )
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

describe('useProfile', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(callRuntimeRpc).mockResolvedValue({})
  })

  it('fetches userProfile and resolvedProfile on mount', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_, method) => {
      if (method === 'profile.getResolved') return { security: { approvedModels: ['gpt-4'] } }
      if (method === 'profile.getUser') return { agent: { preferredModel: 'gpt-3' } }
      return {}
    })
    
    renderHook(() => useProfile())
    
    await waitFor(() => {
      expect(mockStore.setResolved).toHaveBeenCalledWith({ security: { approvedModels: ['gpt-4'] } })
      expect(mockStore.setUserProfile).toHaveBeenCalledWith({ agent: { preferredModel: 'gpt-3' } })
    })
  })
})

describe('useProfileActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(callRuntimeRpc).mockResolvedValue({})
  })

  it('saveProfile scope=user calls profile.updateUser', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', { agent: { preferredModel: 'claude' } })
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateUser',
      { profile: { agent: { preferredModel: 'claude' } } }
    )
  })

  it('saveProfile scope=user re-fetches resolved after save', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', {})
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.getResolved',
      {}
    )
    expect(mockStore.setResolved).toHaveBeenCalled()
  })

  it('saveProfile scope=company calls profile.updateCompany', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('company', {})
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateCompany',
      { profile: {} }
    )
  })

  it('saveProfile scope=dept calls profile.updateDept with deptId', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('dept', {}, 'dept-123')
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateDept',
      { deptId: 'dept-123', profile: {} }
    )
  })
})
