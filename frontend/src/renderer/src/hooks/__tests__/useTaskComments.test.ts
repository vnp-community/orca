/**
 * Tests for `useTaskComments` (TASK-FE-TASKV1-08).
 *
 * `task.listComments`/`task.addComment` don't exist on backend-go (confirmed: 0 RPCs in
 * task.proto's TaskService), and Node only has `task.addComment` — no matching read RPC
 * (confirmed by reading backend/src/main/task/task-rpc-handler.ts). So the mount probe
 * fails on every deploy target today; these tests cover both that gate and the shape the
 * hook would produce once a real read RPC exists.
 *
 * @module renderer/hooks/__tests__/useTaskComments.test
 */
// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore = { settings: {} }

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => mockStore })
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTaskComments()', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('mount → probes task.listComments; on failure isSupported flips false (today: every deploy target)', async () => {
    mockRpc.mockRejectedValueOnce(new Error('method not found'))
    const { useTaskComments } = await import('../useTaskComments')
    const { result } = renderHook(() => useTaskComments('t1'))

    await waitFor(() => {
      expect(result.current.isSupported).toBe(false)
    })
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.listComments', { taskId: 't1' })
  })

  it('mount → task.listComments success populates comments, isSupported stays true', async () => {
    const comments = [
      { id: 1, taskId: 't1', userId: 'u1', content: 'hi', type: 'comment', createdAt: new Date() }
    ]
    mockRpc.mockResolvedValueOnce({ comments })
    const { useTaskComments } = await import('../useTaskComments')
    const { result } = renderHook(() => useTaskComments('t1'))

    await waitFor(() => {
      expect(result.current.comments).toEqual(comments)
    })
    expect(result.current.isSupported).toBe(true)
  })

  it('addComment() calls task.addComment then refetches via task.listComments (addComment response has no comment payload)', async () => {
    mockRpc.mockResolvedValueOnce({ comments: [] }) // mount probe
    const { useTaskComments } = await import('../useTaskComments')
    const { result } = renderHook(() => useTaskComments('t1'))
    await waitFor(() => expect(result.current.isSupported).toBe(true))

    mockRpc.mockResolvedValueOnce({ added: true }) // task.addComment
    const refetched = [
      { id: 2, taskId: 't1', userId: 'u1', content: 'new', type: 'comment', createdAt: new Date() }
    ]
    mockRpc.mockResolvedValueOnce({ comments: refetched }) // refetch task.listComments

    await act(async () => {
      await result.current.addComment('new')
    })

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.addComment', {
      taskId: 't1',
      content: 'new'
    })
    expect(result.current.comments).toEqual(refetched)
  })
})
