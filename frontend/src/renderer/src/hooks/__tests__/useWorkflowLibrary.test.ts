// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { LibraryScope } from '../useWorkflowLibrary'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (state: Record<string, unknown>) => unknown) =>
      fn ? fn({ settings: {} }) : { settings: {} },
    {
      getState: () => ({ settings: {} })
    }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useWorkflowLibrary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it("mount with scope='company' → calls workflow.template.list({scope:'company'})", async () => {
    mockRpc.mockResolvedValue({ templates: [] })
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    renderHook(() => useWorkflowLibrary('company', ''))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.list', {
        scope: 'company'
      })
    })
  })

  it('changing scope → calls the RPC again with the new scope', async () => {
    mockRpc.mockResolvedValue({ templates: [] })
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { rerender } = renderHook(
      ({ scope }: { scope: LibraryScope }) => useWorkflowLibrary(scope, ''),
      { initialProps: { scope: 'company' } }
    )
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))

    rerender({ scope: 'team' })
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.list', {
        scope: 'team'
      })
    })
  })

  it('non-empty search → filters client-side by name, case-insensitively', async () => {
    mockRpc.mockResolvedValue({
      templates: [
        { id: 't1', name: 'Deploy Pipeline', steps: [] },
        { id: 't2', name: 'Nightly Backup', steps: [] }
      ]
    })
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { result } = renderHook(() => useWorkflowLibrary('company', 'deploy'))

    await waitFor(() => {
      expect(result.current.templates).toHaveLength(1)
      expect(result.current.templates[0]?.id).toBe('t1')
    })
  })

  it('RPC error → loadError=true, templates=[]', async () => {
    mockRpc.mockRejectedValueOnce(new Error('boom'))
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { result } = renderHook(() => useWorkflowLibrary('company', ''))

    await waitFor(() => {
      expect(result.current.loadError).toBe(true)
      expect(result.current.templates).toEqual([])
    })
  })

  it('reload() re-runs the RPC', async () => {
    mockRpc.mockResolvedValue({ templates: [] })
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { result } = renderHook(() => useWorkflowLibrary('company', ''))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))

    await act(async () => {
      await result.current.reload()
    })
    expect(mockRpc).toHaveBeenCalledTimes(2)
  })
})
