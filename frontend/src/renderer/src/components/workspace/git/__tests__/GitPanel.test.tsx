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

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    vi.fn(selector => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

// Mock child components
vi.mock('../StagingArea', () => ({ StagingArea: () => <div data-testid="staging-area" /> }))
vi.mock('../CommitForm', () => ({ CommitForm: () => <div data-testid="commit-form" /> }))
vi.mock('../DiffViewer', () => ({ DiffViewer: () => <div data-testid="diff-viewer" /> }))
vi.mock('../GitHistory', () => ({ GitHistory: () => <div data-testid="git-history" /> }))
vi.mock('../BranchManager', () => ({ BranchManager: () => <div data-testid="branch-manager" /> }))
vi.mock('../PullRequestList', () => ({ PullRequestList: () => <div data-testid="pull-request-list" /> }))

describe('GitPanel', () => {
  const refreshGitStatus = vi.fn()
  const emit = vi.fn()

  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
      gitStatus: { branch: 'feat/test', aheadBy: 2, behindBy: 0 },
      refreshGitStatus,
      emit,
    } as any)
    vi.mocked(useGit).mockReturnValue({
      getDiff: vi.fn(),
    } as any)
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

  it('clicking Sync calls git.push RPC', async () => {
    const { callRuntimeRpc } = await import('@/runtime/runtime-rpc-client')
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce({})
    render(<GitPanel />)
    fireEvent.click(screen.getByTestId('sync-button'))
    
    expect(callRuntimeRpc).toHaveBeenCalledWith('mock-target', 'git.push', {
      projectId: 'p1',
      branch: 'feat/test',
      remote: 'origin'
    })
    
    await waitFor(() => {
      expect(refreshGitStatus).toHaveBeenCalled()
      expect(emit).toHaveBeenCalledWith('git.push', { branch: 'feat/test' })
    })
  })

  it('isPushing=true shows Loader2 on sync button', async () => {
    const { callRuntimeRpc } = await import('@/runtime/runtime-rpc-client')
    // Make the RPC hang so it stays in isPushing=true state
    vi.mocked(callRuntimeRpc).mockReturnValue(new Promise(() => {}))
    render(<GitPanel />)
    const btn = screen.getByTestId('sync-button')
    fireEvent.click(btn)
    // After click, it should be disabled and show the spinner
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
})
