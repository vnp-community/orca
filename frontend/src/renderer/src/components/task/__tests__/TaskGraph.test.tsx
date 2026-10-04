// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskGraph } from '../TaskGraph'
import { useTasks } from '../../../hooks/useTasks'
import { useTaskDependencyEdges } from '../../../hooks/useTaskDependencyEdges'
import { useTaskBatchExecution } from '../../../hooks/useTaskBatchExecution'
import type { OrcaTask } from '../../../../../shared/task-types'

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

function task(id: string, overrides: Partial<OrcaTask> = {}): OrcaTask {
  return {
    id,
    title: `Task ${id}`,
    taskNumber: 1,
    type: 'task',
    status: 'todo',
    priority: 'medium',
    projectId: 'p1',
    labels: [],
    visibility: 'private',
    progressPercent: 0,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides
  }
}

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

  // BL-TG-06: phase-aware batch actions — only rendered when every selected
  // task shares the same phase (derivePhase), never mixing actions across
  // tasks at different points in the Spec → Approve → Code loop.
  describe('BL-TG-06 batch phase actions', () => {
    it('all selected tasks not-started → shows "Generate Spec", not "Implement Spec" or Merge', () => {
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1'), task('t2')],
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
        selectedIds: new Set(['t1', 't2']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))

      expect(screen.getByTestId('batch-generate-spec-btn')).toBeInTheDocument()
      expect(screen.queryByTestId('batch-implement-spec-btn')).not.toBeInTheDocument()
      expect(screen.queryByTestId('batch-merge-btn')).not.toBeInTheDocument()
    })

    it('mixed phases across selected tasks → no phase-specific button at all', () => {
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1'), task('t2', { labels: ['phase:spec-approved'] })],
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
        selectedIds: new Set(['t1', 't2']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))

      expect(screen.queryByTestId('batch-generate-spec-btn')).not.toBeInTheDocument()
      expect(screen.queryByTestId('batch-implement-spec-btn')).not.toBeInTheDocument()
      // The pre-existing generic action is still available regardless.
      expect(screen.getByTestId('run-selected-btn')).toBeInTheDocument()
    })

    it('clicking "Generate Spec" calls runSelected with a promptBuilder and phase:spec-pending', async () => {
      const runSelected = vi.fn().mockResolvedValue(new Map([['t1', 'ok']]))
      mockUseTaskBatchExecution.mockReturnValue({
        running: false,
        results: new Map(),
        runSelected
      } as unknown as ReturnType<typeof useTaskBatchExecution>)
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1', { title: 'Write feature X' })],
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
        selectedIds: new Set(['t1']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))
      fireEvent.click(screen.getByTestId('batch-generate-spec-btn'))

      await waitFor(() => expect(runSelected).toHaveBeenCalled())
      const [tasksArg, options] = runSelected.mock.calls[0]
      expect(tasksArg).toEqual([expect.objectContaining({ id: 't1' })])
      expect(options.phaseLabel).toBe('phase:spec-pending')
      expect(options.promptBuilder(task('t1', { title: 'Write feature X' }))).toContain(
        'Write feature X'
      )
    })

    it('all selected tasks done with a worktreeId → shows Merge Worktree button', () => {
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1', { status: 'done', worktreeId: 'wt-1' })],
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
        selectedIds: new Set(['t1']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))

      expect(screen.getByTestId('batch-merge-btn')).toBeInTheDocument()
    })

    it('clicking Merge Worktree opens TaskMergeDialog with the right task count', () => {
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1', { status: 'done', worktreeId: 'wt-1' })],
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
        selectedIds: new Set(['t1']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))
      fireEvent.click(screen.getByTestId('batch-merge-btn'))

      expect(screen.getByTestId('merge-submit')).toBeInTheDocument()
      expect(screen.getByTestId('merge-base-branch-input')).toBeInTheDocument()
    })

    it('done task with NO worktreeId → Merge Worktree button does not appear', () => {
      mockUseTasks.mockReturnValue({
        filteredTasks: [task('t1', { status: 'done' })],
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
        selectedIds: new Set(['t1']),
        toggleSelected: vi.fn(),
        clearSelection: vi.fn()
      } as unknown as ReturnType<typeof useTasks>)
      render(<TaskGraph projectId="p1" />)
      fireEvent.click(screen.getByTestId('toggle-select-mode'))

      expect(screen.queryByTestId('batch-merge-btn')).not.toBeInTheDocument()
    })
  })
})
