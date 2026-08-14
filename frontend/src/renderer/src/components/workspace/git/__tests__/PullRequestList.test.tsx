// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { PullRequestList } from '../PullRequestList'
import { useWorkspace } from '../../../../context/WorkspaceContext'

vi.mock('../../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn()
}))

// Why: 'git.pr.list' has never existed as an RPC method
// (backend/src/main/runtime/rpc/methods/git.ts has no git.pr.* group) — this
// panel used to call it unconditionally and crash. It now shows an honest
// "not available" state instead — see PullRequestList.tsx's file header.
describe('PullRequestList', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('no project selected → shows the no-project state', () => {
    vi.mocked(useWorkspace).mockReturnValue({ project: null } as unknown as ReturnType<
      typeof useWorkspace
    >)
    render(<PullRequestList />)
    expect(screen.getByTestId('pr-no-project')).toBeInTheDocument()
  })

  it('project selected → shows the unavailable state, not a broken PR list', () => {
    vi.mocked(useWorkspace).mockReturnValue({ project: { id: 'p1' } } as unknown as ReturnType<
      typeof useWorkspace
    >)
    render(<PullRequestList />)
    expect(screen.getByTestId('pr-unavailable')).toBeInTheDocument()
    expect(screen.getByText(/Pull requests are not available/)).toBeInTheDocument()
  })
})
