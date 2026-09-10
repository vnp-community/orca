// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { LibraryScope } from '../useWorkflowLibrary'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

const mockStore: Record<string, unknown> = { settings: {} }

vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (state: typeof mockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))

const mockRpc = vi.mocked(callRuntimeRpc)

describe('useWorkflowLibrary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRpc.mockResolvedValue({ templates: [] })
  })

  it("mount with scope='company' → calls workflow.template.list({scope:'company'})", async () => {
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    renderHook(() => useWorkflowLibrary('company', ''))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'workflow.template.list', {
        scope: 'company'
      })
    })
  })

  it('changing scope → re-calls the RPC with the new scope', async () => {
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { rerender } = renderHook(
      ({ scope }: { scope: LibraryScope }) => useWorkflowLibrary(scope, ''),
      { initialProps: { scope: 'company' } }
    )
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))

    rerender({ scope: 'team' })
    await waitFor(() => {
      expect(mockRpc).toHaveBeenLastCalledWith('mock-target', 'workflow.template.list', {
        scope: 'team'
      })
    })
  })

  it('non-empty search → filters client-side by name, case-insensitive', async () => {
    mockRpc.mockResolvedValueOnce({
      templates: [
        { id: 't1', name: 'Deploy Prod', steps: [] },
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
    const { useWorkflowLibrary } = await import('../useWorkflowLibrary')
    const { result } = renderHook(() => useWorkflowLibrary('company', ''))
    await waitFor(() => expect(mockRpc).toHaveBeenCalledTimes(1))

    await act(async () => {
      await result.current.reload()
    })
    expect(mockRpc).toHaveBeenCalledTimes(2)
  })
})
