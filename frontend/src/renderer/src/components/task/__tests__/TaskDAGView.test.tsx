// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import type { Node, Edge } from '@xyflow/react'
import { TaskDAGView } from '../TaskDAGView'
import type { TaskEdgeMap } from '../../../hooks/useTaskDependencyEdges'
import type { OrcaTask } from '../../../../../shared/task-types'

// Mock ReactFlow since it requires a real DOM size and ResizeObserver to render properly —
// same approach as DAGPreview.test.tsx. Exposes onConnect so tests can simulate a drag-connect.
let capturedOnConnect: ((c: { source: string | null; target: string | null }) => void) | null = null

type MockReactFlowProps = {
  nodes: Node[]
  edges: Edge[]
  onNodeClick: (event: unknown, node: Node) => void
  onConnect: (c: { source: string | null; target: string | null }) => void
}

vi.mock('@xyflow/react', () => ({
  ReactFlow: ({ nodes, edges, onNodeClick, onConnect }: MockReactFlowProps) => {
    capturedOnConnect = onConnect
    return (
      <div data-testid="mock-react-flow">
        <div data-testid="nodes-count">{nodes.length}</div>
        <div data-testid="edges-count">{edges.length}</div>
        <div data-testid="edges-json">
          {JSON.stringify(edges.map((e: Edge) => ({ source: e.source, target: e.target })))}
        </div>
        {nodes.map((n: Node) => (
          <button key={n.id} data-testid={`mock-node-${n.id}`} onClick={() => onNodeClick(null, n)}>
            {n.id}
          </button>
        ))}
      </div>
    )
  },
  Background: () => <div />,
  Controls: () => <div />,
  MiniMap: () => <div />
}))

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() }
}))
import { toast } from 'sonner'
const mockToast = vi.mocked(toast)

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

function makeTask(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    title: `Task ${id}`,
    type: 'task',
    status: 'todo',
    priority: 'medium',
    progressPercent: 0,
    ...overrides
  } as OrcaTask
}

