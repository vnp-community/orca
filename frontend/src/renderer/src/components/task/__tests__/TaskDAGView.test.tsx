// @vitest-environment happy-dom
// FE-TASK-002 (task-graph v4): TaskDAGView.test.tsx did not exist before this task — created new
// (the task file said MODIFY, but no such file was ever present in this worktree).
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskDAGView } from '../TaskDAGView'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../../../shared/task-types'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (state: Record<string, unknown>) => unknown) =>
      fn ? fn({ settings: {} }) : { settings: {} },
    {
      getState: () => ({ settings: {} })
    }
  )
}))

// Mock ReactFlow — same pattern as DAGPreview.test.tsx (requires real DOM size/ResizeObserver).
type TestReactFlowProps = { nodes: unknown[]; edges: unknown[] }
// Shape mockRpc's callers actually read off callRuntimeRpc's `params: unknown` arg.
type TestRpcParams = { taskId?: string }

vi.mock('@xyflow/react', () => ({
  ReactFlow: ({ nodes, edges }: TestReactFlowProps) => (
    <div data-testid="mock-react-flow">
      <div data-testid="nodes-count">{nodes.length}</div>
      <div data-testid="edges-count">{edges.length}</div>
    </div>
  ),
  Background: () => <div />,
  Controls: () => <div />,
  MiniMap: () => <div />
}))

const mockRpc = vi.mocked(callRuntimeRpc)

const taskA = { id: 'a', title: 'Task A', status: 'todo', type: 'task' } as unknown as OrcaTask
const taskB = { id: 'b', title: 'Task B', status: 'todo', type: 'task' } as unknown as OrcaTask

describe('TaskDAGView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(cleanup)

  it('2 tasks, task.getDependencies trả 1 cạnh depends_on → edges render đúng 1 cạnh', async () => {
    mockRpc.mockImplementation((_target, _method, params) => {
      if ((params as TestRpcParams).taskId === 'b') {
        return Promise.resolve([{ task: taskA, edgeType: 'depends_on' }])
      }
      return Promise.resolve([])
    })
    render(<TaskDAGView tasks={[taskA, taskB]} onSelect={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByTestId('edges-count')).toHaveTextContent('1')
    })
    expect(screen.getByTestId('nodes-count')).toHaveTextContent('2')
  })

  it('task.getDependencies lỗi cho 1 task → DAG vẫn render nodes, không crash', async () => {
    mockRpc.mockImplementation((_target, _method, params) => {
      if ((params as TestRpcParams).taskId === 'b') {
        return Promise.reject(new Error('boom'))
      }
      return Promise.resolve([])
    })
    render(<TaskDAGView tasks={[taskA, taskB]} onSelect={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByTestId('nodes-count')).toHaveTextContent('2')
    })
    // Promise.all fails the whole batch on any single rejection — depsById stays at its
    // previous (empty, on first load) value, so no edges render for either task. This is
    // the real behavior of the batched fetch, not a partial-failure per task.
    expect(screen.getByTestId('edges-count')).toHaveTextContent('0')
  })

  it('chọn "from" rồi "to" trong dropdown add-dependency → gọi task.addEdge', async () => {
    mockRpc.mockResolvedValue([])
    render(<TaskDAGView tasks={[taskA, taskB]} onSelect={vi.fn()} />)
    await waitFor(() => screen.getByTestId('task-dag-view'))

    fireEvent.change(screen.getByTestId('dag-add-dependency-select-from'), {
      target: { value: 'b' }
    })
    await waitFor(() => screen.getByTestId('dag-add-dependency-select-to'))
    fireEvent.change(screen.getByTestId('dag-add-dependency-select-to'), {
      target: { value: 'a' }
    })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.addEdge', {
        fromTaskId: 'b',
        toTaskId: 'a',
        type: 'depends_on'
      })
    })
  })

  it('task.addEdge lỗi → không crash, addingFor không reset (dropdown "to" vẫn hiện)', async () => {
    mockRpc.mockImplementation((_target, method: string) => {
      if (method === 'task.addEdge') {
        return Promise.reject(new Error('addEdge boom'))
      }
      return Promise.resolve([])
    })
    render(<TaskDAGView tasks={[taskA, taskB]} onSelect={vi.fn()} />)
    await waitFor(() => screen.getByTestId('task-dag-view'))

    fireEvent.change(screen.getByTestId('dag-add-dependency-select-from'), {
      target: { value: 'b' }
    })
    await waitFor(() => screen.getByTestId('dag-add-dependency-select-to'))
    fireEvent.change(screen.getByTestId('dag-add-dependency-select-to'), {
      target: { value: 'a' }
    })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.addEdge', expect.anything())
    })
    // addingFor unchanged on failure — the "to" dropdown should still be present.
    expect(screen.getByTestId('dag-add-dependency-select-to')).toBeInTheDocument()
  })

  it('tasks=[] → shows empty state', () => {
    render(<TaskDAGView tasks={[]} onSelect={vi.fn()} />)
    expect(screen.getByTestId('task-dag-empty')).toBeInTheDocument()
  })
})
