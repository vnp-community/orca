// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { Children, isValidElement } from 'react'
import type { ReactElement, ReactNode } from 'react'
import { LinkedProjectsManager } from '../LinkedProjectsManager'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'

// The literal fixtures below only ever populate `settings`/`projects` — far
// short of the real (many-slice) AppState — so this mirrors just the shape
// these tests actually construct, in place of the full store type.
type MockAppState = {
  settings: Record<string, never>
  projects: typeof myProjects
}

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' }),
  // describeError()'s `err instanceof RuntimeRpcCallError` check needs a real
  // class here — a plain thrown Error() also satisfies the `instanceof Error`
  // half of that check, but the class itself must still exist on the mock.
  RuntimeRpcCallError: class RuntimeRpcCallError extends Error {}
}))

// vi.mock factories are hoisted above top-level const declarations — use
// vi.hoisted() so the fixture is available both inside the factory and in
// test bodies below.
const { myProjects } = vi.hoisted(() => ({
  myProjects: [
    { id: 'p1', displayName: 'Backend' },
    { id: 'p2', displayName: 'Mobile App' }
  ]
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {}, projects: myProjects }) }
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

// Flatten Select/SelectTrigger/SelectContent into one native <select>, pulling
// data-testid off the SelectTrigger child — same pattern as
// CreateProjectDialog.test.tsx (LinkedProjectsManager puts data-testid on
// SelectTrigger, not on Select itself).
vi.mock('../../ui/select', () => {
  const SelectContent = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectTrigger = (p: { children: ReactNode }) => <>{p.children}</>
  const SelectItem = (p: { value: string; children: ReactNode }) => (
    <option value={p.value}>{p.children}</option>
  )

  const Select = (p: {
    value: string
    onValueChange: (value: string) => void
    children: ReactNode
  }) => {
    const children = Children.toArray(p.children)
    const trigger = children.find((c) => isValidElement(c) && c.type === SelectTrigger) as
      | ReactElement<{ 'data-testid'?: string }>
      | undefined
    const content = children.find((c) => isValidElement(c) && c.type === SelectContent) as
      | ReactElement<{ children?: ReactNode }>
      | undefined
    return (
      <select
        data-testid={trigger?.props?.['data-testid']}
        value={p.value}
        onChange={(e) => p.onValueChange(e.target.value)}
      >
        {content?.props?.children}
      </select>
    )
  }

  return { Select, SelectContent, SelectItem, SelectTrigger, SelectValue: () => null }
})

vi.mock('../../ui/button', () => ({
  Button: (p: {
    'data-testid'?: string
    'aria-label'?: string
    disabled?: boolean
    onClick?: () => void
    children: ReactNode
  }) => (
    <button
      data-testid={p['data-testid']}
      aria-label={p['aria-label']}
      disabled={p.disabled}
      onClick={p.onClick}
    >
      {p.children}
    </button>
  )
}))

