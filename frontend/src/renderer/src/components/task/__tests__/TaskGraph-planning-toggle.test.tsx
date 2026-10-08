// @vitest-environment happy-dom
// FE-REQ-TASK-021-06 (b): the Board hides Plan/Phase by default, the toggle shows them, and the
// working Tasks stay visible at the root. Real useTasks; only the runtime RPC and heavy views are stubbed.
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { OrcaTask } from '../../../../../shared/task-types'

const rpc = vi.hoisted(() => ({ tasks: [] as unknown[] }))
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(async () => ({ tasks: rpc.tasks })),
  getActiveRuntimeTarget: () => ({ kind: 'local' })
}))
vi.mock('../../../hooks/useTaskDependencyEdges', () => ({
  useTaskDependencyEdges: () => ({
    edges: new Map(),
    loading: false,
    error: false,
    refetch: vi.fn()
  })
}))
vi.mock('../../../hooks/useTaskBatchExecution', () => ({
  useTaskBatchExecution: () => ({ running: false, results: new Map(), runSelected: vi.fn() })
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('../TaskTreeView', () => ({ TaskTreeView: () => <div data-testid="mock-tree-view" /> }))
vi.mock('../TaskDAGView', () => ({ default: () => <div data-testid="mock-dag-view" /> }))
vi.mock('../TaskBoardView', () => ({
  TaskBoardView: ({ tasks }: { tasks: OrcaTask[] }) => (
    <ul data-testid="board">
      {tasks.map((t) => (
        <li key={t.id} data-parent={t.parentId ?? 'root'}>
          {t.title}
        </li>
      ))}
    </ul>
  )
}))

import { useAppStore } from '@/store'
import { TaskGraph } from '../TaskGraph'

const mk = (id: string, o: Partial<OrcaTask> = {}): OrcaTask => ({
  id,
  projectId: 'p1',
  title: id,
  type: 'task',
  status: 'todo',
  priority: 'medium',
  labels: [],
  visibility: 'private',
  progressPercent: 0,
  createdAt: new Date(),
  updatedAt: new Date(),
  ...o
})

afterEach(cleanup)

describe('TaskGraph Board with Plan/Phase', () => {
  it('hides Plan/Phase by default, shows them on toggle, keeps child tasks visible', async () => {
    rpc.tasks = [
      mk('Plan A', { type: 'plan' }),
      mk('Phase 1', { type: 'phase', parentId: 'Plan A' }),
      mk('Write tests', { parentId: 'Phase 1' })
    ]
    useAppStore.setState({ tasks: rpc.tasks as OrcaTask[] })
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Board' }))
    const board = await screen.findByTestId('board')
    await waitFor(() => expect(board).toHaveTextContent('Write tests'))
    expect(board).not.toHaveTextContent('Plan A')
    expect(board).not.toHaveTextContent('Phase 1')
    expect(screen.getByText('Write tests')).toHaveAttribute('data-parent', 'root')

    fireEvent.click(screen.getByRole('checkbox'))
    await waitFor(() => expect(screen.getByTestId('board')).toHaveTextContent('Plan A'))
    expect(screen.getByTestId('board')).toHaveTextContent('Phase 1')
    expect(screen.getByText('Write tests')).toHaveAttribute('data-parent', 'Phase 1')
  })

  it('has no toggle when the project has no Plan/Phase', async () => {
    rpc.tasks = [mk('Solo')]
    useAppStore.setState({ tasks: rpc.tasks as OrcaTask[] })
    render(<TaskGraph projectId="p1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Board' }))
    await waitFor(() => expect(screen.getByTestId('board')).toHaveTextContent('Solo'))
    expect(screen.queryByTestId('toggle-show-planning')).toBeNull()
  })
})
