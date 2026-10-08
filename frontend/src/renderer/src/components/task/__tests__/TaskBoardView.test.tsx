// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskBoardView } from '../TaskBoardView'
import type { OrcaTask } from '../../../../../shared/task-types'

// Hand-rolled DataTransfer — happy-dom's native DataTransfer doesn't reliably support
// setData/getData across separate fireEvent calls, same pattern used elsewhere in this
// repo for drag payload unit tests (e.g. lib/linear-board-drag-payload.test.ts).
class FakeDataTransfer {
  private readonly data = new Map<string, string>()
  getData(type: string): string {
    return this.data.get(type) ?? ''
  }
  setData(type: string, value: string): void {
    this.data.set(type, value)
  }
}

// `fireEvent.dragStart`/`fireEvent.drop` each wrap `dataTransfer` in a brand-new native
// DataTransfer() (dom-testing-library's events.js — "DataTransfer is not supported in
// jsdom" workaround), so state set in dragStart never survives into drop. Dispatching a
// manually-built event through the base `fireEvent(node, event)` skips that wrapping and
// lets one FakeDataTransfer instance carry state across both events.
function fireDrag(
  eventType: 'dragstart' | 'drop',
  node: Element,
  dataTransfer: FakeDataTransfer
): void {
  const event = new Event(eventType, { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'dataTransfer', { value: dataTransfer })
  fireEvent(node, event)
}

type MockStore = {
  tasks: unknown[]
  settings: Record<string, unknown>
  updateTask: ReturnType<typeof vi.fn>
  requestFlowSupport: string
  setActiveView: ReturnType<typeof vi.fn>
  setRequestPageData: ReturnType<typeof vi.fn>
}

const mockStore: MockStore = {
  tasks: [],
  settings: {},
  updateTask: vi.fn(),
  requestFlowSupport: 'unknown',
  setActiveView: vi.fn(),
  setRequestPageData: vi.fn()
}

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (store: MockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))
import { toast } from 'sonner'
const mockToast = vi.mocked(toast)

function makeTask(overrides: Partial<OrcaTask>): OrcaTask {
  return {
    id: 't1',
    projectId: 'p1',
    title: 'Task',
    type: 'task',
    status: 'todo',
    priority: 'medium',
    labels: [],
    visibility: 'private',
    progressPercent: 0,
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides
  }
}

