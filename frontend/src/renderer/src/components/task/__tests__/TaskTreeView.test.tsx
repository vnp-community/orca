// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { ReactNode } from 'react'
import { TaskTreeView } from '../TaskTreeView'
import type { OrcaTask } from '../../../../../shared/task-types'

type TestTaskCardProps = {
  task: { id: string; title: string }
  isExpanded?: boolean
  onSelect: (id: string) => void
  children?: ReactNode
  onCreateSubtask?: (parentId: string) => void
  selectMode?: boolean
  isSelected?: boolean
  onToggleSelect?: (id: string) => void
}

// We can mock TaskCard to just render its children if expanded, to simplify DOM checks
vi.mock('../TaskCard', () => ({
  TaskCard: ({
    task,
    isExpanded,
    onSelect,
    children,
    onCreateSubtask,
    selectMode,
    isSelected,
    onToggleSelect
  }: TestTaskCardProps) => (
    <div data-testid={`mock-task-card-${task.id}`} onClick={() => onSelect(task.id)}>
      <span>{task.title}</span>
      {selectMode && (
        <button data-testid={`mock-select-${task.id}`} onClick={() => onToggleSelect?.(task.id)}>
          {isSelected ? 'selected' : 'unselected'}
        </button>
      )}
      {onCreateSubtask && (
        <button data-testid={`mock-subtask-${task.id}`} onClick={() => onCreateSubtask(task.id)}>
          + subtask
        </button>
      )}
      {isExpanded && <div data-testid={`children-of-${task.id}`}>{children}</div>}
    </div>
  )
}))

describe('TaskTreeView', () => {
  const tasks = [
    { id: 't1', title: 'Root Task 1', parentId: null },
    { id: 't2', title: 'Root Task 2', parentId: null },
    { id: 'c1', title: 'Child 1', parentId: 't1' },
    { id: 'c2', title: 'Child 2', parentId: 't1' },
    { id: 'g1', title: 'Grandchild 1', parentId: 'c1' }
  ] as OrcaTask[]

  beforeEach(() => {
    cleanup()
  })

  it('renders only root tasks initially (parentId=null/undefined)', () => {
    const expandedNodes = new Set<string>()
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
      />
    )

    expect(screen.getByTestId('mock-task-card-t1')).toBeInTheDocument()
    expect(screen.getByTestId('mock-task-card-t2')).toBeInTheDocument()
    // Children are NOT rendered
    expect(screen.queryByTestId('mock-task-card-c1')).not.toBeInTheDocument()
    expect(screen.queryByTestId('mock-task-card-g1')).not.toBeInTheDocument()
  })

  it('expand node → children become visible', () => {
    const expandedNodes = new Set(['t1'])
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
      />
    )

    expect(screen.getByTestId('mock-task-card-t1')).toBeInTheDocument()
    expect(screen.getByTestId('mock-task-card-c1')).toBeInTheDocument()
    expect(screen.getByTestId('mock-task-card-c2')).toBeInTheDocument()

    // Grandchild is still NOT visible because c1 is not expanded
    expect(screen.queryByTestId('mock-task-card-g1')).not.toBeInTheDocument()
  })

  it('nested 3-level tree works', () => {
    const expandedNodes = new Set(['t1', 'c1'])
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
      />
    )

    expect(screen.getByTestId('mock-task-card-g1')).toBeInTheDocument()
  })

  it('selecting task calls setActiveTask(task.id)', () => {
    const expandedNodes = new Set<string>()
    const setActiveTask = vi.fn()
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={setActiveTask}
      />
    )

    screen.getByTestId('mock-task-card-t1').click()
    expect(setActiveTask).toHaveBeenCalledWith('t1')
  })

  it('threads onCreateSubtask through to nested TaskCards (root and children)', () => {
    const expandedNodes = new Set(['t1'])
    const onCreateSubtask = vi.fn()
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
        onCreateSubtask={onCreateSubtask}
      />
    )
    screen.getByTestId('mock-subtask-t1').click()
    expect(onCreateSubtask).toHaveBeenCalledWith('t1')
    screen.getByTestId('mock-subtask-c1').click()
    expect(onCreateSubtask).toHaveBeenCalledWith('c1')
  })

  it('threads selectMode/selectedIds/onToggleSelect through to nested TaskCards', () => {
    const expandedNodes = new Set(['t1'])
    const onToggleSelect = vi.fn()
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
        selectMode
        selectedIds={new Set(['c1'])}
        onToggleSelect={onToggleSelect}
      />
    )
    expect(screen.getByTestId('mock-select-t1')).toHaveTextContent('unselected')
    expect(screen.getByTestId('mock-select-c1')).toHaveTextContent('selected')

    screen.getByTestId('mock-select-t1').click()
    expect(onToggleSelect).toHaveBeenCalledWith('t1')
  })

  it('selectMode/onCreateSubtask default to off/absent when not passed', () => {
    const expandedNodes = new Set<string>()
    render(
      <TaskTreeView
        tasks={tasks}
        expandedNodes={expandedNodes}
        toggleExpanded={vi.fn()}
        setActiveTask={vi.fn()}
      />
    )
    expect(screen.queryByTestId('mock-select-t1')).not.toBeInTheDocument()
    expect(screen.queryByTestId('mock-subtask-t1')).not.toBeInTheDocument()
  })
})
