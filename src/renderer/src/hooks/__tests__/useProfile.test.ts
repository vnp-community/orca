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

// Mock spans expose id/step/ok/fail so tests can assert both the RPC
// `traceId` forwarding AND the span lifecycle calls (start/ok/fail fields).
const { resolveSpan, updateSpan, uiProfileResolveFlowStart, uiProfileUpdateFlowStart } = vi.hoisted(() => {
  const resolveSpan = { id: 'resolve-span-id', step: vi.fn(), ok: vi.fn(), fail: vi.fn() }
  const updateSpan = { id: 'update-span-id', step: vi.fn(), ok: vi.fn(), fail: vi.fn() }
  return {
    resolveSpan,
    updateSpan,
    uiProfileResolveFlowStart: vi.fn(() => resolveSpan),
    uiProfileUpdateFlowStart: vi.fn(() => updateSpan),
  }
})

vi.mock('../../../../shared/trace/tracers', () => ({
  Tracers: {
    uiProfileResolveFlow: { start: uiProfileResolveFlowStart },
    uiProfileUpdateFlow: { start: uiProfileUpdateFlowStart },
  }
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

  // --- TASK-FE-015.1: ui:profile.resolve tracer coverage ---

  it('starts ui:profile.resolve span on mount and marks ok with hasSecurityLock', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_, method) => {
      if (method === 'profile.getResolved') return { security: { approvedModels: ['gpt-4'] } }
      if (method === 'profile.getUser') return { agent: { preferredModel: 'gpt-3' } }
      return {}
    })

    renderHook(() => useProfile())

    expect(uiProfileResolveFlowStart).toHaveBeenCalledWith()

    await waitFor(() => {
      expect(resolveSpan.ok).toHaveBeenCalledWith({ hasSecurityLock: true })
    })
  })

  it('marks ui:profile.resolve span failed when one of the two RPCs rejects', async () => {
    const err = new Error('network down')
    vi.mocked(callRuntimeRpc).mockImplementation(async (_, method) => {
      if (method === 'profile.getResolved') throw err
      return {}
    })

    renderHook(() => useProfile())

    await waitFor(() => {
      expect(resolveSpan.fail).toHaveBeenCalledWith(err)
    })
    expect(resolveSpan.ok).not.toHaveBeenCalled()
  })

  it('forwards the same span traceId into both profile.getResolved and profile.getUser', async () => {
    renderHook(() => useProfile())

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        expect.anything(),
        'profile.getResolved',
        { traceId: 'resolve-span-id' }
      )
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        expect.anything(),
        'profile.getUser',
        { traceId: 'resolve-span-id' }
      )
    })
  })
})

describe('useProfileActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(callRuntimeRpc).mockResolvedValue({})
  })

  it('saveProfile scope=user calls profile.updateUser with traceId', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', { agent: { preferredModel: 'claude' } })
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateUser',
      { profile: { agent: { preferredModel: 'claude' } }, traceId: 'update-span-id' }
    )
  })

  it('saveProfile scope=user re-fetches resolved after save WITHOUT traceId (not part of the update span)', async () => {
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

  it('saveProfile scope=company calls profile.updateCompany with traceId', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('company', {})
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateCompany',
      { profile: {}, traceId: 'update-span-id' }
    )
  })

  it('saveProfile scope=dept calls profile.updateDept with deptId and traceId', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('dept', {}, 'dept-123')
    })
    expect(callRuntimeRpc).toHaveBeenCalledWith(
      expect.anything(),
      'profile.updateDept',
      { deptId: 'dept-123', profile: {}, traceId: 'update-span-id' }
    )
  })

  // --- TASK-FE-015.2: ui:profile.update tracer coverage ---

  it('starts ui:profile.update span with scope=user field', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('user', {})
    })
    expect(uiProfileUpdateFlowStart).toHaveBeenCalledWith({ scope: 'user', targetId: undefined })
  })

  it('sets targetId field on the span when scope=dept', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('dept', {}, 'dept-123')
    })
    expect(uiProfileUpdateFlowStart).toHaveBeenCalledWith({ scope: 'dept', targetId: 'dept-123' })
  })

  it('marks span ok with scope field on success', async () => {
    const { result } = renderHook(() => useProfileActions())
    await act(async () => {
      await result.current.saveProfile('company', {})
    })
    expect(updateSpan.ok).toHaveBeenCalledWith({ scope: 'company' })
  })

  it('marks span failed with scope field and re-throws before toast.error handles it', async () => {
    const err = new Error('save failed')
    vi.mocked(callRuntimeRpc).mockRejectedValueOnce(err)

    const { result } = renderHook(() => useProfileActions())
    await expect(
      act(async () => {
        await result.current.saveProfile('user', {})
      })
    ).rejects.toThrow('save failed')

    expect(updateSpan.fail).toHaveBeenCalledWith(err, { scope: 'user' })
  })
})
