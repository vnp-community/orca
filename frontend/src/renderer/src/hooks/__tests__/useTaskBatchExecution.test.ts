// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { useWorkspace } from '../../context/WorkspaceContext'
import type { OrcaTask } from '../../../../shared/task-types'

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

function task(id: string, overrides: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    title: `Task ${id}`,
    status: 'todo',
    priority: 'medium',
    projectId: 'p1',
    labels: [],
    ...overrides
  } as OrcaTask
}

describe('useTaskBatchExecution', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // BUG-025's own lesson: currentWorktree is legitimately null on the
    // Tasks tab — this hook must not require it (it used to, silently
    // no-op'ing every batch run whenever the sidebar had no worktree
    // selected, found live in BL-TG-06).
    mockUseWorkspace.mockReturnValue({
      project: { id: 'p1' },
      currentWorktree: null
    } as unknown as ReturnType<typeof useWorkspace>)
  })

  it('runSelected() dispatches task.execute for every task with projectId/traceId, no worktreePath (dead param — task.execute never reads it)', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    await act(async () => {
      await result.current.runSelected([task('t1'), task('t2')])
    })

    expect(mockRpc).toHaveBeenCalledWith(
      'mock-target',
      'task.execute',
      expect.objectContaining({ taskId: 't1', projectId: 'p1' })
    )
    expect(mockRpc).toHaveBeenCalledWith(
      'mock-target',
      'task.execute',
      expect.objectContaining({ taskId: 't2', projectId: 'p1' })
    )
    for (const call of mockRpc.mock.calls) {
      expect(call[2]).not.toHaveProperty('worktreePath')
    }
  })

  it('no project → no-op, returns an empty result map (currentWorktree is NOT required — BUG-025)', async () => {
    mockUseWorkspace.mockReturnValue({
      project: null,
      currentWorktree: null
    } as unknown as ReturnType<typeof useWorkspace>)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    let resultsMap: Map<string, 'ok' | 'error'> = new Map()
    await act(async () => {
      resultsMap = await result.current.runSelected([task('t1')])
    })

    expect(mockRpc).not.toHaveBeenCalled()
    expect(resultsMap.size).toBe(0)
  })

  it('runs even when currentWorktree is null — the Tasks tab has no worktree requirement of its own (BUG-025/BL-TG-06)', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    let resultsMap: Map<string, 'ok' | 'error'> = new Map()
    await act(async () => {
      resultsMap = await result.current.runSelected([task('t1')])
    })

    expect(resultsMap.get('t1')).toBe('ok')
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
      await result.current.runSelected(
        ['t1', 't2', 't3', 't4', 't5', 't6', 't7'].map((id) => task(id))
      )
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
      resultsMap = await result.current.runSelected([task('t1'), task('t2')])
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
      runPromise = result.current.runSelected([task('t1')])
    })
    await waitFor(() => expect(result.current.running).toBe(true))
    await act(async () => {
      await runPromise
    })
    expect(result.current.running).toBe(false)
  })

  // BL-TG-06: promptBuilder/phaseLabel let the same batch primitive drive
  // Generate Spec/Implement Spec hàng loạt, not just a bare "run these" —
  // the label write must land BEFORE task.execute, same ordering
  // TaskPromptEditor.tsx's single-task flow already established.
  it('promptBuilder + phaseLabel: writes the label via task.update BEFORE task.execute, sends the built prompt', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    await act(async () => {
      await result.current.runSelected([task('t1', { labels: ['priority:urgent'] })], {
        promptBuilder: (t) => `spec prompt for ${t.id}`,
        phaseLabel: 'phase:spec-pending'
      })
    })

    const updateCallIndex = mockRpc.mock.calls.findIndex((c) => c[1] === 'task.update')
    const executeCallIndex = mockRpc.mock.calls.findIndex((c) => c[1] === 'task.execute')
    expect(updateCallIndex).toBeGreaterThanOrEqual(0)
    expect(executeCallIndex).toBeGreaterThan(updateCallIndex)
    expect(mockRpc.mock.calls[updateCallIndex][2]).toEqual(
      expect.objectContaining({ id: 't1', labels: ['priority:urgent', 'phase:spec-pending'] })
    )
    expect(mockRpc.mock.calls[executeCallIndex][2]).toEqual(
      expect.objectContaining({ taskId: 't1', prompt: 'spec prompt for t1' })
    )
  })

  it('no promptBuilder/phaseLabel (default) → no task.update call at all, prompt is undefined', async () => {
    mockRpc.mockResolvedValue(undefined)
    const { useTaskBatchExecution } = await import('../useTaskBatchExecution')
    const { result } = renderHook(() => useTaskBatchExecution())

    await act(async () => {
      await result.current.runSelected([task('t1')])
    })

    expect(mockRpc.mock.calls.some((c) => c[1] === 'task.update')).toBe(false)
    const executeCall = mockRpc.mock.calls.find((c) => c[1] === 'task.execute')
    expect(executeCall?.[2]).toEqual(expect.objectContaining({ prompt: undefined }))
  })
})
