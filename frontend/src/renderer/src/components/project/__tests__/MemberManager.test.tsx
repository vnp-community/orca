// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import type { ChangeEvent, ReactNode } from 'react'
import { MemberManager } from '../MemberManager'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() }
}))

vi.mock('../../ui/table', () => ({
  Table: (p: { children: ReactNode }) => <table>{p.children}</table>,
  TableHeader: (p: { children: ReactNode }) => <thead>{p.children}</thead>,
  TableBody: (p: { children: ReactNode }) => <tbody>{p.children}</tbody>,
  TableRow: (p: { children: ReactNode; 'data-testid'?: string }) => (
    <tr data-testid={p['data-testid']}>{p.children}</tr>
  ),
  TableHead: (p: { children: ReactNode }) => <th>{p.children}</th>,
  TableCell: (p: { children: ReactNode }) => <td>{p.children}</td>
}))

// Flatten Select down to a native <select> the same way CreateProjectDialog's
// test does, so onValueChange is actually reachable from a test.
vi.mock('../../ui/select', () => {
  const SelectContent = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectTrigger = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectItem = (p: { value: string; children: ReactNode }) => (
    <option value={p.value}>{p.children}</option>
  )
  const Select = (p: {
    'data-testid'?: string
    value: string
    onValueChange: (value: string) => void
    children: ReactNode
  }) => (
    <select
      data-testid={p['data-testid']}
      value={p.value}
      onChange={(e) => p.onValueChange(e.target.value)}
    >
      {p.children}
    </select>
  )
  return { Select, SelectContent, SelectItem, SelectTrigger, SelectValue: () => null }
})

vi.mock('../../ui/button', () => ({
  Button: (p: {
    'data-testid'?: string
    disabled?: boolean
    onClick?: () => void
    children: ReactNode
  }) => (
    <button data-testid={p['data-testid']} disabled={p.disabled} onClick={p.onClick}>
      {p.children}
    </button>
  )
}))
vi.mock('../../ui/input', () => ({
  Input: (p: {
    'data-testid'?: string
    value: string
    onChange: (e: ChangeEvent<HTMLInputElement>) => void
    placeholder?: string
  }) => (
    <input
      data-testid={p['data-testid']}
      value={p.value}
      onChange={p.onChange}
      placeholder={p.placeholder}
    />
  )
}))

describe('MemberManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(cleanup)

  it('shows loading state while fetching', () => {
    vi.mocked(callRuntimeRpc).mockReturnValue(new Promise(() => {}))
    render(<MemberManager projectId="p1" />)
    expect(screen.getByTestId('member-loading')).toBeInTheDocument()
  })

  it('fetches members via project.getMembers on mount', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([])
    render(<MemberManager projectId="p1" />)
    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.getMembers', {
        projectId: 'p1'
      })
    })
  })

  // Why the raw userId here: project.proto's Member message carries just
  // {user_id, role} — no profile fields — so a userId the directory fetch
  // doesn't resolve (blanket-mocked to the wrong shape in this test) falls
  // back to displaying it verbatim. See the next test for the resolved case.
  it('renders member rows keyed by userId + role', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'u1', role: 'owner' }])
    render(<MemberManager projectId="p1" />)
    await waitFor(() => {
      expect(screen.getByTestId('member-row-u1')).toBeInTheDocument()
      expect(screen.getByText('u1')).toBeInTheDocument()
    })
  })

  // Regression guard: MemberManager used to have no way to resolve a
  // userId to a real name/email at all — this is the whole point of
  // auth.listTenantMemberDirectory existing.
  it('resolves a member row to name+email via the tenant directory', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'project.getMembers') {
        return [{ userId: 'u1', role: 'owner' }]
      }
      if (method === 'auth.listTenantMemberDirectory') {
        return [{ id: 'u1', name: 'Alice', email: 'alice@example.com' }]
      }
      return null
    })
    render(<MemberManager projectId="p1" />)
    await waitFor(() => {
      expect(screen.getByText('Alice (alice@example.com)')).toBeInTheDocument()
    })
  })

  it('role Select calls project.updateMemberRole on change', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'u1', role: 'member' }])
    render(<MemberManager projectId="p1" />)
    await waitFor(() => expect(screen.getByTestId('member-row-u1')).toBeInTheDocument())

    const row = screen.getByTestId('member-row-u1')
    const roleSelect = row.querySelector('select') as HTMLSelectElement
    fireEvent.change(roleSelect, { target: { value: 'owner' } })

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.updateMemberRole', {
        projectId: 'p1',
        userId: 'u1',
        role: 'owner'
      })
    })
  })

  it('remove button calls project.removeMember', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'u1', role: 'member' }])
    render(<MemberManager projectId="p1" />)
    await waitFor(() => expect(screen.getByTestId('member-row-u1')).toBeInTheDocument())

    fireEvent.click(screen.getByTestId('remove-member-u1'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.removeMember', {
        projectId: 'p1',
        userId: 'u1'
      })
    })
  })

  // Regression guard: no project.addMember caller existed anywhere in the
  // frontend before this — MemberManager could list/remove/re-role but had
  // no way to add a new member at all.
  it('add-member form calls project.addMember and reloads the list', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'project.getMembers') {
        return []
      }
      if (method === 'auth.listTenantMemberDirectory') {
        return [{ id: 'u2', name: 'New Member', email: 'u2@example.com' }]
      }
      if (method === 'project.addMember') {
        return { userId: 'u2', role: 'member' }
      }
      return null
    })
    render(<MemberManager projectId="p1" />)
    await waitFor(() => expect(screen.getByTestId('member-empty')).toBeInTheDocument())

    const submit = screen.getByTestId('add-member-submit')
    expect(submit).toBeDisabled()

    fireEvent.change(screen.getByTestId('add-member-user-id'), { target: { value: 'u2' } })
    expect(submit).not.toBeDisabled()

    fireEvent.click(submit)

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.addMember', {
        projectId: 'p1',
        userId: 'u2',
        role: 'member'
      })
    })
    // Reloaded after adding.
    expect(
      vi.mocked(callRuntimeRpc).mock.calls.filter((c) => c[1] === 'project.getMembers').length
    ).toBeGreaterThanOrEqual(2)
  })
})