describe('TaskBoardView', () => {
  const onSelect = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.tasks = []
    mockStore.requestFlowSupport = 'unknown'
  })

  afterEach(cleanup)

  it('renders the columns in STATUS_ORDER, each counting its own tasks', () => {
    const tasks = [
      makeTask({ id: 't1', status: 'todo' }),
      makeTask({ id: 't2', status: 'done' }),
      makeTask({ id: 't3', status: 'done' })
    ]
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)

    const columns = [
      'board-column-todo',
      'board-column-in_progress',
      'board-column-blocked',
      'board-column-review',
      'board-column-done',
      'board-column-cancelled'
    ]
    for (const testId of columns) {
      expect(screen.getByTestId(testId)).toBeInTheDocument()
    }
    expect(screen.getByTestId('board-column-todo')).toHaveTextContent('(1)')
    expect(screen.getByTestId('board-column-done')).toHaveTextContent('(2)')
    expect(screen.queryByTestId('board-column-backlog')).not.toBeInTheDocument()
  })

  it('shows a legacy backlog-status task in the Open column (CR-REQ-018-06)', () => {
    render(<TaskBoardView tasks={[makeTask({ id: 'legacy', status: 'backlog' as never })]} onSelect={onSelect} />)
    expect(screen.getByTestId('board-column-open')).toHaveTextContent('(1)')
  })

  it('dragging a card from one column and dropping in another moves it immediately (optimistic), then calls task.update', async () => {
    // Deferred RPC — asserts the move happens BEFORE the RPC settles (truly
    // optimistic), not just after. Once it resolves, TaskBoardView drops its
    // local override and trusts task.status from a re-passed `tasks` prop
    // (the store update this triggers) instead of holding the move itself.
    let resolveRpc!: () => void
    mockRpc.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveRpc = resolve
        })
    )
    const tasks = [makeTask({ id: 't1', status: 'todo', title: 'Move me' })]
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)

    const transfer = new FakeDataTransfer()
    fireDrag('dragstart', screen.getByTestId('board-card-t1'), transfer)
    fireDrag('drop', screen.getByTestId('board-dropzone-done'), transfer)

    await waitFor(() => {
      expect(screen.getByTestId('board-column-done')).toContainElement(
        screen.getByTestId('board-card-t1')
      )
    })

    resolveRpc()
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.update', {
        taskId: 't1',
        patch: { status: 'done' }
      })
      expect(mockStore.updateTask).toHaveBeenCalledWith('t1', { status: 'done' })
    })
  })

  it('task.update rejected → card returns to its original column, toast.error shows the message', async () => {
    mockRpc.mockRejectedValueOnce(new Error('permission denied'))
    const tasks = [makeTask({ id: 't1', status: 'todo', title: 'Stuck task' })]
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)

    const transfer = new FakeDataTransfer()
    fireDrag('dragstart', screen.getByTestId('board-card-t1'), transfer)
    fireDrag('drop', screen.getByTestId('board-dropzone-done'), transfer)

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalledWith(
        'Cannot move "Stuck task" to done: permission denied'
      )
    })
    expect(screen.getByTestId('board-column-todo')).toContainElement(
      screen.getByTestId('board-card-t1')
    )
  })

  it('clicking a card calls onSelect(taskId)', () => {
    const tasks = [makeTask({ id: 't1', status: 'todo' })]
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)
    fireEvent.click(screen.getByText('Task'))
    expect(onSelect).toHaveBeenCalledWith('t1')
  })

  // BL-TG-06: TaskBoardView was the one view (Tree/DAG already had it)
  // missing selectMode/selectedIds/onToggleSelect passthrough to TaskCard —
  // confirmed by reading this file's props before this change.
  it('selectMode renders a checkbox per card, checked per selectedIds, and onToggleSelect fires without triggering onSelect', () => {
    const onToggleSelect = vi.fn()
    const tasks = [makeTask({ id: 't1', status: 'todo' }), makeTask({ id: 't2', status: 'todo' })]
    render(
      <TaskBoardView
        tasks={tasks}
        onSelect={onSelect}
        selectMode
        selectedIds={new Set(['t2'])}
        onToggleSelect={onToggleSelect}
      />
    )

    const cb1 = screen.getByTestId('task-select-t1') as HTMLInputElement
    const cb2 = screen.getByTestId('task-select-t2') as HTMLInputElement
    expect(cb1.checked).toBe(false)
    expect(cb2.checked).toBe(true)

    fireEvent.click(cb1)
    expect(onToggleSelect).toHaveBeenCalledWith('t1')
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('selectMode omitted → no checkboxes rendered (default behavior unchanged)', () => {
    const tasks = [makeTask({ id: 't1', status: 'todo' })]
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)
    expect(screen.queryByTestId('task-select-t1')).not.toBeInTheDocument()
  })

  describe('backlog hint (CR-REQ-023)', () => {
    it('has no backlog column', () => {
      render(<TaskBoardView tasks={[]} onSelect={onSelect} />)
      expect(screen.queryByTestId('board-column-backlog')).toBeNull()
    })

    it('shows the hint only when the request flow is supported', () => {
      const { unmount } = render(<TaskBoardView tasks={[]} onSelect={onSelect} />)
      expect(screen.queryByTestId('board-backlog-hint')).toBeNull()
      unmount()
      mockStore.requestFlowSupport = 'unsupported'
      const second = render(<TaskBoardView tasks={[]} onSelect={onSelect} />)
      expect(screen.queryByTestId('board-backlog-hint')).toBeNull()
      second.unmount()
      mockStore.requestFlowSupport = 'supported'
      render(<TaskBoardView tasks={[]} onSelect={onSelect} />)
      expect(screen.getByTestId('board-backlog-hint')).toBeInTheDocument()
    })

    it('opens the Requests > Backlog section from the hint button', () => {
      mockStore.requestFlowSupport = 'supported'
      render(<TaskBoardView tasks={[]} onSelect={onSelect} />)
      fireEvent.click(screen.getByRole('button', { name: 'Open backlog' }))
      expect(mockStore.setActiveView).toHaveBeenCalledWith('requests')
      expect(mockStore.setRequestPageData).toHaveBeenCalledWith({ section: 'backlog', focus: undefined })
    })
  })
})
