// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskDetail } from '../TaskDetail'
import { useTask } from '../../../hooks/useTask'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'

// Mock useAppStore for activeTaskId and settings
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) =>
      selector({
        activeTaskId: 't1',
        settings: {},
        tasks: [],
        templates: [],
        currentUser: { id: 'u1' }
      })
    ),
    { getState: () => ({ settings: {}, updateTask: vi.fn() }) }
  )
}))

// Mock useTask
vi.mock('../../../hooks/useTask', () => ({
  useTask: vi.fn()
}))

// Mock useWorkspace (real one requires <WorkspaceProvider>)
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: { id: 'p1' },
    currentWorktree: { id: 'wt-1', path: '/repo/p1', branch: 'main', isMain: true }
  })
}))

// Mock RPC
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

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// TaskDetail mounts several RPC-calling effects at once now (task.getDependencies,
// AttachWorkflowTemplateAction's workflow.template.list) — React fires child effects
// before a parent's own, so the call order between them isn't the source-code order.
// Route by method name instead of relying on mockResolvedValueOnce's call-position queue.
type RpcHandlers = Record<string, (params?: unknown) => Promise<unknown>>

function mockRpcByMethod(overrides: RpcHandlers = {}) {
  mockRpc.mockImplementation(((_target: unknown, method: string, params?: unknown) => {
    if (overrides[method]) {
      return overrides[method](params)
    }
    if (method === 'task.getDependencies') {
      return Promise.resolve([])
    }
    if (method === 'workflow.template.list') {
      return Promise.resolve({ templates: [] })
    }
    return Promise.resolve(undefined)
  }) as typeof callRuntimeRpc)
}

