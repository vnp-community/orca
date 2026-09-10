// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useWorkspace } from '../../context/WorkspaceContext'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

const mockRpc = vi.mocked(callRuntimeRpc)
const mockUseWorkspace = vi.mocked(useWorkspace)

describe('useTaskBatchExecution', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseWorkspace.mockReturnValue({
      project: { id: 'p1' },
      currentWorktree: { id: 'wt-1', path: '/repo/p1', branch: 'main', isMain: true }
    } as unknown as ReturnType<typeof useWorkspace>)
  })

  it('runSelected() dispatches task.execute for every taskId with projectId/worktreePath/traceId', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    await act(async () => {
      await result.current.runSelected(['t1', 't2'])
    })

    expect(mockRpc).toHaveBeenCalledWith(
      'mock-target',
      'task.execute',
      expect.objectContaining({ taskId: 't1', projectId: 'p1', worktreePath: '/repo/p1' })
    )
    expect(mockRpc).toHaveBeenCalledWith(
      'mock-target',
      'task.execute',
      expect.objectContaining({ taskId: 't2', projectId: 'p1', worktreePath: '/repo/p1' })
    )
  })

  it('respects MAX_CONCURRENCY=3 — never more than 3 in-flight task.execute calls at once', async () => {
    let inFlight = 0
    let maxInFlight = 0
    mockRpc.mockImplementation(() => {
      inFlight++
      maxInFlight = Math.max(maxInFlight, inFlight)
      return new Promise((resolve) => {
        setTimeout(() => {
          inFlight--
          resolve(undefined)
        }, 5)
      })
    })
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    await act(async () => {
      await result.current.runSelected(['t1', 't2', 't3', 't4', 't5', 't6', 't7'])
    })

    expect(maxInFlight).toBeLessThanOrEqual(3)
    expect(mockRpc).toHaveBeenCalledTimes(7)
  })

  it('marks a task "error" (not throwing) when task.execute rejects, and continues the rest', async () => {
    mockRpc.mockImplementation((_t, _m, params: unknown) =>
      (params as { taskId: string }).taskId === 't2'
        ? Promise.reject(new Error('dispatch failed'))
        : Promise.resolve(undefined)
    )
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    let resultsMap: Map<string, 'ok' | 'error'> = new Map()
    await act(async () => {
      resultsMap = await result.current.runSelected(['t1', 't2'])
    })

    expect(resultsMap.get('t1')).toBe('ok')
    expect(resultsMap.get('t2')).toBe('error')
  })

  it('running flag is true during the batch and false again after it settles', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    expect(result.current.running).toBe(false)
    let runPromise!: Promise<unknown>
    act(() => {
      runPromise = result.current.runSelected(['t1'])
    })
    await waitFor(() => expect(result.current.running).toBe(true))
    await act(async () => {
      await runPromise
    })
    expect(result.current.running).toBe(false)
  })

  it('no project or worktree → no-op, returns an empty result map', async () => {
    mockUseWorkspace.mockReturnValue({
      project: null,
      currentWorktree: null
    } as unknown as ReturnType<typeof useWorkspace>)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    let resultsMap: Map<string, 'ok' | 'error'> = new Map()
    await act(async () => {
      resultsMap = await result.current.runSelected(['t1'])
    })

    expect(mockRpc).not.toHaveBeenCalled()
    expect(resultsMap.size).toBe(0)
  })
})