describe('LinkedProjectsManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useAppStore.getState).mockReturnValue({
      settings: {},
      projects: myProjects
    } as MockAppState)
  })

  afterEach(cleanup)

  it('shows loading state while fetching', () => {
    vi.mocked(callRuntimeRpc).mockReturnValue(new Promise(() => {}))
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)
    expect(screen.getByTestId('linked-loading')).toBeInTheDocument()
  })

  it('fetches via orcaProjects.list and filters to the current orcaProjectId', async () => {
    // p1/p2 appear as their own entries too — orcaProjects.list returns
    // EVERY OrcaProject the caller belongs to, real project.projects rows
    // (real UUIDs), which is also where the link picker and label
    // resolution now source from (never useAppStore's `projects`, the
    // client-only Project Host Setup projection — see
    // LinkedProjectsManager.tsx's linkableProjects doc comment for why).
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u1', projectId: 'p1' }] },
      { orcaProject: { id: 'op2' }, sourceProjects: [{ ownerUserId: 'u1', projectId: 'p2' }] },
      { orcaProject: { id: 'p1', name: 'Backend' }, sourceProjects: [] }
    ])
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'orcaProjects.list', null)
    })
    await waitFor(() => {
      expect(screen.getByTestId('linked-row-p1')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('linked-row-p2')).not.toBeInTheDocument()
    expect(screen.getByText('Backend')).toBeInTheDocument()
    expect(screen.getByText('u1')).toBeInTheDocument()
  })

  it('shows empty state when no projects are linked', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { orcaProject: { id: 'op1' }, sourceProjects: [] }
    ])
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="owner" />)
    await waitFor(() => expect(screen.getByTestId('linked-empty')).toBeInTheDocument())
  })

  it('link form only offers not-yet-linked projects, and submits linkSourceProject', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'orcaProjects.list') {
        return [
          { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u1', projectId: 'p1' }] },
          { orcaProject: { id: 'p1', name: 'Backend' }, sourceProjects: [] },
          { orcaProject: { id: 'p2', name: 'Mobile App' }, sourceProjects: [] }
        ]
      }
      if (method === 'orcaProjects.linkSourceProject') {
        return { success: true }
      }
      return null
    })
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="owner" />)
    await waitFor(() => expect(screen.getByTestId('linked-row-p1')).toBeInTheDocument())

    const select = screen.getByTestId('link-project-select') as HTMLSelectElement
    // p1 already linked — only p2 should be selectable.
    await waitFor(() => expect(select.querySelectorAll('option')).toHaveLength(1))

    fireEvent.change(select, { target: { value: 'p2' } })
    fireEvent.click(screen.getByTestId('link-project-submit'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        expect.anything(),
        'orcaProjects.linkSourceProject',
        {
          orcaProjectId: 'op1',
          projectId: 'p2'
        }
      )
    })
    // Reloaded after linking.
    expect(
      vi.mocked(callRuntimeRpc).mock.calls.filter((c) => c[1] === 'orcaProjects.list').length
    ).toBeGreaterThanOrEqual(2)
  })

  it('unlink button only renders when currentUserRole is owner', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue([
      { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u1', projectId: 'p1' }] }
    ])
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)
    await waitFor(() => expect(screen.getByTestId('linked-row-p1')).toBeInTheDocument())
    expect(screen.queryByTestId('unlink-project-p1')).not.toBeInTheDocument()
  })

  it('owner can unlink — calls unlinkSourceProject and removes the row immediately', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'orcaProjects.list') {
        return [
          { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u1', projectId: 'p1' }] }
        ]
      }
      if (method === 'orcaProjects.unlinkSourceProject') {
        return { success: true }
      }
      return null
    })
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="owner" />)
    await waitFor(() => expect(screen.getByTestId('linked-row-p1')).toBeInTheDocument())

    fireEvent.click(screen.getByTestId('unlink-project-p1'))

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(
        expect.anything(),
        'orcaProjects.unlinkSourceProject',
        {
          orcaProjectId: 'op1',
          projectId: 'p1'
        }
      )
    })
    await waitFor(() => expect(screen.queryByTestId('linked-row-p1')).not.toBeInTheDocument())
  })

  it('resolves the display name via getProjectData for a linked project not in myProjects', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method, params) => {
      if (method === 'orcaProjects.list') {
        return [
          { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u2', projectId: 'p99' }] }
        ]
      }
      if (method === 'orcaProjects.getProjectData') {
        expect(params).toEqual({ orcaProjectId: 'op1', projectId: 'p99' })
        return { project: { name: 'Shared By Teammate' }, repos: [], worktrees: [] }
      }
      return null
    })
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)

    await waitFor(() => expect(screen.getByTestId('linked-row-p99')).toBeInTheDocument())
    await waitFor(() => expect(screen.getByText('Shared By Teammate')).toBeInTheDocument())
    expect(screen.queryByText('p99')).not.toBeInTheDocument()
  })

  it('falls back to the raw projectId when getProjectData fails to resolve it', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'orcaProjects.list') {
        return [
          { orcaProject: { id: 'op1' }, sourceProjects: [{ ownerUserId: 'u2', projectId: 'p99' }] }
        ]
      }
      if (method === 'orcaProjects.getProjectData') {
        throw new Error('FORBIDDEN: not a member')
      }
      return null
    })
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)

    await waitFor(() => expect(screen.getByTestId('linked-row-p99')).toBeInTheDocument())
    expect(screen.getByText('p99')).toBeInTheDocument()
  })

  it('shows a friendly message on FORBIDDEN errors from link', async () => {
    const { toast } = await import('sonner')
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'orcaProjects.list') {
        return [
          { orcaProject: { id: 'op1' }, sourceProjects: [] },
          { orcaProject: { id: 'p1', name: 'Backend' }, sourceProjects: [] }
        ]
      }
      if (method === 'orcaProjects.linkSourceProject') {
        throw new Error('FORBIDDEN: not a member')
      }
      return null
    })
    render(<LinkedProjectsManager orcaProjectId="op1" currentUserRole="member" />)
    await waitFor(() => expect(screen.getByTestId('linked-empty')).toBeInTheDocument())
    await waitFor(() => expect(screen.getByText('Backend')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('link-project-select'), { target: { value: 'p1' } })
    fireEvent.click(screen.getByTestId('link-project-submit'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('You do not have permission to do that.')
    })
  })
})
