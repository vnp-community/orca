// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskBoardView } from '../TaskBoardView'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../../../shared/task-types'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

const updateTask = vi.fn()
const mockStore = {
  settings: {},
  tasks: [] as unknown[],
  updateTask
}

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (store: typeof mockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

const tasks = [
  {
    id: 't1',
    title: 'Task 1',
    status: 'todo',
    priority: 'medium',
    type: 'task',
    progressPercent: 0
  },
  { id: 't2', title: 'Task 2', status: 'done', priority: 'low', type: 'task', progressPercent: 100 }
] as unknown as OrcaTask[]

function dropOn(dropzoneTestId: string, taskId: string) {
  const dt = {
    data: {} as Record<string, string>,
    getData(key: string) {
      return this.data[key]
    },
    setData(key: string, value: string) {
      this.data[key] = value
    }
  }
  dt.setData('taskId', taskId)
  fireEvent.drop(screen.getByTestId(dropzoneTestId), { dataTransfer: dt })
}

describe('TaskBoardView', () => {
  const onSelect = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.tasks = tasks
  })

  afterEach(cleanup)

  it('render 7 cột đúng thứ tự STATUS_ORDER, mỗi cột đếm đúng số task', () => {
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)
    const order = ['backlog', 'todo', 'in_progress', 'blocked', 'review', 'done', 'cancelled']
    const columns = screen.getAllByTestId(/^board-column-/)
    expect(columns.map((c) => c.getAttribute('data-testid'))).toEqual(
      order.map((s) => `board-column-${s}`)
    )
    expect(screen.getByTestId('board-column-todo')).toHaveTextContent('(1)')
    expect(screen.getByTestId('board-column-done')).toHaveTextContent('(1)')
    expect(screen.getByTestId('board-column-backlog')).toHaveTextContent('(0)')
  })

  it('kéo card từ cột A thả vào cột B → card chuyển cột ngay (optimistic), rồi gọi task.update', async () => {
    // Resolve only after we've asserted the synchronous optimistic move below —
    // isolates "moves immediately" from "confirms via RPC" as 2 separate facts.
    let resolveRpc: (v: unknown) => void = () => {}
    mockRpc.mockReturnValueOnce(new Promise((resolve) => (resolveRpc = resolve)))
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)

    dropOn('board-dropzone-in_progress', 't1')

    // Optimistic: statusOverride applied synchronously, before the RPC resolves.
    expect(screen.getByTestId('board-column-in_progress')).toHaveTextContent('(1)')
    expect(screen.getByTestId('board-column-todo')).toHaveTextContent('(0)')

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.update', {
      taskId: 't1',
      patch: { status: 'in_progress' }
    })

    resolveRpc(undefined)
    await waitFor(() => {
      expect(updateTask).toHaveBeenCalledWith('t1', { status: 'in_progress' })
    })
  })

  it('task.update reject → card quay lại cột cũ, toast.error hiện đúng message', async () => {
    mockRpc.mockRejectedValueOnce(new Error('permission denied'))
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)

    dropOn('board-dropzone-in_progress', 't1')

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalled()
    })
    await waitFor(() => {
      // Rolled back — t1 still shows in its original 'todo' column.
      expect(screen.getByTestId('board-column-todo')).toHaveTextContent('(1)')
      expect(screen.getByTestId('board-column-in_progress')).toHaveTextContent('(0)')
    })
    expect(updateTask).not.toHaveBeenCalled()
  })

  it('click card → gọi onSelect(taskId)', () => {
    render(<TaskBoardView tasks={tasks} onSelect={onSelect} />)
    // TaskCard's onClick is on its inner row div, not the outer draggable wrapper —
    // click the task's title text (inside that row) to trigger it via bubbling.
    fireEvent.click(screen.getByText('Task 1'))
    expect(onSelect).toHaveBeenCalledWith('t1')
  })
})