describe('TaskDetail', () => {
  const updateTask = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useTask).mockReturnValue({
      task: { id: 't1', title: 'My Task', status: 'todo', priority: 'high', projectId: 'p1' },
      updateTask
    } as unknown as ReturnType<typeof useTask>)
    mockRpcByMethod()
  })

  it('null task → renders empty state', () => {
    vi.mocked(useTask).mockReturnValue({ task: null, updateTask } as unknown as ReturnType<
      typeof useTask
    >)
    render(<TaskDetail />)
    expect(screen.getByText('Select a task')).toBeInTheDocument()
  })

  it('renders task title in input', () => {
    render(<TaskDetail />)
    expect(screen.getByTestId('task-title-input')).toHaveValue('My Task')
  })

  it('Execute with Agent button calls task.execute with taskId/projectId/worktreePath + traceId', async () => {
    render(<TaskDetail />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        'mock-target',
        'task.execute',
        expect.objectContaining({
          taskId: 't1',
          projectId: 'p1',
          worktreePath: '/repo/p1',
          traceId: expect.any(String)
        })
      )
    })
  })

  it('click run-agent-btn → Tracers.uiTaskGraphExecuteFlow.start({taskId, entryPoint: "task-detail"}), traceId forwarded to RPC', async () => {
    const { events, stop } = captureTraceEvents()
    render(<TaskDetail />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.execute', expect.any(Object))
    })
    stop()

    const flowEvents = events.filter((e) => e.flow === 'ui:taskGraph.execute')
    const startEvent = flowEvents.find((e) => e.level === 'start')
    expect(startEvent?.fields.taskId).toBe('t1')
    expect(startEvent?.fields.entryPoint).toBe('task-detail')

    const runAgentCall = mockRpc.mock.calls.find((c) => c[1] === 'task.execute')
    expect((runAgentCall?.[2] as { traceId?: string } | undefined)?.traceId).toBe(startEvent?.id)
  })

  it('RPC success → span.ok({taskId}), toast.success shown', async () => {
    const { events, stop } = captureTraceEvents()
    render(<TaskDetail />)

    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockToast.success).toHaveBeenCalledWith('Agent started for: My Task')
    })
    stop()

    const okEvent = events.find((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'ok')
    expect(okEvent?.fields.taskId).toBe('t1')
  })

  it('RPC error → span.fail(err, {taskId}), toast.error shown', async () => {
    const err = new Error('agent spawn failed')
    mockRpcByMethod({ 'task.execute': () => Promise.reject(err) })
    render(<TaskDetail />)

    const { events, stop } = captureTraceEvents()
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalledWith('Failed to start agent: agent spawn failed')
    })
    stop()

    const failEvents = events.filter((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.taskId).toBe('t1')
  })

  it('renders ExecutionEngineBadge next to the Run button', () => {
    render(<TaskDetail />)
    expect(screen.getByTestId('execution-engine-badge')).toHaveTextContent('Direct Agent')
  })

  it('dependencies section renders blocked-by list', async () => {
    // task.getDependencies returns a flat Task[] (this task's own "depends on" edges) —
    // see TaskDetail.tsx's own comment on the deps effect.
    mockRpcByMethod({
      'task.getDependencies': () => Promise.resolve([{ id: 'b1', title: 'Blocker 1' }])
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.getByText('Blocker 1')).toBeInTheDocument()
      expect(screen.getByText('← Blocked by:')).toBeInTheDocument()
    })
  })

  it('renders AttachWorkflowTemplateAction next to the Run button, populated from workflow.template.list', async () => {
    mockRpcByMethod({
      'workflow.template.list': () =>
        Promise.resolve({ templates: [{ id: 'wt1', name: 'Template One' }] })
    })
    render(<TaskDetail />)
    expect(screen.getByTestId('attach-workflow-template-trigger')).toBeInTheDocument()
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        'mock-target',
        'workflow.template.list',
        expect.any(Object)
      )
    })
  })

  // FE-TASK-003: useTaskActivity polls task.get and this replaces the previous
  // "fire handleRunAgent then watch nothing" silent gap — status now re-renders without F5.
  it('useTaskActivity polling → polled task.get status displays without needing F5', async () => {
    mockRpcByMethod({
      'task.get': () => Promise.resolve({ id: 't1', status: 'done' })
    })
    render(<TaskDetail />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.get', { id: 't1' })
    })
    await waitFor(() => {
      expect(screen.getByTestId('task-live-status')).toHaveTextContent('Status: done')
    })
  })

  it('before any poll resolves, live status falls back to task.status', () => {
    mockRpcByMethod({ 'task.get': () => new Promise(() => {}) }) // never resolves
    render(<TaskDetail />)
    expect(screen.getByTestId('task-live-status')).toHaveTextContent('Status: todo')
  })

  // useTaskActivity's polled status also drives the Details tab's TaskStatusBadge (icon +
  // label), not just the raw "Status: x" text above — confirms TaskDetail prefers
  // polledTask.status over the (stale) task.status prop there too.
  it("useTaskActivity's polled status renders via TaskStatusBadge (icon/label), not raw text", async () => {
    mockRpcByMethod({
      'task.get': () => Promise.resolve({ id: 't1', status: 'in_progress' })
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.getByText('In Progress')).toBeInTheDocument()
      expect(screen.getByText('🔄')).toBeInTheDocument()
    })
  })

  // TASK-FE-TASKV1-10: dispatch status panel sits under the Execute button.
  it('renders TaskDispatchStatusPanel, calling orchestration.dispatchShow for this task', async () => {
    mockRpcByMethod({
      'orchestration.dispatchShow': () =>
        Promise.resolve({
          dispatch: { id: 'd1', orchestration_task_id: 't1', assignee_handle: '', status: 'queued' }
        })
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'orchestration.dispatchShow', {
        task: 't1'
      })
    })
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-status')).toBeInTheDocument()
    })
  })

  // TASK-FE-TASKV1-08: Comments tab is always present; on backend-go/today's Node (no
  // task.listComments read RPC anywhere) it shows the "unavailable" message.
  it('Comments tab renders TaskComments, showing "unavailable" when task.listComments is unsupported', async () => {
    render(<TaskDetail />)
    // Radix TabsTrigger switches tabs on mousedown (not click) — see
    // @radix-ui/react-tabs's TabsTrigger onMouseDown handler.
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Comments' }))
    await waitFor(() => {
      expect(screen.getByTestId('task-comments-unsupported')).toBeInTheDocument()
    })
  })

  // TASK-FE-TASKV1-06: task.resolvePermission isn't wired at backend-go today, so it
  // rejects for every call — isSupported flips to false and Execute must stay visible.
  it('Execute with Agent stays visible when task.resolvePermission is not wired (isSupported === false)', async () => {
    render(<TaskDetail />)
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.resolvePermission', {
        taskId: 't1',
        userId: 'u1'
      })
    })
    expect(screen.getByTestId('run-agent-btn')).toBeInTheDocument()
  })

  it('Execute with Agent is hidden once permission resolves to a level below "user" (e.g. "team")', async () => {
    mockRpcByMethod({
      'task.resolvePermission': () => Promise.resolve({ effectiveLevel: 'GRANT_LEVEL_TEAM' })
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.queryByTestId('run-agent-btn')).not.toBeInTheDocument()
    })
  })

  it('Execute with Agent stays visible when permission resolves to "user" or above', async () => {
    mockRpcByMethod({
      'task.resolvePermission': () => Promise.resolve({ effectiveLevel: 'GRANT_LEVEL_USER' })
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.getByTestId('run-agent-btn')).toBeInTheDocument()
    })
  })

  it('Access tab renders TaskAccessPanel — unsupported message by default (RPC not wired)', async () => {
    render(<TaskDetail />)
    // Radix TabsTrigger switches tabs on mousedown (not click) — see
    // @radix-ui/react-tabs's TabsTrigger onMouseDown handler.
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'Access' }))
    await waitFor(() => {
      expect(screen.getByTestId('task-access-unsupported')).toBeInTheDocument()
    })
  })
})
