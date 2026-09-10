// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../../shared/task-types'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

type MockStore = {
  tasks: unknown[]
  setTasks: (t: unknown[]) => void
  setActiveTask: (id: string | null) => void
  addTask: (t: unknown) => void
  settings: Record<string, never>
}

const mockStore: MockStore = {
  tasks: [],
  setTasks: vi.fn((t: unknown[]) => {
    mockStore.tasks = t
  }),
  setActiveTask: vi.fn(),
  addTask: vi.fn(),
  settings: {}
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (store: MockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTasks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.tasks = []
    mockStore.addTask = vi.fn((t: unknown) => {
      mockStore.tasks = [...mockStore.tasks, t]
    })
    // Why {tasks: [...]}, not a bare array: the real Go handler returns the
    // raw ListTasksResponse proto ({tasks, nextPageToken}) — pinning a bare
    // array here previously masked a live "s.filter is not a function" bug
    // (useTasks.ts stored the whole wrapper as if it were the array).
    mockRpc.mockResolvedValue({ tasks: [] })
  })

  it('fetches tasks via task.list(projectId) on mount', async () => {
    mockRpc.mockResolvedValueOnce({
      tasks: [{ id: 't1', title: 'Task 1', projectId: 'p1', status: 'todo' }]
    })
    const { useTasks } = await import('../useTasks')
    renderHook(() => useTasks('p1'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.list', { projectId: 'p1' })
      expect(mockStore.setTasks).toHaveBeenCalledWith([
        { id: 't1', title: 'Task 1', projectId: 'p1', status: 'todo' }
      ])
    })
  })

  it('defaults to an empty array when task.list returns a null tasks field', async () => {
    mockRpc.mockResolvedValueOnce({ tasks: null })
    const { useTasks } = await import('../useTasks')
    renderHook(() => useTasks('p1'))

    await waitFor(() => {
      expect(mockStore.setTasks).toHaveBeenCalledWith([])
    })
  })

  it("filterStatus='done' → filteredTasks contains only done tasks", async () => {
    mockStore.tasks = [
      { id: 't1', title: 'Task 1', projectId: 'p1', status: 'todo' },
      { id: 't2', title: 'Task 2', projectId: 'p1', status: 'done' }
    ]
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    act(() => {
      result.current.setFilterStatus('done')
    })

    expect(result.current.filteredTasks).toHaveLength(1)
    expect(result.current.filteredTasks[0].id).toBe('t2')
  })

  it('searchQuery filters tasks by title (case-insensitive)', async () => {
    mockStore.tasks = [
      { id: 't1', title: 'Hello World', projectId: 'p1', status: 'todo' },
      { id: 't2', title: 'Another task', projectId: 'p1', status: 'todo' }
    ]
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    act(() => {
      result.current.setSearchQuery('hello')
    })

    expect(result.current.filteredTasks).toHaveLength(1)
    expect(result.current.filteredTasks[0].id).toBe('t1')
  })

  it('toggleExpanded(id) adds id to expandedNodes Set', async () => {
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    act(() => {
      result.current.toggleExpanded('t1')
    })

    expect(result.current.expandedNodes.has('t1')).toBe(true)
  })

  it('refetch() re-calls task.list with the same projectId', async () => {
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledTimes(1)
    })

    act(() => {
      result.current.refetch()
    })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledTimes(2)
    })
    expect(mockRpc).toHaveBeenNthCalledWith(2, 'mock-target', 'task.list', { projectId: 'p1' })
  })

  it('toggleExpanded(id) again removes id (toggle behavior)', async () => {
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    act(() => {
      result.current.toggleExpanded('t1')
    })
    expect(result.current.expandedNodes.has('t1')).toBe(true)

    act(() => {
      result.current.toggleExpanded('t1')
    })
    expect(result.current.expandedNodes.has('t1')).toBe(false)
  })

  it('createTask() calls task.create and defaults missing OrcaTask fields on the response', async () => {
    mockRpc.mockResolvedValueOnce({ tasks: [] }) // task.list on mount
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))
    await waitFor(() =>
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.list', { projectId: 'p1' })
    )

    // backend-go's CreateTaskResponse only returns {id, title, status, parentId, projectId}
    mockRpc.mockResolvedValueOnce({
      id: 't-new',
      title: 'New Task',
      status: 'todo',
      projectId: 'p1'
    })

    let created!: OrcaTask
    await act(async () => {
      created = await result.current.createTask('New Task', undefined)
    })

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.create', {
      title: 'New Task',
      parentId: undefined,
      projectId: 'p1'
    })
    expect(created).toMatchObject({
      id: 't-new',
      title: 'New Task',
      type: 'task',
      priority: 'medium',
      labels: [],
      visibility: 'private',
      progressPercent: 0
    })
    expect(mockStore.addTask).toHaveBeenCalledWith(created)
  })

  it('selectedIds/toggleSelected/clearSelection manage a Set of selected task ids', async () => {
    const { useTasks } = await import('../useTasks')
    const { result } = renderHook(() => useTasks('p1'))

    expect(result.current.selectedIds.size).toBe(0)

    act(() => result.current.toggleSelected('t1'))
    expect(result.current.selectedIds.has('t1')).toBe(true)

    act(() => result.current.toggleSelected('t1'))
    expect(result.current.selectedIds.has('t1')).toBe(false)

    act(() => result.current.toggleSelected('t1'))
    act(() => result.current.clearSelection())
    expect(result.current.selectedIds.size).toBe(0)
  })

  it('resets selectedIds when projectId changes', async () => {
    const { useTasks } = await import('../useTasks')
    const { result, rerender } = renderHook(({ projectId }) => useTasks(projectId), {
      initialProps: { projectId: 'p1' }
    })

    act(() => result.current.toggleSelected('t1'))
    expect(result.current.selectedIds.has('t1')).toBe(true)

    rerender({ projectId: 'p2' })
    await waitFor(() => expect(result.current.selectedIds.size).toBe(0))
  })
})

describe('computeClientProgress', () => {
  it('returns null for a task with no children (leaf task)', async () => {
    const { computeClientProgress } = await import('../useTasks')
    expect(
      computeClientProgress([{ id: 't1', parentId: undefined } as unknown as OrcaTask], 't1')
    ).toBeNull()
  })

  it('returns the % of direct children with status done', async () => {
    const { computeClientProgress } = await import('../useTasks')
    const tasks = [
      { id: 'parent', parentId: undefined },
      { id: 'c1', parentId: 'parent', status: 'done' },
      { id: 'c2', parentId: 'parent', status: 'todo' },
      { id: 'c3', parentId: 'parent', status: 'done' },
      { id: 'c4', parentId: 'parent', status: 'in_progress' }
    ] as unknown as OrcaTask[]
    expect(computeClientProgress(tasks, 'parent')).toBe(50)
  })

  it('returns 100 when all children are done', async () => {
    const { computeClientProgress } = await import('../useTasks')
    const tasks = [
      { id: 'parent', parentId: undefined },
      { id: 'c1', parentId: 'parent', status: 'done' },
      { id: 'c2', parentId: 'parent', status: 'done' }
    ] as unknown as OrcaTask[]
    expect(computeClientProgress(tasks, 'parent')).toBe(100)
  })
})
