// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { PullRequestList } from '../PullRequestList'
import { useWorkspace } from '../../../../context/WorkspaceContext'

vi.mock('../../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    vi.fn(selector => selector({ settings: {} })),
    { getState: () => ({ settings: {} }) }
  )
}))

vi.mock('@/runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))

import { callRuntimeRpc } from '@/runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

describe('PullRequestList', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.mocked(useWorkspace).mockReturnValue({
      project: { id: 'p1' },
    } as any)
  })

  it('fetches PRs on mount via git.pr.list', async () => {
    mockRpc.mockResolvedValueOnce([])
    render(<PullRequestList />)
    expect(screen.getByTestId('pr-loading')).toBeInTheDocument()
    
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith('mock-target', 'git.pr.list', { projectId: 'p1', state: 'open' })
      expect(screen.queryByTestId('pr-loading')).not.toBeInTheDocument()
    })
  })

  it('renders PR items with title and number', async () => {
    mockRpc.mockResolvedValueOnce([
      { number: 42, title: 'Add new feature', state: 'open', url: 'https://github.com/foo/bar/pull/42', author: 'alice', baseBranch: 'main', headBranch: 'feature' }
    ])
    render(<PullRequestList />)
    
    await waitFor(() => {
      expect(screen.getByText('Add new feature')).toBeInTheDocument()
      expect(screen.getByText('#42 · alice · feature → main')).toBeInTheDocument()
    })
  })

  it('empty list → shows empty state icon + text', async () => {
    mockRpc.mockResolvedValueOnce([])
    render(<PullRequestList />)
    
    await waitFor(() => {
      expect(screen.getByTestId('pr-empty')).toBeInTheDocument()
      expect(screen.getByText('No open pull requests')).toBeInTheDocument()
    })
  })

  it('external link has target="_blank" and correct href', async () => {
    mockRpc.mockResolvedValueOnce([
      { number: 99, title: 'Fix bug', state: 'open', url: 'https://github.com/foo/bar/pull/99', author: 'bob', baseBranch: 'main', headBranch: 'bugfix' }
    ])
    render(<PullRequestList />)
    
    await waitFor(() => {
      const link = screen.getByTestId('pr-link-99')
      expect(link).toHaveAttribute('href', 'https://github.com/foo/bar/pull/99')
      expect(link).toHaveAttribute('target', '_blank')
    })
  })

  it('refresh button re-fetches without showing loading skeleton', async () => {
    mockRpc.mockResolvedValueOnce([]) // Mount
    render(<PullRequestList />)
    
    await waitFor(() => {
      expect(screen.queryByTestId('pr-loading')).not.toBeInTheDocument()
    })
    
    mockRpc.mockResolvedValueOnce([]) // Refresh
    const refreshBtn = screen.getByTestId('pr-refresh')
    fireEvent.click(refreshBtn)
    
    // Should NOT show pr-loading div during refresh
    expect(screen.queryByTestId('pr-loading')).not.toBeInTheDocument()
    
    // Should show spinner icon inside button
    expect(refreshBtn.querySelector('.animate-spin')).toBeInTheDocument()
    
    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledTimes(2)
      expect(refreshBtn.querySelector('.animate-spin')).not.toBeInTheDocument()
    })
  })
})
