// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore: any = {
  tasks: [],
  setTasks: vi.fn((t) => { mockStore.tasks = t }),
  setActiveTask: vi.fn(),
  settings: {}
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: any) => fn ? fn(mockStore) : mockStore,
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useTasks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.tasks = []
    mockRpc.mockResolvedValue([])
  })

  it('fetches tasks via task.list(projectId) on mount', async () => {
    mockRpc.mockResolvedValueOnce([{ id: 't1', title: 'Task 1', projectId: 'p1', status: 'todo' }])
    const { useTasks } = await import('../useTasks')
    renderHook(() => useTasks('p1'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.list', { projectId: 'p1' })
      expect(mockStore.setTasks).toHaveBeenCalledWith([{ id: 't1', title: 'Task 1', projectId: 'p1', status: 'todo' }])
    })
  })

  it('filterStatus=\'done\' → filteredTasks contains only done tasks', async () => {
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
})
