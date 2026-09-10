/**
 * Tests for `useTaskActivity` (FE-TASK-003) — polling fallback for `task.get` since no
 * real push channel (`task.activity:{taskId}`) exists anywhere in the codebase yet.
 *
 * @module renderer/hooks/__tests__/useTaskActivity.test
 */
// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
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

describe('useTaskActivity()', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('mounting with a taskId calls task.get({id}) immediately, without waiting for the interval', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.get', { id: 't1' })
    expect(mockRpc).toHaveBeenCalledTimes(1)
  })

  it('advancing 4000ms calls task.get again and updates state.task from the response', async () => {
    mockRpc
      .mockResolvedValueOnce({ id: 't1', status: 'todo' })
      .mockResolvedValueOnce({ id: 't1', status: 'in_progress' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { result } = renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })
    expect(result.current.task?.status).toBe('todo')

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(mockRpc).toHaveBeenCalledTimes(2)
    expect(result.current.task?.status).toBe('in_progress')
  })

  it('unmounting clears the interval — no further task.get calls', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { unmount } = renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })
    unmount()
    mockRpc.mockClear()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000)
    })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('a null taskId runs no effect — no RPC call', async () => {
    const { useTaskActivity } = await import('../useTaskActivity')
    renderHook(() => useTaskActivity(null))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('isLive is always false (fixed until a real push channel exists)', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { result } = renderHook(() => useTaskActivity('t1'))
    expect(result.current.isLive).toBe(false)
  })

  it('a rejected poll does not throw or crash the hook', async () => {
    mockRpc.mockRejectedValue(new Error('network down'))
    const { useTaskActivity } = await import('../useTaskActivity')

    const { result } = renderHook(() => useTaskActivity('t1'))
    await act(async () => {
      await Promise.resolve()
    })

    expect(result.current.task).toBeNull()
  })
})
