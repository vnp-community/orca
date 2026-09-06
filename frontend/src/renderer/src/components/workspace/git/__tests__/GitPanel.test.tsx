// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { GitPanel } from '../GitPanel'
import { useWorkspace } from '../../../../context/WorkspaceContext'
import { useGit } from '../../../../hooks/useGit'

vi.mock('../../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('../../../../hooks/useGit', () => ({
  useGit: vi.fn()
}))

// Why: GitPanel now reads `state.repos` directly (CR-PW-002 repo label) — mock the selector
// hook the same way TaskCard.test.tsx does, so `useAppStore(selector)` calls `selector(mockStore)`.
const mockStore = { repos: [] as { id: string; displayName: string }[] }
vi.mock('../../../../store', () => ({
  useAppStore: vi.fn((selector) => selector(mockStore))
}))

// Mock child components
vi.mock('../StagingArea', () => ({ StagingArea: () => <div data-testid="staging-area" /> }))
vi.mock('../CommitForm', () => ({ CommitForm: () => <div data-testid="commit-form" /> }))
vi.mock('../DiffViewer', () => ({ DiffViewer: () => <div data-testid="diff-viewer" /> }))
vi.mock('../GitHistory', () => ({ GitHistory: () => <div data-testid="git-history" /> }))
vi.mock('../BranchManager', () => ({ BranchManager: () => <div data-testid="branch-manager" /> }))
vi.mock('../PullRequestList', () => ({
  PullRequestList: () => <div data-testid="pull-request-list" />
}))

describe('GitPanel', () => {
  const emit = vi.fn()
  const push = vi.fn().mockResolvedValue(undefined)
  const getDiff = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    mockStore.repos = []
    push.mockResolvedValue(undefined)
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      currentWorktree: {
        id: 'repo1::/repo/worktree',
        path: '/repo/worktree',
        branch: 'feat/test',
        isMain: false
      },
      gitStatus: { branch: 'feat/test', branchUnavailable: undefined, aheadBy: 2, behindBy: 0 },
      gitStatusError: false,
      emit
    } as unknown as ReturnType<typeof useWorkspace>)
    vi.mocked(useGit).mockReturnValue({
      getDiff,
      push,
      isPushing: false
    } as unknown as ReturnType<typeof useGit>)
  })

  it('renders 4 tabs: changes, history, branches, pullrequests', () => {
    render(<GitPanel />)
    expect(screen.getByTestId('git-tab-changes')).toBeInTheDocument()
    expect(screen.getByTestId('git-tab-history')).toBeInTheDocument()
    expect(screen.getByTestId('git-tab-branches')).toBeInTheDocument()
    expect(screen.getByTestId('git-tab-pullrequests')).toBeInTheDocument()
  })

  it('shows branch name from gitStatus.branch', () => {
    render(<GitPanel />)
    expect(screen.getByText('feat/test')).toBeInTheDocument()
    expect(screen.getByText('↑2 ↓0')).toBeInTheDocument() // Using unicode arrows from source: &uarr;2 &darr;0
  })

  it('sync-button is visible', () => {
    render(<GitPanel />)
    expect(screen.getByTestId('sync-button')).toBeInTheDocument()
  })

  // Why (crash reported by user): GitPanel used to build its own broken
  // callRuntimeRpc('git.push', {projectId, branch, remote}) call — it now
  // reuses useGit().push(), which sends the real {worktree, pushTarget}
  // request (FIX BUG-FE-HLD-002). This test asserts the reuse, not a raw RPC shape.
  it('clicking Sync calls useGit().push with the current branch', async () => {
    render(<GitPanel />)
    fireEvent.click(screen.getByTestId('sync-button'))

    expect(push).toHaveBeenCalledWith('feat/test')

    await waitFor(() => {
      expect(emit).toHaveBeenCalledWith('git.push', { branch: 'feat/test' })
    })
  })

  it('isPushing=true shows Loader2 on sync button and disables it', () => {
    vi.mocked(useGit).mockReturnValue({
      getDiff,
      push,
      isPushing: true
    } as unknown as ReturnType<typeof useGit>)
    render(<GitPanel />)
    const btn = screen.getByTestId('sync-button')
    expect(btn).toBeDisabled()
    expect(btn.querySelector('.animate-spin')).toBeInTheDocument()
  })

  it('switching to "pullrequests" tab renders PullRequestList', async () => {
    render(<GitPanel />)
    fireEvent.click(screen.getByTestId('git-tab-pullrequests'))
    await waitFor(() => {
      expect(screen.getByTestId('pull-request-list')).toBeInTheDocument()
    })
  })

  // CR-PW-001: "(no branch)" used to cover 3 unrelated states — these assert each is now labeled
  // distinctly instead of collapsing back into that one ambiguous string.
  describe('branch label states (CR-PW-001)', () => {
    it('shows "Detached HEAD" when branchUnavailable is detached-head', () => {
      vi.mocked(useWorkspace).mockReturnValue({
        project: { id: 'p1' },
        currentWorktree: { id: 'repo1::/repo/worktree', path: '/repo/worktree', isMain: false },
        gitStatus: {
          branch: undefined,
          branchUnavailable: 'detached-head',
          aheadBy: 0,
          behindBy: 0
        },
        gitStatusError: false,
        emit
      } as unknown as ReturnType<typeof useWorkspace>)
      render(<GitPanel />)
      expect(screen.getByTestId('git-panel-branch')).toHaveTextContent('Detached HEAD')
    })

    it('shows "Git unavailable" when branchUnavailable is status-unavailable', () => {
      vi.mocked(useWorkspace).mockReturnValue({
        project: { id: 'p1' },
        currentWorktree: { id: 'repo1::/repo/worktree', path: '/repo/worktree', isMain: false },
        gitStatus: {
          branch: undefined,
          branchUnavailable: 'status-unavailable',
          aheadBy: 0,
          behindBy: 0
        },
        gitStatusError: false,
        emit
      } as unknown as ReturnType<typeof useWorkspace>)
      render(<GitPanel />)
      expect(screen.getByTestId('git-panel-branch')).toHaveTextContent('Git unavailable')
    })

    it('shows "Git status unavailable" when the git.status RPC itself failed', () => {
      vi.mocked(useWorkspace).mockReturnValue({
        project: { id: 'p1' },
        currentWorktree: { id: 'repo1::/repo/worktree', path: '/repo/worktree', isMain: false },
        gitStatus: null,
        gitStatusError: true,
        emit
      } as unknown as ReturnType<typeof useWorkspace>)
      render(<GitPanel />)
      expect(screen.getByTestId('git-panel-branch')).toHaveTextContent('Git status unavailable')
    })
  })

  // CR-PW-002: label which repo the branch belongs to when a project has more than one.
  it('shows the repo display name next to the branch when the worktree resolves to a known repo', () => {
    mockStore.repos = [{ id: 'repo1', displayName: 'vnp-asm' }]
    render(<GitPanel />)
    expect(screen.getByTestId('git-panel-repo-label')).toHaveTextContent('vnp-asm')
  })

  it('omits the repo label when the worktree does not resolve to a known repo', () => {
    mockStore.repos = []
    render(<GitPanel />)
    expect(screen.queryByTestId('git-panel-repo-label')).not.toBeInTheDocument()
  })
})
