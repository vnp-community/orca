// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import type { MouseEventHandler, ReactNode } from 'react'
import { ProjectSettings } from '../ProjectSettings'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

// Why WorkspaceContext, not useAppStore: ProjectSettings used to resolve the
// project name via `useAppStore(s => s.projects as OrcaProject[])` — an
// unsafe cast onto the legacy RepoSlice's own unrelated `projects` field,
// which never actually matched. WorkspaceContext.project is the real
// OrcaProject switchProject fetched.
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: { id: 'p1', name: 'Test Project 1' }
  })
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {}, currentUser: { id: 'u1' } }) }
}))

vi.mock('../MemberManager', () => ({
  MemberManager: () => <div data-testid="member-manager">MemberManager Content</div>
}))

vi.mock('../RepoMemberManager', () => ({
  RepoMemberManager: ({ repoId }: { repoId: string }) => (
    <div data-testid="repo-member-manager">RepoMemberManager for {repoId}</div>
  )
}))

vi.mock('../LinkedProjectsManager', () => ({
  LinkedProjectsManager: ({
    orcaProjectId,
    currentUserRole
  }: {
    orcaProjectId: string
    currentUserRole: string | null
  }) => (
    <div data-testid="linked-projects-manager">
      LinkedProjectsManager for {orcaProjectId} (role: {currentUserRole ?? 'none'})
    </div>
  )
}))

vi.mock('../ProjectDevServerSection', () => ({
  ProjectDevServerSection: ({ projectId }: { projectId: string }) => (
    <div data-testid="project-dev-server-section">ProjectDevServerSection for {projectId}</div>
  )
}))

vi.mock('../ProjectDevServerFilterSection', () => ({
  ProjectDevServerFilterSection: () => <div data-testid="project-dev-server-filter-section" />
}))

vi.mock('../ProjectRepoCandidatesSection', () => ({
  ProjectRepoCandidatesSection: ({ projectId }: { projectId: string }) => (
    <div data-testid="project-repo-candidates-section">
      ProjectRepoCandidatesSection for {projectId}
    </div>
  )
}))

vi.mock('../../ui/dialog', () => ({
  Dialog: (p: { onOpenChange: MouseEventHandler; children: ReactNode }) => (
    <div data-testid="dialog" onClick={p.onOpenChange}>
      {p.children}
    </div>
  ),
  DialogContent: (p: { children: ReactNode }) => (
    <div data-testid="project-settings-dialog">{p.children}</div>
  ),
  DialogHeader: (p: { children: ReactNode }) => <div>{p.children}</div>,
  DialogTitle: (p: { children: ReactNode }) => <h2>{p.children}</h2>
}))

// All three TabsContent instances render their real children unconditionally
// (no activeTab tracking) — tests query by testid, so this is enough
// without reimplementing Radix Tabs' actual show/hide behavior.
vi.mock('../../ui/tabs', () => ({
  Tabs: (p: { children: ReactNode }) => <div data-testid="tabs">{p.children}</div>,
  TabsList: (p: { children: ReactNode }) => <div>{p.children}</div>,
  TabsTrigger: (p: { children: ReactNode; 'data-testid'?: string }) => (
    <button data-testid={p['data-testid']}>{p.children}</button>
  ),
  TabsContent: (p: { children: ReactNode }) => <div>{p.children}</div>
}))

describe('ProjectSettings', () => {
  beforeEach(() => {
    // Default: no repos and no members, so tests that don't care about the
    // Repos/Linked-Projects tabs' fetches (both mounted unconditionally on
    // open) don't need their own mock setup.
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'repo.list') {
        return { repos: [] }
      }
      if (method === 'project.getMembers') {
        return []
      }
      return null
    })
  })
  afterEach(cleanup)

  it('renders dialog with "General", "Members", "Repos", and "Linked Projects" tabs', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByTestId('tab-general')).toBeInTheDocument()
    expect(screen.getByTestId('tab-members')).toBeInTheDocument()
    expect(screen.getByTestId('tab-repos')).toBeInTheDocument()
    expect(screen.getByTestId('tab-linked')).toBeInTheDocument()
  })

  // BUG-FE-PW-002 fix: the Linked Projects tab needs to know the viewer's own
  // role. There is no RPC returning "my own membership row" directly, so this
  // resolves it via project.getMembers + filtering by the signed-in user id.
  it('resolves currentUserRole via project.getMembers and passes it to LinkedProjectsManager', async () => {
    vi.mocked(callRuntimeRpc).mockImplementation(async (_target, method) => {
      if (method === 'repo.list') {
        return { repos: [] }
      }
      if (method === 'project.getMembers') {
        return [
          { userId: 'u1', role: 'owner' },
          { userId: 'u2', role: 'member' }
        ]
      }
      return null
    })
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)

    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'project.getMembers', {
        projectId: 'p1'
      })
    })
    await waitFor(() => {
      expect(screen.getByTestId('linked-projects-manager')).toHaveTextContent(
        'for p1 (role: owner)'
      )
    })
  })

  it('LinkedProjectsManager gets role "none" when the current user has no membership row', async () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(screen.getByTestId('linked-projects-manager')).toHaveTextContent('(role: none)')
    })
  })

  it('project name appears in dialog title', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByText('Project Settings — Test Project 1')).toBeInTheDocument()
  })

  it('closes when onClose called', () => {
    const onClose = vi.fn()
    render(<ProjectSettings projectId="p1" open={true} onClose={onClose} />)
    fireEvent.click(screen.getByTestId('dialog'))
    expect(onClose).toHaveBeenCalled()
  })

  it('Members tab renders MemberManager', () => {
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    expect(screen.getByTestId('member-manager')).toBeInTheDocument()
  })

  // Regression guard: RepoMemberManager/ProjectSettings' Repos tab existed
  // (as of this same change) but this is the first test asserting the
  // repo-picker -> RepoMemberManager wiring actually works end-to-end.
  it('Repos tab lists project repos via repo.list, scoped to projectId', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue({
      repos: [{ id: 'r1', displayName: 'my-repo', url: 'https://x' }]
    })
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.list', {
        projectId: 'p1'
      })
    })
    await waitFor(() => {
      expect(screen.getByTestId('repo-picker-item-r1')).toBeInTheDocument()
    })
  })

  it('selecting a repo in the picker renders RepoMemberManager for it', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue({
      repos: [{ id: 'r1', displayName: 'my-repo', url: 'https://x' }]
    })
    render(<ProjectSettings projectId="p1" open={true} onClose={vi.fn()} />)
    await waitFor(() => expect(screen.getByTestId('repo-picker-item-r1')).toBeInTheDocument())

    expect(screen.queryByTestId('repo-member-manager')).not.toBeInTheDocument()
    fireEvent.click(screen.getByTestId('repo-picker-item-r1'))

    expect(screen.getByTestId('repo-member-manager')).toHaveTextContent('r1')
  })
})
