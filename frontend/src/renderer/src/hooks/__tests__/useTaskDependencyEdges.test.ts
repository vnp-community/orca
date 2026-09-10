// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../../shared/task-types'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

const mockRpc = vi.mocked(callRuntimeRpc)

function makeTask(id: string): OrcaTask {
  return { id, title: id } as unknown as OrcaTask
}

describe('useTaskDependencyEdges', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('empty tasks → empty edges, no RPC call', async () => {
    const { useTaskDependencyEdges } = await import('../useTaskDependencyEdges')
    const emptyTasks: OrcaTask[] = []
    const { result } = renderHook(() => useTaskDependencyEdges(emptyTasks))
    expect(result.current.edges.size).toBe(0)
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('fetches task.getDependencies(taskId) for every task; response is a flat Task[] (no edgeType)', async () => {
    mockRpc.mockImplementation((_t, _m, params: unknown) =>
      Promise.resolve(
        (params as { taskId: string }).taskId === 'b' ? [{ id: 'a', title: 'A' }] : []
      )
    )
    const { useTaskDependencyEdges } = await import('../useTaskDependencyEdges')
    // A stable `tasks` reference matters: useTaskDependencyEdges re-fetches whenever the
    // `tasks` array reference changes, so a fresh literal on every render would loop forever.
    const tasks = [makeTask('a'), makeTask('b')]
    const { result } = renderHook(() => useTaskDependencyEdges(tasks))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.getDependencies', { taskId: 'a' })
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.getDependencies', { taskId: 'b' })
    expect(result.current.edges.get('b')?.blockedBy).toEqual(['a'])
  })

  it('derives the reverse "blocks" direction by cross-scanning blockedBy across the batch', async () => {
    // b depends on a → a "blocks" b
    mockRpc.mockImplementation((_t, _m, params: unknown) =>
      Promise.resolve(
        (params as { taskId: string }).taskId === 'b' ? [{ id: 'a', title: 'A' }] : []
      )
    )
    const { useTaskDependencyEdges } = await import('../useTaskDependencyEdges')
    const tasks = [makeTask('a'), makeTask('b')]
    const { result } = renderHook(() => useTaskDependencyEdges(tasks))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.edges.get('a')?.blocks).toEqual(['b'])
    expect(result.current.edges.get('b')?.blocks).toEqual([])
  })

  it('a failed fetch for one task sets error=true but does not throw / block others', async () => {
    mockRpc.mockImplementation((_t, _m, params: unknown) => {
      if ((params as { taskId: string }).taskId === 'a') {
        return Promise.reject(new Error('boom'))
      }
      return Promise.resolve([])
    })
    const { useTaskDependencyEdges } = await import('../useTaskDependencyEdges')
    const tasks = [makeTask('a'), makeTask('b')]
    const { result } = renderHook(() => useTaskDependencyEdges(tasks))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe(true)
    expect(result.current.edges.get('b')).toBeDefined()
  })

  it('refetch() re-runs the fetch even when the `tasks` array reference is unchanged', async () => {
    mockRpc.mockResolvedValue([])
    const { useTaskDependencyEdges } = await import('../useTaskDependencyEdges')
    const tasks = [makeTask('a')]
    const { result } = renderHook(() => useTaskDependencyEdges(tasks))

    await waitFor(() => expect(result.current.loading).toBe(false))
    const callsAfterMount = mockRpc.mock.calls.length

    act(() => result.current.refetch())

    await waitFor(() => expect(mockRpc.mock.calls.length).toBeGreaterThan(callsAfterMount))
  })
})
