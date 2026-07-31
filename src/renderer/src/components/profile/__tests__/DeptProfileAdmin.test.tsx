// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { DeptProfileAdmin } from '../DeptProfileAdmin'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('../../ui/badge', () => ({
  Badge: ({ children, onClick, 'data-testid': testId }: any) => (
    <button data-testid={testId} onClick={onClick}>{children}</button>
  )
}))

vi.mock('../../ui/skeleton', () => ({
  Skeleton: () => <div data-testid="skeleton" />
}))

vi.mock('../ProfileEditor', () => ({
  ProfileEditor: ({ scope, scopeId }: any) => (
    <div data-testid="mock-profile-editor">{scope} - {scopeId}</div>
  )
}))

describe('DeptProfileAdmin', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading skeleton while fetching departments', () => {
    // Return an unresolved promise to keep it loading
    vi.mocked(callRuntimeRpc).mockReturnValue(new Promise(() => {}))
    render(<DeptProfileAdmin />)
    expect(screen.getByTestId('dept-profile-loading')).toBeInTheDocument()
  })

  it('shows empty state when no departments returned', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([])
    render(<DeptProfileAdmin />)
    await waitFor(() => {
      expect(screen.getByTestId('dept-profile-empty')).toBeInTheDocument()
    })
  })

  it('shows department badges after load and selects first', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { id: 'd1', name: 'Engineering', memberCount: 10 },
      { id: 'd2', name: 'Sales', memberCount: 5 }
    ])
    render(<DeptProfileAdmin />)
    
    await waitFor(() => {
      expect(screen.getByTestId('dept-badge-d1')).toBeInTheDocument()
      expect(screen.getByTestId('dept-badge-d2')).toBeInTheDocument()
    })
    
    // First dept should be selected, so ProfileEditor should render with d1
    expect(screen.getByTestId('mock-profile-editor')).toHaveTextContent('dept - d1')
  })

  it('clicking a dept badge shows ProfileEditor with scope=dept for that dept', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { id: 'd1', name: 'Engineering' },
      { id: 'd2', name: 'Sales' }
    ])
    render(<DeptProfileAdmin />)
    
    await waitFor(() => {
      expect(screen.getByTestId('dept-badge-d2')).toBeInTheDocument()
    })
    
    fireEvent.click(screen.getByTestId('dept-badge-d2'))
    expect(screen.getByTestId('mock-profile-editor')).toHaveTextContent('dept - d2')
  })
})
