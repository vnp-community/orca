// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TeamAdmin } from '../TeamAdmin'
import { callRuntimeRpc, RuntimeRpcCallError } from '../../../runtime/runtime-rpc-client'
import type * as RuntimeRpcClientModule from '../../../runtime/runtime-rpc-client'
import type { RuntimeRpcFailure } from '../../../../../shared/runtime-rpc-envelope'

vi.mock('../../../runtime/runtime-rpc-client', async () => {
  const actual = await vi.importActual<typeof RuntimeRpcClientModule>(
    '../../../runtime/runtime-rpc-client'
  )
  return {
    ...actual,
    callRuntimeRpc: vi.fn(),
    getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
  }
})

function forbiddenFailure(message: string): RuntimeRpcFailure {
  return { id: 'test', ok: false, error: { code: 'runtime_error', message } }
}

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('../../ui/skeleton', () => ({
  Skeleton: () => <div data-testid="skeleton" />
}))

const TEAMS = [
  { id: 't1', name: 'Platform', createdAt: '2026-01-01', updatedAt: '2026-01-01' },
  { id: 't2', name: 'Growth', createdAt: '2026-01-02', updatedAt: '2026-01-02' }
]

const MEMBERS = [
  { teamId: 't1', userId: 'u1', role: 'lead', priority: 1, addedAt: '2026-01-01' },
  { teamId: 't1', userId: 'u2', role: 'member', priority: 2, addedAt: '2026-01-02' }
]

describe('TeamAdmin', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading skeleton while fetching teams', () => {
    vi.mocked(callRuntimeRpc).mockReturnValue(new Promise(() => {}))
    render(<TeamAdmin />)
    expect(screen.getByTestId('team-admin-loading')).toBeInTheDocument()
  })

  it('shows empty state when no teams returned', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([])
    render(<TeamAdmin />)
    await waitFor(() => {
      expect(screen.getByTestId('team-list-empty')).toBeInTheDocument()
    })
  })

  it('lists teams after load', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue(TEAMS)
    render(<TeamAdmin />)

    await waitFor(() => {
      expect(screen.getByTestId('team-row-t1')).toBeInTheDocument()
      expect(screen.getByTestId('team-row-t2')).toBeInTheDocument()
    })
    expect(screen.getByText('Platform')).toBeInTheDocument()
    expect(screen.getByText('Growth')).toBeInTheDocument()
  })

  it('shows a clear message when team.list is forbidden for a non-admin', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue(
      new RuntimeRpcCallError(forbiddenFailure('FORBIDDEN: admin role required'))
    )
    render(<TeamAdmin />)

    await waitFor(() => {
      expect(screen.getByTestId('team-list-error')).toHaveTextContent('Admin access required')
    })
  })

  it('creates a new team via team.create', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('team-list')).toBeInTheDocument())

    const created = { id: 't3', name: 'Infra', createdAt: '2026-01-03', updatedAt: '2026-01-03' }
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(created)

    fireEvent.change(screen.getByTestId('new-team-name-input'), { target: { value: 'Infra' } })
    fireEvent.click(screen.getByTestId('create-team-btn'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        { type: 'local' },
        'team.create',
        { name: 'Infra' }
      )
    })
    await waitFor(() => {
      expect(screen.getByTestId('team-row-t3')).toBeInTheDocument()
    })
  })

  it('surfaces a forbidden error from team.create without crashing', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('team-list')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockRejectedValueOnce(
      new RuntimeRpcCallError(forbiddenFailure('UNAUTHENTICATED'))
    )

    fireEvent.change(screen.getByTestId('new-team-name-input'), { target: { value: 'Infra' } })
    fireEvent.click(screen.getByTestId('create-team-btn'))

    await waitFor(() => {
      expect(screen.getByTestId('create-team-error')).toHaveTextContent('Admin access required')
    })
    // UI stays intact — team list still rendered
    expect(screen.getByTestId('team-list')).toBeInTheDocument()
  })

  it('loads and displays members when "View Members" is clicked', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('view-members-btn-t1')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(MEMBERS)
    fireEvent.click(screen.getByTestId('view-members-btn-t1'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        { type: 'local' },
        'team.listMembers',
        { teamId: 't1' }
      )
    })
    await waitFor(() => {
      expect(screen.getByTestId('member-row-u1')).toBeInTheDocument()
      expect(screen.getByTestId('member-row-u2')).toBeInTheDocument()
    })
  })

  it('adds a member via team.addMember and refreshes the list', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('view-members-btn-t1')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce([])
    fireEvent.click(screen.getByTestId('view-members-btn-t1'))
    await waitFor(() => expect(screen.getByTestId('team-members-empty')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(undefined) // team.addMember -> void
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(MEMBERS)   // refetch after add

    fireEvent.change(screen.getByTestId('new-member-user-id-input'), { target: { value: 'u1' } })
    fireEvent.change(screen.getByTestId('new-member-role-input'), { target: { value: 'lead' } })
    fireEvent.change(screen.getByTestId('new-member-priority-input'), { target: { value: '1' } })
    fireEvent.click(screen.getByTestId('add-member-btn'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        { type: 'local' },
        'team.addMember',
        { teamId: 't1', userId: 'u1', role: 'lead', priority: 1 }
      )
    })
    await waitFor(() => {
      expect(screen.getByTestId('member-row-u1')).toBeInTheDocument()
    })
  })

  it('removes a member via team.removeMember', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('view-members-btn-t1')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(MEMBERS)
    fireEvent.click(screen.getByTestId('view-members-btn-t1'))
    await waitFor(() => expect(screen.getByTestId('member-row-u1')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(undefined) // team.removeMember -> void
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce([MEMBERS[1]]) // refetch after remove

    fireEvent.click(screen.getByTestId('remove-member-btn-u1'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        { type: 'local' },
        'team.removeMember',
        { teamId: 't1', userId: 'u1' }
      )
    })
    await waitFor(() => {
      expect(screen.queryByTestId('member-row-u1')).not.toBeInTheDocument()
      expect(screen.getByTestId('member-row-u2')).toBeInTheDocument()
    })
  })

  it('shows a clear message when addMember is forbidden, without crashing the panel', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce(TEAMS)
    render(<TeamAdmin />)
    await waitFor(() => expect(screen.getByTestId('view-members-btn-t1')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockResolvedValueOnce([])
    fireEvent.click(screen.getByTestId('view-members-btn-t1'))
    await waitFor(() => expect(screen.getByTestId('team-members-empty')).toBeInTheDocument())

    vi.mocked(callRuntimeRpc).mockRejectedValueOnce(
      new RuntimeRpcCallError(forbiddenFailure('FORBIDDEN: admin role required'))
    )

    fireEvent.change(screen.getByTestId('new-member-user-id-input'), { target: { value: 'u9' } })
    fireEvent.change(screen.getByTestId('new-member-role-input'), { target: { value: 'member' } })
    fireEvent.click(screen.getByTestId('add-member-btn'))

    await waitFor(() => {
      expect(screen.getByTestId('add-member-error')).toHaveTextContent('Admin access required')
    })
    expect(screen.getByTestId('team-members-panel')).toBeInTheDocument()
  })
})
