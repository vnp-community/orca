// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore: any = {
  templates: [],
  executions: [],
  addTemplate: vi.fn()
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

describe('useWorkflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockStore.templates = []
    mockStore.executions = []
  })

  it('saveTemplate without templateId → calls workflow.template.create', async () => {
    mockRpc.mockResolvedValueOnce({ id: 'new-id' })
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow())
    
    await act(async () => {
      await result.current.saveTemplate()
    })
    
    expect(mockRpc).toHaveBeenCalledWith('workflow.template.create', expect.any(Object))
    expect(mockStore.addTemplate).toHaveBeenCalledWith({ id: 'new-id' })
  })

  it('saveTemplate with templateId → calls workflow.template.update', async () => {
    mockStore.templates = [{ id: 't1', name: 'Existing' }]
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow('t1'))
    
    await act(async () => {
      await result.current.saveTemplate()
    })
    
    expect(mockRpc).toHaveBeenCalledWith('workflow.template.update', expect.objectContaining({ templateId: 't1' }))
  })

  it('runWorkflow(templateId) calls workflow.execute', async () => {
    mockStore.templates = [{ id: 't1', name: 'Existing' }]
    mockRpc.mockResolvedValueOnce({ id: 'exec-1' })
    const { useWorkflow } = await import('../useWorkflow')
    const { result } = renderHook(() => useWorkflow('t1'))
    
    let execId: string | null = null
    await act(async () => {
      execId = await result.current.runWorkflow({ foo: 'bar' })
    })
    
    expect(mockRpc).toHaveBeenCalledWith('workflow.execute', { templateId: 't1', inputs: { foo: 'bar' } })
    expect(execId).toBe('exec-1')
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
