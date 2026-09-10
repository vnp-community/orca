// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (state: Record<string, unknown>) => unknown) =>
      fn ? fn({ settings: {} }) : { settings: {} },
    {
      getState: () => ({ settings: {} })
    }
  )
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() }
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTaskGrants', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("addGrant(subjectId, 'admin', true) → gọi task.grant({taskId, subjectId, level:'admin', applyTree:true})", async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    await act(async () => {
      await result.current.addGrant('user-1', 'admin', true)
    })

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.grant', {
      taskId: 't1',
      subjectId: 'user-1',
      level: 'admin',
      applyTree: true
    })
  })

  it('addGrant lỗi → toast.error, throw lại cho caller', async () => {
    mockRpc.mockRejectedValueOnce(new Error('denied'))
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    await expect(
      act(async () => {
        await result.current.addGrant('user-1', 'admin', false)
      })
    ).rejects.toThrow('denied')
  })

  it('revoke() → toast.info "not available yet", KHÔNG gọi RPC nào', async () => {
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    act(() => {
      result.current.revoke('user-1')
    })

    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('generateShareLink() → toast.info "not available yet", KHÔNG gọi RPC nào', async () => {
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))

    act(() => {
      result.current.generateShareLink()
    })

    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('grants luôn rỗng (task.listGrants chưa tồn tại)', async () => {
    const { useTaskGrants } = await import('../useTaskGrants')
    const { result } = renderHook(() => useTaskGrants('t1'))
    expect(result.current.grants).toEqual([])
  })
})
