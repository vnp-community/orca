// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import type { ChangeEvent, ReactNode } from 'react'
import { RepoMemberManager } from '../RepoMemberManager'
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

describe('RepoMemberManager', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(cleanup)

  it('fetches members via repo.getMembers on mount', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([])
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.getMembers', {
        repoId: 'r1'
      })
    })
  })

  it('renders repo-member rows keyed by userId + functional role', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'dev-1', role: 'developer' }])
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => {
      expect(screen.getByTestId('repo-member-row-dev-1')).toBeInTheDocument()
      expect(screen.getByText('dev-1')).toBeInTheDocument()
    })
  })

  it('shows the opt-in empty state when there are no grants', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([])
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => expect(screen.getByTestId('repo-member-empty')).toBeInTheDocument())
  })

  it('role Select calls repo.updateMemberRole on change', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'dev-1', role: 'developer' }])
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => expect(screen.getByTestId('repo-member-row-dev-1')).toBeInTheDocument())

    const row = screen.getByTestId('repo-member-row-dev-1')
    const roleSelect = row.querySelector('select') as HTMLSelectElement
    fireEvent.change(roleSelect, { target: { value: 'admin' } })

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.updateMemberRole', {
        repoId: 'r1',
        userId: 'dev-1',
        role: 'admin'
      })
    })
  })

  it('remove button calls repo.removeMember', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([{ userId: 'dev-1', role: 'developer' }])
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => expect(screen.getByTestId('repo-member-row-dev-1')).toBeInTheDocument())

    fireEvent.click(screen.getByTestId('remove-repo-member-dev-1'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.removeMember', {
        repoId: 'r1',
        userId: 'dev-1'
      })
    })
  })

  it('add-member form calls repo.addMember and reloads the list', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'repo.getMembers') {
        return []
      }
      if (method === 'auth.listTenantMemberDirectory') {
        return [{ id: 'dev-2', name: 'New Dev', email: 'dev-2@example.com' }]
      }
      if (method === 'repo.addMember') {
        return { userId: 'dev-2', role: 'lead' }
      }
      return null
    })
    render(<RepoMemberManager repoId="r1" />)
    await waitFor(() => expect(screen.getByTestId('repo-member-empty')).toBeInTheDocument())

    const submit = screen.getByTestId('add-repo-member-submit')
    expect(submit).toBeDisabled()

    fireEvent.change(screen.getByTestId('add-repo-member-user-id'), { target: { value: 'dev-2' } })
    expect(submit).not.toBeDisabled()

    fireEvent.click(submit)

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.addMember', {
        repoId: 'r1',
        userId: 'dev-2',
        role: 'developer'
      })
    })
    expect(
      vi.mocked(callRuntimeRpc).mock.calls.filter((c) => c[1] === 'repo.getMembers').length
    ).toBeGreaterThanOrEqual(2)
  })
})
