// @vitest-environment happy-dom
// FE-TASK-001 (task-graph v4): no TaskGraph.test.tsx existed before this task — created new
// (the task file said MODIFY, but the file was never actually present in this worktree).
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskGraph } from '../TaskGraph'
import { useTasks } from '../../../hooks/useTasks'

vi.mock('../../../hooks/useTasks', () => ({
  useTasks: vi.fn()
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
  TaskCreateDialog: ({ onCreated, onCancel }: { onCreated: () => void; onCancel: () => void }) => (
    <div data-testid="mock-task-create-dialog">
      <button data-testid="mock-dialog-created" onClick={onCreated}>
        Created
      </button>
      <button data-testid="mock-dialog-cancel" onClick={onCancel}>
        Cancel
      </button>
    </div>
  )
}))

describe('TaskGraph', () => {
  const refetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useTasks).mockReturnValue({
      filteredTasks: [],
      expandedNodes: new Set(),
      toggleExpanded: vi.fn(),
      setActiveTask: vi.fn(),
      filterStatus: 'all',
      setFilterStatus: vi.fn(),
      searchQuery: '',
      setSearchQuery: vi.fn(),
      isLoading: false,
      refetch,
      dagView: null
    } as unknown as ReturnType<typeof useTasks>)
  })

  afterEach(cleanup)

  it('bấm "+ New Task" → TaskCreateDialog xuất hiện', () => {
    render(<TaskGraph projectId="p1" />)
    expect(screen.queryByTestId('mock-task-create-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByTestId('new-task-btn'))
    expect(screen.getByTestId('mock-task-create-dialog')).toBeInTheDocument()
  })

  it("TaskCreateDialog's onCreated → gọi refetch() và đóng dialog", () => {
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByTestId('new-task-btn'))
    fireEvent.click(screen.getByTestId('mock-dialog-created'))
    expect(refetch).toHaveBeenCalled()
    expect(screen.queryByTestId('mock-task-create-dialog')).not.toBeInTheDocument()
  })

  it("TaskCreateDialog's onCancel → đóng dialog, không gọi refetch", () => {
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByTestId('new-task-btn'))
    fireEvent.click(screen.getByTestId('mock-dialog-cancel'))
    expect(refetch).not.toHaveBeenCalled()
    expect(screen.queryByTestId('mock-task-create-dialog')).not.toBeInTheDocument()
  })

  it('renders Tree view by default', () => {
    render(<TaskGraph projectId="p1" />)
    expect(screen.getByTestId('mock-tree-view')).toBeInTheDocument()
  })

  // FE-TASK-003 (task-graph v4): + Board view toggle.
  it('bấm nút "Board" → viewMode đổi, TaskBoardView render thay Tree/DAG', async () => {
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByTestId('view-board'))
    expect(await screen.findByTestId('mock-board-view')).toBeInTheDocument()
    expect(screen.queryByTestId('mock-tree-view')).not.toBeInTheDocument()
  })
})
