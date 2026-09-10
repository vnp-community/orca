// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() }
}))
import { toast } from 'sonner'
const mockToast = vi.mocked(toast)
const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTaskGrants', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("addGrant(subjectId, 'admin', true) → calls task.grant({taskId, subjectId, level:'admin', applyTree:true})", async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    await act(async () => {
      await result.current.addGrant('user-9', 'admin', true)
    })

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.grant', {
      taskId: 't1',
      subjectId: 'user-9',
      level: 'admin',
      applyTree: true
    })
    expect(mockToast.success).toHaveBeenCalledWith('Granted admin to user-9')
  })

  it('addGrant error → toast.error, re-throws to the caller', async () => {
    const err = new Error('denied')
    mockRpc.mockRejectedValueOnce(err)
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    await expect(
      act(async () => {
        await result.current.addGrant('user-9', 'user', false)
      })
    ).rejects.toThrow('denied')

    expect(mockToast.error).toHaveBeenCalledWith('Failed to grant: denied')
  })

  it('revoke() → toast.info "not available yet", calls no RPC', async () => {
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    act(() => {
      result.current.revoke('user-9')
    })

    expect(mockToast.info).toHaveBeenCalledWith(
      'Revoke grant is not available yet (pending BE-SOL-003)'
    )
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('generateShareLink() → toast.info "not available yet", calls no RPC', async () => {
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    act(() => {
      result.current.generateShareLink()
    })

    expect(mockToast.info).toHaveBeenCalledWith(
      'Share link is not available yet (pending BE-SOL-003)'
    )
    expect(mockRpc).not.toHaveBeenCalled()
  })
})
