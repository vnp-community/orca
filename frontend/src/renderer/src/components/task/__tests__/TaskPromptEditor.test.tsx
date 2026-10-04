// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskPromptEditor } from '../TaskPromptEditor'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'
import type { OrcaTask } from '../../../../../shared/task-types'

// Mock useAppStore for settings
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

// Mock useWorkspace (real one requires <WorkspaceProvider>)
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: { id: 'proj-1' },
    currentWorktree: { id: 'wt-1', path: '/repo/proj-1', branch: 'main', isMain: true }
  })
}))

import { useWorkspace } from '../../../context/WorkspaceContext'

// Mock RPC
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

const mockUpdateTask = vi.fn()
vi.mock('../../../hooks/useTask', () => ({
  useTask: () => ({ updateTask: mockUpdateTask })
}))

const mockAddComment = vi.fn()
vi.mock('../../../hooks/useTaskComments', () => ({
  useTaskComments: () => ({ comments: [], addComment: mockAddComment, isSupported: true })
}))

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

const task: OrcaTask = {
  id: 't1',
  title: 'My Task',
  status: 'todo',
  priority: 'high',
  projectId: 'proj-1',
  labels: [] as string[],
  promptTemplate: 'do the thing'
} as OrcaTask

describe('TaskPromptEditor.runWithAgent() tracing', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockRpc.mockResolvedValue(undefined)
  })

  it('click run-agent-btn → Tracers.uiTaskGraphExecuteFlow.start({taskId, entryPoint: "prompt-editor", promptLength})', async () => {
    const { events, stop } = captureTraceEvents()
    render(<TaskPromptEditor task={task} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalled()
    })
    stop()

    const startEvent = events.find((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'start')
    expect(startEvent?.fields.taskId).toBe('t1')
    expect(startEvent?.fields.entryPoint).toBe('prompt-editor')
    expect(startEvent?.fields.promptLength).toBe('do the thing'.length)
  })

  it('task.execute RPC receives taskId/projectId/prompt + traceId === span.id (BACKLOG-016), never worktreePath (dead param, crashed with no worktree selected)', async () => {
    const { events, stop } = captureTraceEvents()
    render(<TaskPromptEditor task={task} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        'mock-target',
        'task.execute',
        expect.objectContaining({
          taskId: 't1',
          projectId: 'proj-1',
          prompt: 'do the thing'
        })
      )
    })
    expect(mockRpc.mock.calls[0]?.[2]).not.toHaveProperty('worktreePath')
    stop()

    const startEvent = events.find((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'start')
    const callArgs = mockRpc.mock.calls[0]
    expect(callArgs?.[1]).toBe('task.execute')
    expect((callArgs?.[2] as { traceId?: string } | undefined)?.traceId).toBe(startEvent?.id)
  })

  it('RPC success → span.ok({taskId})', async () => {
    const { events, stop } = captureTraceEvents()
    render(<TaskPromptEditor task={task} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalled()
    })
    stop()

    const okEvent = events.find((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'ok')
    expect(okEvent?.fields.taskId).toBe('t1')
  })

  it('RPC error → span.fail(err, {taskId}) before re-throw, isRunning resets', async () => {
    const err = new Error('agent spawn failed')
    mockRpc.mockRejectedValueOnce(err)
    const { events, stop } = captureTraceEvents()

    // `runWithAgent` re-throws after span.fail() (matches TASK-FE-018.3 spec) but the
    // <Button onClick> call site is fire-and-forget (pre-existing, not this task's scope)
    // → swallow the resulting unhandled rejection so it doesn't fail the test run.
    const onUnhandled = (reason: unknown) => {
      void reason
    }
    process.on('unhandledRejection', onUnhandled)

    render(<TaskPromptEditor task={task} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      const failEvents = events.filter(
        (e) => e.flow === 'ui:taskGraph.execute' && e.level === 'fail'
      )
      expect(failEvents).toHaveLength(1)
    })
    stop()
    process.off('unhandledRejection', onUnhandled)

    const failEvent = events.find((e) => e.flow === 'ui:taskGraph.execute' && e.level === 'fail')
    expect(failEvent?.fields.taskId).toBe('t1')
    expect(failEvent?.fields.err).toContain('agent spawn failed')

    // isRunning reset back to false in `finally` → button re-enabled (not stuck "Running...")
    await waitFor(() => {
      expect(screen.getByTestId('run-agent-btn')).not.toHaveTextContent('Running...')
    })
  })

  it('Run button is NOT disabled when the prompt textarea is empty (content was never actually sent before this task)', () => {
    render(<TaskPromptEditor task={{ ...task, promptTemplate: undefined }} />)
    const textarea = screen.getByPlaceholderText(
      'Describe what the agent should do for this task...'
    )
    expect(textarea).toHaveValue('')
    expect(screen.getByTestId('run-agent-btn')).not.toBeDisabled()
  })

  it('runWithAgent sends `prompt` (textarea value, or undefined when empty) in the task.execute payload', async () => {
    render(<TaskPromptEditor task={{ ...task, promptTemplate: undefined }} />)
    const textarea = screen.getByPlaceholderText(
      'Describe what the agent should do for this task...'
    )
    fireEvent.change(textarea, { target: { value: 'do the extra thing' } })
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        'mock-target',
        'task.execute',
        expect.objectContaining({ prompt: 'do the extra thing' })
      )
    })
  })

  it('runWithAgent sends prompt: undefined when the textarea is empty', async () => {
    render(<TaskPromptEditor task={{ ...task, promptTemplate: undefined }} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith(
        'mock-target',
        'task.execute',
        expect.objectContaining({ prompt: undefined })
      )
    })
  })
})

