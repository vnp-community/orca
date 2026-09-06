/**
 * Tests for `useWorkflowExecution().cancelExecution()` tracing (TASK-FE-017.2).
 *
 * Covers `src/renderer/src/hooks/useWorkflowExecution.ts` — instrumented with
 * `Tracers.uiWorkflowCancelFlow` (`ui:workflow.cancel`), forwarding `traceId`
 * to `workflow.cancel` and reading `parentTraceId` from the execution's
 * previously-saved `rootTraceId` (TASK-FE-017.1) — a free-form grouping
 * field, NOT the `resume` mechanism.
 *
 * @module renderer/hooks/__tests__/useWorkflowExecution.test
 */
// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { registerTraceSink, type TraceEvent } from '../../../../shared/trace'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

type MockExecution = { id: string; templateId: string; status: string; rootTraceId?: string }
type MockStore = {
  executions: MockExecution[]
  stepStatuses: Record<string, unknown>
  streamingOutput: Record<string, unknown>
  updateExecutionStatus: ReturnType<typeof vi.fn>
  settings: Record<string, unknown>
}

const mockStore: MockStore = {
  executions: [],
  stepStatuses: {},
  streamingOutput: {},
  updateExecutionStatus: vi.fn(),
  settings: {}
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (store: MockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('useWorkflowExecution().cancelExecution() tracing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.executions = [
      { id: 'exec-1', templateId: 't1', status: 'running', rootTraceId: 'root-abc' }
    ]
    mockStore.stepStatuses = {}
    mockStore.streamingOutput = {}
  })

  it('start({executionId, parentTraceId}) reads parentTraceId from execution.rootTraceId', async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const { events, stop } = captureTraceEvents()
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    const { result } = renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await result.current.cancelExecution()
    })
    stop()

    const startEvent = events.find((e) => e.flow === 'ui:workflow.cancel' && e.level === 'start')
    expect(startEvent?.fields.executionId).toBe('exec-1')
    expect(startEvent?.fields.parentTraceId).toBe('root-abc')
  })

  it('forwards traceId: span.id into workflow.cancel params, target as first arg', async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const { events, stop } = captureTraceEvents()
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    const { result } = renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await result.current.cancelExecution()
    })
    stop()

    const startEvent = events.find((e) => e.flow === 'ui:workflow.cancel' && e.level === 'start')
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.cancel', {
      executionId: 'exec-1',
      traceId: startEvent?.id
    })
  })

  it('success → updateExecutionStatus(executionId, "cancelled") then span.ok({executionId})', async () => {
    mockRpc.mockResolvedValueOnce(undefined)
    const { events, stop } = captureTraceEvents()
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    const { result } = renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await result.current.cancelExecution()
    })
    stop()

    expect(mockStore.updateExecutionStatus).toHaveBeenCalledWith('exec-1', 'cancelled')
    const okEvent = events.find((e) => e.flow === 'ui:workflow.cancel' && e.level === 'ok')
    expect(okEvent?.fields.executionId).toBe('exec-1')
  })

  it('RPC reject → span.fail(err, {executionId}), status not changed, error re-thrown', async () => {
    const err = new Error('cancel boom')
    mockRpc.mockRejectedValueOnce(err)
    const { events, stop } = captureTraceEvents()
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    const { result } = renderHook(() => useWorkflowExecution('exec-1'))

    await expect(
      act(async () => {
        await result.current.cancelExecution()
      })
    ).rejects.toThrow('cancel boom')
    stop()

    const failEvents = events.filter((e) => e.flow === 'ui:workflow.cancel' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.executionId).toBe('exec-1')
    expect(mockStore.updateExecutionStatus).not.toHaveBeenCalled()
  })

  it('no ui:workflow.stepExecute span emitted for step-level push events (backend owns step tracing)', async () => {
    const { events, stop } = captureTraceEvents()
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    renderHook(() => useWorkflowExecution('exec-1'))
    stop()

    expect(events.filter((e) => e.flow.includes('stepExecute'))).toHaveLength(0)
  })
})

// CR-PW-006 (interim): useWorkflowExecution used to subscribe via
// `window.api.on('workflow:stepStatus'|'workflow:stepOutput'|'workflow:complete', ...)`,
// which is silently a no-op in Web mode (see this file's own header comment). It now polls
// `workflow.getExecution` instead — cross-platform, since it goes through callRuntimeRpc
// like every other RPC call in this codebase.
describe('useWorkflowExecution() execution-status polling (CR-PW-006 interim)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    mockStore.executions = [
      { id: 'exec-1', templateId: 't1', status: 'running', rootTraceId: 'root-abc' }
    ]
    mockStore.stepStatuses = {}
    mockStore.streamingOutput = {}
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('polls workflow.getExecution on an interval while status is running', async () => {
    mockRpc.mockResolvedValue({ id: 'exec-1', status: 'running' })
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    renderHook(() => useWorkflowExecution('exec-1'))

    expect(mockRpc).not.toHaveBeenCalledWith(
      'mock-target',
      'workflow.getExecution',
      expect.anything()
    )

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.getExecution', {
      executionId: 'exec-1'
    })

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(mockRpc).toHaveBeenCalledTimes(2)
  })

  it('a poll result updates the store via updateExecutionStatus', async () => {
    mockRpc.mockResolvedValue({ id: 'exec-1', status: 'completed' })
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(mockStore.updateExecutionStatus).toHaveBeenCalledWith('exec-1', 'completed')
  })

  it('does not poll once the execution is no longer running (terminal status)', async () => {
    mockStore.executions = [{ id: 'exec-1', templateId: 't1', status: 'completed' }]
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000)
    })
    expect(mockRpc).not.toHaveBeenCalledWith(
      'mock-target',
      'workflow.getExecution',
      expect.anything()
    )
  })

  it('stops polling after unmount', async () => {
    mockRpc.mockResolvedValue({ id: 'exec-1', status: 'running' })
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    const { unmount } = renderHook(() => useWorkflowExecution('exec-1'))
    unmount()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000)
    })
    expect(mockRpc).not.toHaveBeenCalledWith(
      'mock-target',
      'workflow.getExecution',
      expect.anything()
    )
  })

  it('a rejected poll does not throw or crash the hook', async () => {
    mockRpc.mockRejectedValue(new Error('network down'))
    const { useWorkflowExecution } = await import('../useWorkflowExecution')
    renderHook(() => useWorkflowExecution('exec-1'))

    await act(async () => {
      await vi.advanceTimersByTimeAsync(4_000)
    })
    expect(mockStore.updateExecutionStatus).not.toHaveBeenCalled()
  })
})
