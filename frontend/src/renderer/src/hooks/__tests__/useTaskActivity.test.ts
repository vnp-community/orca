// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
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

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTaskActivity', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('mount với taskId → gọi task.get ngay lần đầu (không đợi hết interval)', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.get', { taskId: 't1' })
    expect(mockRpc).toHaveBeenCalledTimes(1)
  })

  it('advance 4000ms → gọi task.get lần 2, state.task cập nhật theo response mới', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { result } = renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })
    expect(result.current.task).toEqual({ id: 't1', status: 'todo' })

    mockRpc.mockResolvedValue({ id: 't1', status: 'done' })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })

    expect(mockRpc).toHaveBeenCalledTimes(2)
    expect(result.current.task).toEqual({ id: 't1', status: 'done' })
  })

  it('unmount → clearInterval, không còn RPC nào sau đó', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { unmount } = renderHook(() => useTaskActivity('t1'))

    await act(async () => {
      await Promise.resolve()
    })
    const callsAtUnmount = mockRpc.mock.calls.length
    unmount()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000)
    })
    expect(mockRpc).toHaveBeenCalledTimes(callsAtUnmount)
  })

  it('taskId null → không effect nào chạy, không RPC nào', async () => {
    const { useTaskActivity } = await import('../useTaskActivity')
    renderHook(() => useTaskActivity(null))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('isLive luôn false — chưa có kênh WS thật', async () => {
    mockRpc.mockResolvedValue({ id: 't1', status: 'todo' })
    const { useTaskActivity } = await import('../useTaskActivity')
    const { result } = renderHook(() => useTaskActivity('t1'))
    expect(result.current.isLive).toBe(false)
  })
})