describe('BL-TG-05 spec/build loop', () => {
  const specTask: OrcaTask = { ...task, taskNumber: 42, description: 'do the extra thing' }

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockRpc.mockResolvedValue(undefined)
    mockUpdateTask.mockResolvedValue(undefined)
    mockAddComment.mockResolvedValue(undefined)
  })

  it('Generate Spec fills the textarea with the spec prompt referencing the deterministic file path', () => {
    render(<TaskPromptEditor task={specTask} />)
    fireEvent.click(screen.getByTestId('generate-spec-btn'))

    const textarea = screen.getByPlaceholderText(
      'Describe what the agent should do for this task...'
    ) as HTMLTextAreaElement
    expect(textarea.value).toContain('specs/generated/TASK-42-spec.md')
    expect(textarea.value).toContain('Do NOT write implementation code yet')
  })

  it('Implement Spec is disabled until the spec is approved', () => {
    render(<TaskPromptEditor task={specTask} />)
    expect(screen.getByTestId('implement-spec-btn')).toBeDisabled()
    cleanup()

    render(<TaskPromptEditor task={{ ...specTask, labels: ['phase:spec-approved'] }} />)
    expect(screen.getByTestId('implement-spec-btn')).not.toBeDisabled()
  })

  it('clicking Generate Spec then Run with Agent sets phase:spec-pending BEFORE task.execute', async () => {
    render(<TaskPromptEditor task={specTask} />)
    fireEvent.click(screen.getByTestId('generate-spec-btn'))
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => expect(mockRpc).toHaveBeenCalled())
    expect(mockUpdateTask).toHaveBeenCalledWith({ labels: ['phase:spec-pending'] })
    const updateOrder = mockUpdateTask.mock.invocationCallOrder[0]
    const executeOrder = mockRpc.mock.invocationCallOrder[0]
    expect(updateOrder).toBeLessThan(executeOrder)
  })

  it('a plain custom-prompt run (no preset clicked) never touches labels', async () => {
    render(<TaskPromptEditor task={specTask} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => expect(mockRpc).toHaveBeenCalled())
    expect(mockUpdateTask).not.toHaveBeenCalled()
  })

  it('shows the spec review panel only when phase is spec-pending AND status is review', () => {
    render(<TaskPromptEditor task={{ ...specTask, labels: ['phase:spec-pending'] }} />)
    expect(screen.queryByTestId('spec-review-panel')).not.toBeInTheDocument()
    cleanup()

    render(
      <TaskPromptEditor task={{ ...specTask, status: 'review', labels: ['phase:spec-pending'] }} />
    )
    expect(screen.getByTestId('spec-review-panel')).toBeInTheDocument()
  })

  it('Approve Spec replaces phase:spec-pending with phase:spec-approved', async () => {
    render(
      <TaskPromptEditor task={{ ...specTask, status: 'review', labels: ['phase:spec-pending'] }} />
    )
    fireEvent.click(screen.getByTestId('approve-spec-btn'))

    await waitFor(() =>
      expect(mockUpdateTask).toHaveBeenCalledWith({ labels: ['phase:spec-approved'] })
    )
  })

  it('Request Changes posts a comment and does not change status/labels', async () => {
    render(
      <TaskPromptEditor task={{ ...specTask, status: 'review', labels: ['phase:spec-pending'] }} />
    )
    const textarea = screen.getByPlaceholderText('What needs to change?')
    fireEvent.change(textarea, { target: { value: 'please add error handling' } })
    fireEvent.click(screen.getByTestId('request-changes-btn'))

    await waitFor(() => expect(mockAddComment).toHaveBeenCalledWith('please add error handling'))
    expect(mockUpdateTask).not.toHaveBeenCalled()
  })

  it('shows the code review panel when phase is code-pending AND status is review', () => {
    render(
      <TaskPromptEditor task={{ ...specTask, status: 'review', labels: ['phase:code-pending'] }} />
    )
    expect(screen.getByTestId('code-review-panel')).toBeInTheDocument()
  })

  it('Approve & Mark Done sets status done and clears the phase label', async () => {
    render(
      <TaskPromptEditor task={{ ...specTask, status: 'review', labels: ['phase:code-pending'] }} />
    )
    fireEvent.click(screen.getByTestId('approve-code-btn'))

    await waitFor(() => expect(mockUpdateTask).toHaveBeenCalledWith({ status: 'done', labels: [] }))
  })

  it('runWithAgent does not crash when no worktree is selected in the sidebar (found live: TypeError reading .path on null)', async () => {
    vi.mocked(useWorkspace).mockReturnValueOnce({
      project: { id: 'proj-1' },
      currentWorktree: null
    } as unknown as ReturnType<typeof useWorkspace>)
    render(<TaskPromptEditor task={specTask} />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))

    await waitFor(() => expect(mockRpc).toHaveBeenCalled())
    expect(mockRpc.mock.calls[0]?.[2]).not.toHaveProperty('worktreePath')
  })

  it('withPhase never drops a non-phase label the user set independently', async () => {
    render(
      <TaskPromptEditor
        task={{ ...specTask, status: 'review', labels: ['priority:urgent', 'phase:spec-pending'] }}
      />
    )
    fireEvent.click(screen.getByTestId('approve-spec-btn'))

    await waitFor(() =>
      expect(mockUpdateTask).toHaveBeenCalledWith({
        labels: ['priority:urgent', 'phase:spec-approved']
      })
    )
  })

  // BUG-027 follow-up: task.execute now dispatches async (returns before the
  // agent finishes) — local `isRunning` alone resets right after that
  // now-fast RPC resolves, no longer reflecting whether the agent is still
  // actually working. The caller (TaskDetail) is expected to pass its
  // freshest known task (polled, once available), so task.status ===
  // 'in_progress' must independently keep every dispatch button disabled.
  it('BUG-027: task.status === in_progress disables Run/Generate Spec/Implement Spec regardless of local isRunning', () => {
    render(<TaskPromptEditor task={{ ...task, status: 'in_progress' }} />)
    expect(screen.getByTestId('run-agent-btn')).toBeDisabled()
    expect(screen.getByTestId('run-agent-btn')).toHaveTextContent('Running...')
    expect(screen.getByTestId('generate-spec-btn')).toBeDisabled()
  })
})
