// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskAccessPanel } from '../TaskAccessPanel'

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector) => selector({ currentUser: { id: 'u1' }, settings: {} })),
    {
      getState: () => ({ settings: {} })
    }
  )
}))

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

describe('TaskAccessPanel', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('task.resolvePermission not wired (rejects) → shows the unsupported message, no form', async () => {
    mockRpc.mockRejectedValueOnce(new Error('method not found'))
    render(<TaskAccessPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('task-access-unsupported')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('task-access-panel')).not.toBeInTheDocument()
  })

  it('task.resolvePermission resolves → shows own permission level and the grant form', async () => {
    mockRpc.mockResolvedValueOnce({ effectiveLevel: 'GRANT_LEVEL_ADMIN' })
    render(<TaskAccessPanel taskId="t1" />)
    await waitFor(() => {
      expect(screen.getByTestId('own-permission-badge')).toHaveTextContent('admin')
    })
    expect(screen.getByTestId('task-access-panel')).toBeInTheDocument()
  })

  it('submitting a grant calls task.grant with GRANT_LEVEL_<level> and applyTree', async () => {
    mockRpc.mockResolvedValueOnce({ effectiveLevel: 'GRANT_LEVEL_OWNER' }) // resolvePermission
    render(<TaskAccessPanel taskId="t1" />)
    await waitFor(() => expect(screen.getByTestId('task-access-panel')).toBeInTheDocument())

    fireEvent.change(screen.getByPlaceholderText('User ID hoặc Team ID'), {
      target: { value: 'user-42' }
    })
    mockRpc.mockResolvedValueOnce(undefined) // task.grant
    fireEvent.click(screen.getByRole('button', { name: /Cấp quyền/ }))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'task.grant', {
        taskId: 't1',
        subjectId: 'user-42',
        level: 'GRANT_LEVEL_USER',
        applyTree: false
      })
    })
    expect(mockToast.success).toHaveBeenCalled()
  })

  it('task.grant failure → toast.error, does not throw', async () => {
    mockRpc.mockResolvedValueOnce({ effectiveLevel: 'GRANT_LEVEL_OWNER' })
    render(<TaskAccessPanel taskId="t1" />)
    await waitFor(() => expect(screen.getByTestId('task-access-panel')).toBeInTheDocument())

    fireEvent.change(screen.getByPlaceholderText('User ID hoặc Team ID'), {
      target: { value: 'user-42' }
    })
    mockRpc.mockRejectedValueOnce(new Error('rpc not wired'))
    fireEvent.click(screen.getByRole('button', { name: /Cấp quyền/ }))

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalled()
    })
  })
})
