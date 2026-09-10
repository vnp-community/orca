// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskDispatchStatusPanel } from '../TaskDispatchStatusPanel'

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ kind: 'local' })
}))
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import type { RuntimeClientTarget } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)
const mockGetTarget = vi.mocked(getActiveRuntimeTarget)

const mockFocus = vi.fn().mockResolvedValue(undefined)
vi.mock('../../terminal-pane/terminal-orchestration-task-links', () => ({
  focusRuntimeOrchestrationTask: (...args: unknown[]) => mockFocus(...args)
}))

describe('TaskDispatchStatusPanel', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockGetTarget.mockReturnValue({ kind: 'local' } as RuntimeClientTarget)
  })

  it('loading state renders before the RPC resolves', () => {
    mockRpc.mockReturnValue(new Promise(() => {})) // never resolves
    render(<TaskDispatchStatusPanel taskId="t1" />)
    expect(screen.getByText('Loading dispatch status…')).toBeInTheDocument()
  })

  it('calls orchestration.dispatchShow with { task: taskId }', async () => {
    mockRpc.mockResolvedValueOnce({ dispatch: null })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'orchestration.dispatchShow', {
        task: 't1'
      })
    })
  })

  it('dispatch: null → shows "not dispatched" message', async () => {
    mockRpc.mockResolvedValueOnce({ dispatch: null })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-none')).toBeInTheDocument()
    })
  })

  it('RPC rejects → falls back to "not dispatched" (does not crash)', async () => {
    mockRpc.mockRejectedValueOnce(new Error('boom'))
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-none')).toBeInTheDocument()
    })
  })

  it('dispatch present → shows status/assignee_handle, and Focus terminal button when assignee set', async () => {
    mockRpc.mockResolvedValueOnce({
      dispatch: {
        id: 'd1',
        orchestration_task_id: 't1',
        assignee_handle: 'term-1',
        status: 'running'
      }
    })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-status')).toBeInTheDocument()
    })
    expect(screen.getByText('running')).toBeInTheDocument()
    expect(screen.getByText('term-1')).toBeInTheDocument()
    expect(screen.getByTestId('task-dispatch-focus-terminal')).toBeInTheDocument()
  })

  it('no assignee_handle → no Focus terminal button', async () => {
    mockRpc.mockResolvedValueOnce({
      dispatch: { id: 'd1', orchestration_task_id: 't1', assignee_handle: '', status: 'queued' }
    })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-status')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('task-dispatch-focus-terminal')).not.toBeInTheDocument()
  })

  it('Focus terminal click → focusRuntimeOrchestrationTask(taskId, environmentId) using the active target', async () => {
    mockGetTarget.mockReturnValue({
      kind: 'environment',
      environmentId: 'env-1'
    } as RuntimeClientTarget)
    mockRpc.mockResolvedValueOnce({
      dispatch: {
        id: 'd1',
        orchestration_task_id: 't1',
        assignee_handle: 'term-1',
        status: 'running'
      }
    })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-focus-terminal')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTestId('task-dispatch-focus-terminal'))

    expect(mockFocus).toHaveBeenCalledWith('t1', 'env-1')
  })

  it('Focus terminal click on a local target → environmentId is null (not hard-coded, but derived)', async () => {
    mockGetTarget.mockReturnValue({ kind: 'local' } as RuntimeClientTarget)
    mockRpc.mockResolvedValueOnce({
      dispatch: {
        id: 'd1',
        orchestration_task_id: 't1',
        assignee_handle: 'term-1',
        status: 'running'
      }
    })
    render(<TaskDispatchStatusPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-dispatch-focus-terminal')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTestId('task-dispatch-focus-terminal'))

    expect(mockFocus).toHaveBeenCalledWith('t1', null)
  })
})
