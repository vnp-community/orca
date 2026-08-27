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
    vi.fn((selector) => selector({ activeTaskId: 't1', settings: {} })),
    { getState: () => ({ settings: {} }) }
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

describe('TaskDetail', () => {
  const updateTask = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useTask).mockReturnValue({
      task: { id: 't1', title: 'My Task', status: 'todo', priority: 'high', projectId: 'p1' },
      updateTask
    } as any)
    mockRpc.mockResolvedValue([]) // task.getDependencies returns a flat { task, edgeType }[]
  })

  it('null task → renders empty state', () => {
    vi.mocked(useTask).mockReturnValue({ task: null, updateTask } as any)
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
    expect((runAgentCall?.[2] as { traceId?: string }).traceId).toBe(startEvent?.id)
  })

  it('RPC success → span.ok({taskId}), toast.success shown', async () => {
    mockRpc.mockResolvedValueOnce([]) // task.getDependencies (mount)
    const { events, stop } = captureTraceEvents()
    render(<TaskDetail />)

    mockRpc.mockResolvedValueOnce(undefined) // task.execute
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
    render(<TaskDetail />)

    mockRpc.mockRejectedValueOnce(err) // task.execute
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

  it('dependencies section renders blocked-by list', async () => {
    mockRpc.mockResolvedValueOnce([
      { task: { id: 'b1', title: 'Blocker 1' }, edgeType: 'depends_on' }
    ])
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.getByText('Blocker 1')).toBeInTheDocument()
      expect(screen.getByText('← Blocked by:')).toBeInTheDocument()
    })
  })
})
