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
  templates: [],
  executions: [],
  addTemplate: vi.fn(),
  addExecution: vi.fn(),
  settings: {}
}

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: any) => fn ? fn(mockStore) : mockStore,
    { getState: () => mockStore }
  )
}))

vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn() }
}))

const mockRpc = vi.mocked(callRuntimeRpc)

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('useWorkflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.templates = []
    mockStore.executions = []
  })

  it('saveTemplate without templateId → calls workflow.template.create with target as first arg, mode=create', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-id' })
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    await act(async () => {
      await result.current.saveTemplate()
    })
    stop()

    // Bug fix: signature is callRuntimeRpc(target, method, params)
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.create', expect.any(Object))
    expect(mockStore.addTemplate).toHaveBeenCalledWith({ id: 'new-id' })

    const startEvent = events.find(e => e.flow === 'ui:workflow.templateSave' && e.level === 'start')
    expect(startEvent?.fields.mode).toBe('create')
  })

  it('saveTemplate with templateId → calls workflow.template.update, mode=update', async () => {
    mockStore.templates = [{ id: 't1', name: 'Existing' }]
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow('t1'))

    await act(async () => {
      await result.current.saveTemplate()
    })
    stop()

    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.update', expect.objectContaining({ templateId: 't1' }))
    const startEvent = events.find(e => e.flow === 'ui:workflow.templateSave' && e.level === 'start')
    expect(startEvent?.fields.mode).toBe('update')
  })

  it('saveTemplate forwards traceId: span.id into RPC params', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-id' })
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    await act(async () => {
      await result.current.saveTemplate()
    })
    stop()

    const startEvent = events.find(e => e.flow === 'ui:workflow.templateSave' && e.level === 'start')
    const callArgs = mockRpc.mock.calls[0]
    expect((callArgs?.[2] as { traceId?: string }).traceId).toBe(startEvent?.id)
  })

  it('runWorkflow(templateId) calls workflow.execute with target as first arg, forwards traceId, saves rootTraceId on addExecution', async () => {
    mockStore.templates = [{ id: 't1', name: 'Existing' }]
    mockRpc.mockResolvedValueOnce({ id: 'exec-1' })
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow('t1'))

    let execId: string | null = null
    await act(async () => {
      execId = await result.current.runWorkflow({ foo: 'bar' })
    })
    stop()

    const startEvent = events.find(e => e.flow === 'ui:workflow.execute' && e.level === 'start')
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.execute', { templateId: 't1', inputs: { foo: 'bar' }, traceId: startEvent?.id })
    expect(execId).toBe('exec-1')

    expect(mockStore.addExecution).toHaveBeenCalledWith(expect.objectContaining({
      id: 'exec-1', templateId: 't1', rootTraceId: startEvent?.id
    }))
  })

  it('runWorkflow without templateId → no span created, returns null', async () => {
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    let execId: string | null = 'not-null'
    await act(async () => {
      execId = await result.current.runWorkflow()
    })
    stop()

    expect(execId).toBeNull()
    expect(mockRpc).not.toHaveBeenCalled()
    expect(events.filter(e => e.flow === 'ui:workflow.execute')).toHaveLength(0)
  })

  it('runWorkflow RPC error → span.fail(err, {templateId}), returns null', async () => {
    mockStore.templates = [{ id: 't1', name: 'Existing' }]
    const err = new Error('rpc boom')
    mockRpc.mockRejectedValueOnce(err)
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow('t1'))

    let execId: string | null = 'not-null'
    await act(async () => {
      execId = await result.current.runWorkflow()
    })
    stop()

    expect(execId).toBeNull()
    const failEvents = events.filter(e => e.flow === 'ui:workflow.execute' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.templateId).toBe('t1')
    expect(mockStore.addExecution).not.toHaveBeenCalled()
  })

  it('saveTemplate RPC error → span.fail(err, {mode}), error re-thrown', async () => {
    const err = new Error('save boom')
    mockRpc.mockRejectedValueOnce(err)
    const { events, stop } = captureTraceEvents()
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    await expect(
      act(async () => {
        await result.current.saveTemplate()
      })
    ).rejects.toThrow('save boom')
    stop()

    const failEvents = events.filter(e => e.flow === 'ui:workflow.templateSave' && e.level === 'fail')
    expect(failEvents).toHaveLength(1)
    expect(failEvents[0]?.fields.mode).toBe('create')
  })

  it('updateTemplate merges patch into existing template', async () => {
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    act(() => {
      result.current.updateTemplate({ name: 'New Name' })
    })

    expect(result.current.template.name).toBe('New Name')
  })

  it('addStep adds a new step and removeStep removes it', async () => {
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())

    let stepId = ''
    act(() => {
      stepId = result.current.addStep()
    })

    expect(result.current.template.steps).toHaveLength(1)
    expect(result.current.template.steps?.[0].id).toBe(stepId)

    act(() => {
      result.current.removeStep(stepId)
    })

    expect(result.current.template.steps).toHaveLength(0)
  })
})
