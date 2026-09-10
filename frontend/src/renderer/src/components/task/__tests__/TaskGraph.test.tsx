// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskGraph } from '../TaskGraph'
import { useTasks } from '../../../hooks/useTasks'
import { useTaskDependencyEdges } from '../../../hooks/useTaskDependencyEdges'
import { useTaskBatchExecution } from '../../../hooks/useTaskBatchExecution'

vi.mock('../../../hooks/useTasks', () => ({
  useTasks: vi.fn()
}))

vi.mock('../../../hooks/useTaskDependencyEdges', () => ({
  useTaskDependencyEdges: vi.fn()
}))

vi.mock('../../../hooks/useTaskBatchExecution', () => ({
  useTaskBatchExecution: vi.fn()
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

vi.mock('../TaskTreeView', () => ({
  TaskTreeView: () => <div data-testid="mock-tree-view" />
}))
vi.mock('../TaskDAGView', () => ({
  default: () => <div data-testid="mock-dag-view" />
}))
vi.mock('../TaskBoardView', () => ({
  TaskBoardView: () => <div data-testid="mock-board-view" />
}))
vi.mock('../TaskCreateDialog', () => ({
  TaskCreateDialog: ({
    open,
    onCreate
  }: {
    open: boolean
    onCreate: (title: string, parentId?: string) => Promise<void>
  }) =>
    open ? (
      <div data-testid="mock-task-create-dialog">
        <button data-testid="mock-created" onClick={() => onCreate('New Task')}>
          created
        </button>
      </div>
    ) : null
}))

const mockUseTasks = vi.mocked(useTasks)
const mockUseTaskDependencyEdges = vi.mocked(useTaskDependencyEdges)
const mockUseTaskBatchExecution = vi.mocked(useTaskBatchExecution)

describe('TaskGraph', () => {
  const createTask = vi.fn().mockResolvedValue({ id: 't-new' })

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    createTask.mockResolvedValue({ id: 't-new' })
    mockUseTasks.mockReturnValue({
      filteredTasks: [],
      expandedNodes: new Set(),
      toggleExpanded: vi.fn(),
      setActiveTask: vi.fn(),
      filterStatus: 'all',
      setFilterStatus: vi.fn(),
      searchQuery: '',
      setSearchQuery: vi.fn(),
      isLoading: false,
      refetch: vi.fn(),
      dagView: null,
      createTask,
      selectedIds: new Set<string>(),
      toggleSelected: vi.fn(),
      clearSelection: vi.fn()
    } as unknown as ReturnType<typeof useTasks>)
    mockUseTaskDependencyEdges.mockReturnValue({
      edges: new Map(),
      loading: false,
      error: false,
      refetch: vi.fn()
    })
    mockUseTaskBatchExecution.mockReturnValue({
      running: false,
      results: new Map(),
      runSelected: vi.fn().mockResolvedValue(new Map())
    } as unknown as ReturnType<typeof useTaskBatchExecution>)
  })

  it('clicking "+ New Task" shows TaskCreateDialog', () => {
    render(<TaskGraph projectId="p1" />)
    expect(screen.queryByTestId('mock-task-create-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByTestId('new-task-btn'))
    expect(screen.getByTestId('mock-task-create-dialog')).toBeInTheDocument()
  })

  it('TaskCreateDialog onCreate calls createTask(title, parentId)', async () => {
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByTestId('new-task-btn'))
    fireEvent.click(screen.getByTestId('mock-created'))
    await waitFor(() => {
      expect(createTask).toHaveBeenCalledWith('New Task', undefined)
    })
  })

  it('clicking "Board" switches viewMode, renders TaskBoardView instead of Tree/DAG', async () => {
    render(<TaskGraph projectId="p1" />)
    expect(screen.getByTestId('mock-tree-view')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('view-board'))
    await waitFor(() => {
      expect(screen.getByTestId('mock-board-view')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('mock-tree-view')).not.toBeInTheDocument()
  })
})
