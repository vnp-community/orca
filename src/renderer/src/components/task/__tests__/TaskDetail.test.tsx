// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskDetail } from '../TaskDetail'
import { useTask } from '../../../hooks/useTask'

// Mock useAppStore for activeTaskId and settings
vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn(selector => selector({ activeTaskId: 't1', settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

// Mock useTask
vi.mock('../../../hooks/useTask', () => ({
  useTask: vi.fn()
}))

// Mock RPC
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

describe('TaskDetail', () => {
  const updateTask = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useTask).mockReturnValue({
      task: { id: 't1', title: 'My Task', status: 'todo', priority: 'high', projectId: 'p1' },
      updateTask
    } as any)
    mockRpc.mockResolvedValue({ blockedBy: [], blocks: [] })
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

  it('Execute with Agent button calls tasks.runAgent', async () => {
    render(<TaskDetail />)
    fireEvent.click(screen.getByTestId('run-agent-btn'))
    expect(mockRpc).toHaveBeenCalledWith('mock-target', 'tasks.runAgent', { taskId: 't1' })
  })

  it('dependencies section renders blocked-by list', async () => {
    mockRpc.mockResolvedValueOnce({
      blockedBy: [{ id: 'b1', title: 'Blocker 1' }],
      blocks: []
    })
    render(<TaskDetail />)
    await waitFor(() => {
      expect(screen.getByText('Blocker 1')).toBeInTheDocument()
      expect(screen.getByText('← Blocked by:')).toBeInTheDocument()
    })
  })
})
