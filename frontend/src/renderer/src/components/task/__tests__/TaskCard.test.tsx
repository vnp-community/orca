// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskCard } from '../TaskCard'
import type { OrcaTask, TaskStatus } from '../../../../../shared/task-types'

type TestTask = {
  id: string
  parentId?: string
  status?: TaskStatus
}

// We need to mock useAppStore to control `hasChildren`
const mockStore = {
  tasks: [] as TestTask[]
}
vi.mock('../../../store', () => ({
  useAppStore: vi.fn((selector) => selector(mockStore))
}))

// Mock icons
vi.mock('lucide-react', () => ({
  ChevronDown: () => <span data-testid="chevron-down" />,
  ChevronRight: () => <span data-testid="chevron-right" />,
  Plus: () => <span data-testid="plus-icon" />
}))

describe('TaskCard', () => {
  const mockTask = {
    id: 't1',
    title: 'Test Task',
    type: 'feature',
    status: 'todo',
    priority: 'high',
    progressPercent: 50
  } as unknown as OrcaTask

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockStore.tasks = []
  })

  it('renders task title and type badge', () => {
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('Test Task')).toBeInTheDocument()
    expect(screen.getByText('feature')).toBeInTheDocument()
  })

  it('renders progressPercent when > 0', () => {
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('50%')).toBeInTheDocument()
  })

  it('shows expand chevron when task has children', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByTestId('chevron-right')).toBeInTheDocument()
  })

  it('shows chevron down when expanded', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    render(
      <TaskCard task={mockTask} depth={0} isExpanded={true} onToggle={vi.fn()} onSelect={vi.fn()} />
    )
    expect(screen.getByTestId('chevron-down')).toBeInTheDocument()
  })

  it('no chevron when task has no children', () => {
    mockStore.tasks = [] // No children
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.queryByTestId('chevron-right')).not.toBeInTheDocument()
    expect(screen.queryByTestId('chevron-down')).not.toBeInTheDocument()
  })

  it('clicking card calls onSelect', () => {
    const onSelect = vi.fn()
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={onSelect}
      />
    )
    fireEvent.click(screen.getByText('Test Task'))
    expect(onSelect).toHaveBeenCalledWith('t1')
  })

  it('clicking chevron calls onToggle', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    const onToggle = vi.fn()
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={onToggle}
        onSelect={vi.fn()}
      />
    )
    fireEvent.click(screen.getByTestId('chevron-right'))
    expect(onToggle).toHaveBeenCalledWith('t1')
  })

  it('no subtask button when onCreateSubtask is not passed', () => {
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.queryByTestId('task-add-subtask-t1')).not.toBeInTheDocument()
  })

  it('clicking subtask button calls onCreateSubtask(task.id), not onSelect', () => {
    const onCreateSubtask = vi.fn()
    const onSelect = vi.fn()
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={onSelect}
        onCreateSubtask={onCreateSubtask}
      />
    )
    fireEvent.click(screen.getByTestId('task-add-subtask-t1'))
    expect(onCreateSubtask).toHaveBeenCalledWith('t1')
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('shows checkbox when selectMode is true, reflects isSelected', () => {
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
        selectMode
        isSelected
      />
    )
    expect(screen.getByTestId('task-select-t1')).toBeChecked()
  })

  it('no checkbox when selectMode is false', () => {
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.queryByTestId('task-select-t1')).not.toBeInTheDocument()
  })

  it('toggling checkbox calls onToggleSelect(task.id), not onSelect', () => {
    const onToggleSelect = vi.fn()
    const onSelect = vi.fn()
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={onSelect}
        selectMode
        isSelected={false}
        onToggleSelect={onToggleSelect}
      />
    )
    fireEvent.click(screen.getByTestId('task-select-t1'))
    expect(onToggleSelect).toHaveBeenCalledWith('t1')
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('parent task with children shows client-computed progress (done-ratio), not raw progressPercent', () => {
    // mockTask.progressPercent is 50 (stale/static) — 2/2 done children should override it to 100%.
    mockStore.tasks = [
      { id: 'c1', parentId: 't1', status: 'done' },
      { id: 'c2', parentId: 't1', status: 'done' }
    ]
    render(
      <TaskCard
        task={mockTask}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('100%')).toBeInTheDocument()
    expect(screen.queryByText('50%')).not.toBeInTheDocument()
  })

  it('leaf task (no children) shows its own progressPercent', () => {
    mockStore.tasks = []
    render(
      <TaskCard
        task={{ ...mockTask, progressPercent: 30 }}
        depth={0}
        isExpanded={false}
        onToggle={vi.fn()}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('30%')).toBeInTheDocument()
  })
})
