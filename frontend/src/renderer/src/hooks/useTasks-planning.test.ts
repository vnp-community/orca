// @vitest-environment happy-dom
import { renderHook, act, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { OrcaTask } from '../../../shared/task-types'

const rpc = vi.hoisted(() => ({ tasks: [] as unknown[] }))
vi.mock('../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => ({ tasks: rpc.tasks })),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))

import { useAppStore } from '../store'
import { useTasks } from './useTasks'

function mk(id: string, o: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    projectId: 'p1',
    title: `T ${id}`,
    type: 'task',
    status: 'todo',
    priority: 'medium',
    labels: [],
    visibility: 'private',
    progressPercent: 0,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...o
  }
}

describe('useTasks plan/phase filtering', () => {
  beforeEach(() => {
    rpc.tasks = []
  })

  it('hides plan/phase by default, promotes children, and shows them when toggled', async () => {
    rpc.tasks = [
      mk('plan', { type: 'plan', title: 'Plan' }),
      mk('ph', { type: 'phase', parentId: 'plan', title: 'Phase' }),
      mk('w', { parentId: 'ph' })
    ]
    useAppStore.setState({ tasks: rpc.tasks as OrcaTask[] })
    const { result } = renderHook(() => useTasks('p1'))
    await waitFor(() => expect(result.current.hasPlanningTasks).toBe(true))
    expect(result.current.filteredTasks.map((t) => t.id)).toEqual(['w'])
    expect(result.current.filteredTasks[0].parentId).toBeUndefined()

    act(() => result.current.setShowPlanningTasks(true))
    expect(result.current.filteredTasks.map((t) => t.id)).toEqual(['plan', 'ph', 'w'])
    expect(result.current.filteredTasks[2].parentId).toBe('ph')
  })

  it('keeps the filtered array contents untouched when no plan/phase exists', async () => {
    rpc.tasks = [mk('a'), mk('b', { parentId: 'a' })]
    useAppStore.setState({ tasks: rpc.tasks as OrcaTask[] })
    const { result } = renderHook(() => useTasks('p1'))
    await waitFor(() => expect(result.current.filteredTasks).toHaveLength(2))
    expect(result.current.hasPlanningTasks).toBe(false)
    expect(result.current.filteredTasks[1]).toBe((rpc.tasks as OrcaTask[])[1])
  })
})
