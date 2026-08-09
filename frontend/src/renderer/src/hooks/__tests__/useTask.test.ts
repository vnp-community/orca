/**
 * Tests for `useTask().aiDecompose()` tracing (TASK-FE-018.1, BL-TG-02).
 *
 * Covers `src/renderer/src/hooks/useTask.ts` — instrumented with
 * `Tracers.uiTaskGraphAiPlanFlow` (`ui:taskGraph.aiPlan`), forwarding
 * `traceId` to `tasks.aiPlan` and logging `promptLength` instead of the
 * full instruction text.
 *
 * @module renderer/hooks/__tests__/useTask.test
 */
// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { registerTraceSink, type TraceEvent } from '../../../../shared/trace'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore: any = {
  tasks: [{ id: 'task-1', title: 'Task 1' }],
  updateTask: vi.fn(),
  removeTask: vi.fn(),
  addTask: vi.fn(),
  settings: {}
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: any) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('useTask().aiDecompose() tracing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('with instruction → start({hasInstruction:true, promptLength}), traceId forwarded to tasks.aiPlan', async () => {
    mockRpc.mockResolvedValueOnce({ subtasks: [{ title: 'a' }, { title: 'b' }] })
    const { events, stop } = captureTraceEvents()
    const { useTask } = await import('../useTask')
    const { result } = renderHook(() => useTask('task-1'))

    await act(async () => {
      await result.current.aiDecompose('do the thing')
    })
    stop()

    const flowEvents = events.filter((e) => e.flow === 'ui:taskGraph.aiPlan')
    const startEvent = flowEvents.find((e) => e.level === 'start')
    expect(startEvent?.fields.hasInstruction).toBe(true)
    expect(startEvent?.fields.promptLength).toBe('do the thing'.length)

    const callArgs = mockRpc.mock.calls[0]
    expect(callArgs?.[1]).toBe('tasks.aiPlan')
    expect((callArgs?.[2] as { traceId?: string }).traceId).toBe(startEvent?.id)
  })

  it('without instruction → start({hasInstruction:false, promptLength:0})', async () => {
    mockRpc.mockResolvedValueOnce({ subtasks: [] })
    const { events, stop } = captureTraceEvents()
    const { useTask } = await import('../useTask')
    const { result } = renderHook(() => useTask('task-1'))

    await act(async () => {
      await result.current.aiDecompose()
    })
    stop()

    const startEvent = events.find((e) => e.flow === 'ui:taskGraph.aiPlan' && e.level === 'start')
    expect(startEvent?.fields.hasInstruction).toBe(false)
    expect(startEvent?.fields.promptLength).toBe(0)
  })

  it('success → span.ok({taskId, subtaskCount}) matches result.subtasks.length', async () => {
    mockRpc.mockResolvedValueOnce({ subtasks: [{ title: 'a' }, { title: 'b' }, { title: 'c' }] })
    const { events, stop } = captureTraceEvents()
    const { useTask } = await import('../useTask')
    const { result } = renderHook(() => useTask('task-1'))

    await act(async () => {
      await result.current.aiDecompose('instr')
    })
    stop()

    const okEvent = events.find((e) => e.flow === 'ui:taskGraph.aiPlan' && e.level === 'ok')
    expect(okEvent?.fields.taskId).toBe('task-1')
    expect(okEvent?.fields.subtaskCount).toBe(3)
  })

  it('RPC error → span.fail(err, {taskId}) before re-throw', async () => {
    const err = new Error('rpc boom')
    mockRpc.mockRejectedValueOnce(err)
    const { events, stop } = captureTraceEvents()
    const { useTask } = await import('../useTask')
    const { result } = renderHook(() => useTask('task-1'))

    await expect(
      act(async () => {
        await result.current.aiDecompose('instr')
      })
    ).rejects.toThrow('rpc boom')
    stop()

    const failEvents = events.filter((e) => e.flow === 'ui:taskGraph.aiPlan' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.taskId).toBe('task-1')
    expect(failEvents[0]?.fields.err).toContain('rpc boom')
  })

  it('updateTask/deleteTask/acceptSubtasks — regression: no ui:taskGraph.aiPlan span emitted', async () => {
    mockRpc.mockResolvedValueOnce(undefined) // tasks.update
    mockRpc.mockResolvedValueOnce(undefined) // tasks.delete
    mockRpc.mockResolvedValueOnce([]) // tasks.createSubtasks
    const { events, stop } = captureTraceEvents()
    const { useTask } = await import('../useTask')
    const { result } = renderHook(() => useTask('task-1'))

    await act(async () => {
      await result.current.updateTask({ title: 'new title' })
      await result.current.deleteTask()
      await result.current.acceptSubtasks([{ title: 'x' }], 'proj-1')
    })
    stop()

    expect(events.filter((e) => e.flow === 'ui:taskGraph.aiPlan')).toHaveLength(0)
  })
})
