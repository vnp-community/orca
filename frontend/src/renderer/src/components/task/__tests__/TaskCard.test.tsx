// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskCard } from '../TaskCard'

// We need to mock useAppStore to control `hasChildren`
const mockStore = {
  tasks: [] as any[]
}
vi.mock('../../../store', () => ({
  useAppStore: vi.fn(selector => selector(mockStore))
}))

// Mock icons
vi.mock('lucide-react', () => ({
  ChevronDown: () => <span data-testid="chevron-down" />,
  ChevronRight: () => <span data-testid="chevron-right" />
}))

describe('TaskCard', () => {
  const mockTask = {
    id: 't1',
    title: 'Test Task',
    type: 'feature',
    status: 'todo',
    priority: 'high',
    progressPercent: 50,
  } as any

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockStore.tasks = []
  })

  it('renders task title and type badge', () => {
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={vi.fn()} onSelect={vi.fn()} />)
    expect(screen.getByText('Test Task')).toBeInTheDocument()
    expect(screen.getByText('feature')).toBeInTheDocument()
  })

  it('renders progressPercent when > 0', () => {
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={vi.fn()} onSelect={vi.fn()} />)
    expect(screen.getByText('50%')).toBeInTheDocument()
  })

  it('shows expand chevron when task has children', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={vi.fn()} onSelect={vi.fn()} />)
    expect(screen.getByTestId('chevron-right')).toBeInTheDocument()
  })

  it('shows chevron down when expanded', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    render(<TaskCard task={mockTask} depth={0} isExpanded={true} onToggle={vi.fn()} onSelect={vi.fn()} />)
    expect(screen.getByTestId('chevron-down')).toBeInTheDocument()
  })

  it('no chevron when task has no children', () => {
    mockStore.tasks = [] // No children
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={vi.fn()} onSelect={vi.fn()} />)
    expect(screen.queryByTestId('chevron-right')).not.toBeInTheDocument()
    expect(screen.queryByTestId('chevron-down')).not.toBeInTheDocument()
  })

  it('clicking card calls onSelect', () => {
    const onSelect = vi.fn()
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={vi.fn()} onSelect={onSelect} />)
    fireEvent.click(screen.getByText('Test Task'))
    expect(onSelect).toHaveBeenCalledWith('t1')
  })

  it('clicking chevron calls onToggle', () => {
    mockStore.tasks = [{ id: 'child-1', parentId: 't1' }]
    const onToggle = vi.fn()
    render(<TaskCard task={mockTask} depth={0} isExpanded={false} onToggle={onToggle} onSelect={vi.fn()} />)
    fireEvent.click(screen.getByTestId('chevron-right'))
    expect(onToggle).toHaveBeenCalledWith('t1')
  })
})
