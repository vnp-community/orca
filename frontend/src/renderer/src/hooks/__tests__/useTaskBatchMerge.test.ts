// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
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

function task(id: string, overrides: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    title: `Task ${id}`,
    taskNumber: 1,
    status: 'done',
    priority: 'medium',
    projectId: 'p1',
    labels: [],
    worktreeId: `wt-${id}`,
    ...overrides
  } as OrcaTask
}

describe('useTaskBatchMerge', () => {
  beforeEach(() => vi.clearAllMocks())

  it('merges every task sequentially, one worktree.merge call per task, with cleanupWorktreeIds set', async () => {
    mockRpc.mockResolvedValue({ resultSha: 'sha-1', hasConflicts: false })
    const { useTaskBatchMerge } = await import('../useTaskBatchMerge')
    const { result } = renderHook(() => useTaskBatchMerge())

    let outcomes!: Map<string, unknown>
    await act(async () => {
      outcomes = await result.current.batchMergeWorktrees([task('t1'), task('t2')], 'main', 'merge')
    })

    expect(mockRpc).toHaveBeenCalledTimes(2)
    expect(mockRpc).toHaveBeenNthCalledWith(
      1,
      'mock-target',
      'worktree.merge',
      expect.objectContaining({
        worktreeId: 'wt-t1',
        baseBranch: 'main',
        strategy: 'merge',
        cleanupWorktreeIds: ['wt-t1']
      })
    )
    expect(outcomes.get('t1')).toEqual({ ok: true, resultSha: 'sha-1' })
    expect(outcomes.get('t2')).toEqual({ ok: true, resultSha: 'sha-1' })
  })

  it('stops the batch at the first conflict — never calls worktree.merge for tasks after it', async () => {
    mockRpc.mockResolvedValueOnce({ hasConflicts: true, conflictedPaths: ['a.go'] })
    const { useTaskBatchMerge } = await import('../useTaskBatchMerge')
    const { result } = renderHook(() => useTaskBatchMerge())

    let outcomes!: Map<string, unknown>
    await act(async () => {
      outcomes = await result.current.batchMergeWorktrees(
        [task('t1'), task('t2'), task('t3')],
        'main',
        'merge'
      )
    })

    expect(mockRpc).toHaveBeenCalledTimes(1)
    expect(outcomes.get('t1')).toEqual({ ok: 'conflict', paths: ['a.go'] })
    expect(outcomes.has('t2')).toBe(false)
    expect(outcomes.has('t3')).toBe(false)
  })

  it('stops the batch at the first error — never calls worktree.merge for tasks after it', async () => {
    mockRpc.mockRejectedValueOnce(new Error('merge failed'))
    const { useTaskBatchMerge } = await import('../useTaskBatchMerge')
    const { result } = renderHook(() => useTaskBatchMerge())

    let outcomes!: Map<string, unknown>
    await act(async () => {
      outcomes = await result.current.batchMergeWorktrees([task('t1'), task('t2')], 'main', 'merge')
    })

    expect(mockRpc).toHaveBeenCalledTimes(1)
    expect(outcomes.get('t1')).toEqual({ ok: false, error: 'merge failed' })
    expect(outcomes.has('t2')).toBe(false)
  })

  it('a task with no worktreeId is reported as an error, batch stops there without calling the RPC', async () => {
    mockRpc.mockResolvedValue({ hasConflicts: false, resultSha: 'sha-1' })
    const { useTaskBatchMerge } = await import('../useTaskBatchMerge')
    const { result } = renderHook(() => useTaskBatchMerge())

    let outcomes!: Map<string, unknown>
    await act(async () => {
      outcomes = await result.current.batchMergeWorktrees(
        [task('t1', { worktreeId: undefined })],
        'main',
        'merge'
      )
    })

    expect(mockRpc).not.toHaveBeenCalled()
    expect(outcomes.get('t1')).toEqual({
      ok: false,
      error: 'Task has no worktreeId — nothing to merge'
    })
  })
})