describe('TaskDAGView', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    capturedOnConnect = null
    // happy-dom doesn't implement window.confirm — stub it before spying.
    window.confirm = vi.fn().mockReturnValue(true)
  })

  it('empty tasks → shows empty state', () => {
    render(<TaskDAGView tasks={[]} dependencyEdges={new Map()} onSelect={vi.fn()} />)
    expect(screen.getByTestId('task-dag-empty')).toBeInTheDocument()
  })

  it('builds an edge from dependencyEdges.blockedBy (real data, not a fake `dependsOn` field)', () => {
    const tasks = [makeTask('a'), makeTask('b')]
    const dependencyEdges: TaskEdgeMap = new Map([
      ['b', { blockedBy: ['a'], blocks: [] }],
      ['a', { blockedBy: [], blocks: ['b'] }]
    ])
    render(<TaskDAGView tasks={tasks} dependencyEdges={dependencyEdges} onSelect={vi.fn()} />)

    expect(screen.getByTestId('edges-count')).toHaveTextContent('1')
    const edges = JSON.parse(screen.getByTestId('edges-json').textContent!)
    expect(edges).toEqual([{ source: 'a', target: 'b' }])
  })

  it('no dependencyEdges entry for a task → no edges built (not a crash)', () => {
    const tasks = [makeTask('a'), makeTask('b')]
    render(<TaskDAGView tasks={tasks} dependencyEdges={new Map()} onSelect={vi.fn()} />)
    expect(screen.getByTestId('edges-count')).toHaveTextContent('0')
  })

  it('clicking a node calls onSelect(taskId)', () => {
    const onSelect = vi.fn()
    render(<TaskDAGView tasks={[makeTask('a')]} dependencyEdges={new Map()} onSelect={onSelect} />)
    fireEvent.click(screen.getByTestId('mock-node-a'))
    expect(onSelect).toHaveBeenCalledWith('a')
  })

  it('onConnect: confirms, calls task.addEdge with fromTaskId/toTaskId/type, then onEdgeAdded()', async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const onEdgeAdded = vi.fn()
    const tasks = [makeTask('a', { title: 'Source' }), makeTask('b', { title: 'Target' })]
    render(
      <TaskDAGView
        tasks={tasks}
        dependencyEdges={new Map()}
        onSelect={vi.fn()}
        onEdgeAdded={onEdgeAdded}
      />
    )

    await capturedOnConnect?.({ source: 'a', target: 'b' })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.addEdge', {
        fromTaskId: 'a',
        toTaskId: 'b',
        type: 'EDGE_TYPE_DEPENDS_ON'
      })
      expect(onEdgeAdded).toHaveBeenCalled()
    })
  })

  it('onConnect: user cancels confirm → no RPC call', async () => {
    window.confirm = vi.fn().mockReturnValue(false)
    render(
      <TaskDAGView
        tasks={[makeTask('a'), makeTask('b')]}
        dependencyEdges={new Map()}
        onSelect={vi.fn()}
      />
    )
    await capturedOnConnect?.({ source: 'a', target: 'b' })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('onConnect: RPC failure (e.g. "method not found" until backend wires task.addEdge) shows toast.error, does not throw', async () => {
    const err = new Error('method not found: task.addEdge')
    mockRpc.mockRejectedValueOnce(err)
    render(
      <TaskDAGView
        tasks={[makeTask('a'), makeTask('b')]}
        dependencyEdges={new Map()}
        onSelect={vi.fn()}
      />
    )
    await capturedOnConnect?.({ source: 'a', target: 'b' })

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalledWith(
        'Could not add dependency: method not found: task.addEdge'
      )
    })
  })

  it('onConnect: missing source or target → no-op', async () => {
    render(<TaskDAGView tasks={[makeTask('a')]} dependencyEdges={new Map()} onSelect={vi.fn()} />)
    await capturedOnConnect?.({ source: null, target: 'a' })
    expect(mockRpc).not.toHaveBeenCalled()
  })

  it('selecting "from" then "to" in the add-dependency dropdowns calls task.addEdge', async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const onEdgeAdded = vi.fn()
    const tasks = [makeTask('a', { title: 'A' }), makeTask('b', { title: 'B' })]

    render(
      <TaskDAGView
        tasks={tasks}
        dependencyEdges={new Map()}
        onSelect={vi.fn()}
        onEdgeAdded={onEdgeAdded}
      />
    )

    fireEvent.change(screen.getByTestId('dag-add-dependency-select-from'), {
      target: { value: 'a' }
    })
    fireEvent.change(screen.getByTestId('dag-add-dependency-select-to'), {
      target: { value: 'b' }
    })

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.addEdge', {
        fromTaskId: 'a',
        toTaskId: 'b',
        type: 'depends_on'
      })
      expect(onEdgeAdded).toHaveBeenCalled()
    })
  })

  it('dropdown task.addEdge failure does not crash and leaves addingFor set (select-to stays visible)', async () => {
    mockRpc.mockRejectedValueOnce(new Error('cycle detected'))
    const tasks = [makeTask('a', { title: 'A' }), makeTask('b', { title: 'B' })]

    render(<TaskDAGView tasks={tasks} dependencyEdges={new Map()} onSelect={vi.fn()} />)

    fireEvent.change(screen.getByTestId('dag-add-dependency-select-from'), {
      target: { value: 'a' }
    })
    fireEvent.change(screen.getByTestId('dag-add-dependency-select-to'), {
      target: { value: 'b' }
    })

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalledWith('Failed to add dependency: cycle detected')
    })
    // addingFor was not reset on failure — the "to" dropdown (rendered only while addingFor
    // is set) is still present, letting the user retry.
    expect(screen.getByTestId('dag-add-dependency-select-to')).toBeInTheDocument()
  })
})
