import { beforeEach, describe, expect, it, vi } from 'vitest'

type FakeState = {
  activeView: string
  repos: { id: string }[]
  settings: Record<string, unknown>
  known: Set<string>
  getKnownWorktreeById: (id: string) => unknown
  fetchRepos: ReturnType<typeof vi.fn>
  fetchWorktrees: ReturnType<typeof vi.fn>
  fetchAllWorktrees: ReturnType<typeof vi.fn>
  setActiveWorkspaceTab: ReturnType<typeof vi.fn>
  setRightSidebarTab: ReturnType<typeof vi.fn>
  setRightSidebarOpen: ReturnType<typeof vi.fn>
}

const h = vi.hoisted(() => ({
  state: {} as FakeState,
  activate: vi.fn(),
  rpc: vi.fn()
}))

vi.mock('../../store', () => ({ useAppStore: { getState: () => h.state } }))
vi.mock('../../lib/worktree-activation', () => ({ activateAndRevealWorktree: h.activate }))
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: h.rpc,
  getActiveRuntimeTarget: () => 'target'
}))

import { openTaskWorktreeInGit } from './open-task-worktree-in-git'

beforeEach(() => {
  const known = new Set<string>()
  h.state = {
    activeView: 'requests',
    repos: [{ id: 'repo-1' }],
    settings: {},
    known,
    getKnownWorktreeById: (id) => (known.has(id) ? { id } : undefined),
    fetchRepos: vi.fn().mockResolvedValue(undefined),
    fetchWorktrees: vi.fn().mockImplementation(async () => void known.add('wt-1')),
    fetchAllWorktrees: vi.fn().mockImplementation(async () => void known.add('wt-1')),
    setActiveWorkspaceTab: vi.fn(),
    setRightSidebarTab: vi.fn(),
    setRightSidebarOpen: vi.fn()
  }
  h.activate.mockReset().mockReturnValue({ primaryTabId: null })
  h.rpc.mockReset().mockResolvedValue({ worktrees: [{ id: 'wt-1', repoId: 'repo-1' }] })
})

describe('openTaskWorktreeInGit', () => {
  it('a worktree the sidebar already knows is activated with Source Control open, with no lookup', async () => {
    h.state.known.add('wt-1')
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('opened')
    expect(h.rpc).not.toHaveBeenCalled()
    expect(h.activate).toHaveBeenCalledWith('wt-1')
    expect(h.state.setRightSidebarTab).toHaveBeenCalledWith('source-control')
    expect(h.state.setRightSidebarOpen).toHaveBeenCalledWith(true)
    expect(h.state.setActiveWorkspaceTab).not.toHaveBeenCalled()
  })

  it('a worktree that is not loaded yet is looked up by project, then its repo is refreshed', async () => {
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('opened')
    expect(h.rpc).toHaveBeenCalledWith('target', 'worktree.list', { projectId: 'p1' })
    expect(h.state.fetchRepos).not.toHaveBeenCalled()
    expect(h.state.fetchWorktrees).toHaveBeenCalledWith('repo-1')
    expect(h.activate).toHaveBeenCalledWith('wt-1')
  })

  it('loads the repo catalog first when the worktree belongs to a repo the store has not seen', async () => {
    h.state.repos = []
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('opened')
    expect(h.state.fetchRepos).toHaveBeenCalledTimes(1)
    expect(h.state.fetchWorktrees).toHaveBeenCalledWith('repo-1')
  })

  it('refreshes every repo when the backend row names no repo', async () => {
    h.rpc.mockResolvedValue({ worktrees: [{ id: 'wt-1' }] })
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('opened')
    expect(h.state.fetchAllWorktrees).toHaveBeenCalledTimes(1)
    expect(h.state.fetchWorktrees).not.toHaveBeenCalled()
  })

  it('"unknown" when the backend has no such worktree; nothing navigates', async () => {
    h.rpc.mockResolvedValue({ worktrees: [{ id: 'other', repoId: 'repo-1' }] })
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('unknown')
    expect(h.activate).not.toHaveBeenCalled()
    expect(h.state.setRightSidebarOpen).not.toHaveBeenCalled()
  })

  it('"not-on-host" when the row exists but the host does not list the checkout (cleaned up)', async () => {
    h.state.fetchWorktrees.mockResolvedValue(undefined) // the host reports nothing new
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('not-on-host')
    expect(h.activate).not.toHaveBeenCalled()
    expect(h.state.setRightSidebarOpen).not.toHaveBeenCalled()
  })

  it('"not-on-host" when activation refuses the worktree', async () => {
    h.state.known.add('wt-1')
    h.activate.mockReturnValue(false)
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('not-on-host')
    expect(h.state.setRightSidebarOpen).not.toHaveBeenCalled()
  })

  it('inside Project Workspace it stays there and selects the Git tab instead of the sidebar', async () => {
    h.state.known.add('wt-1')
    h.state.activeView = 'workspace'
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).resolves.toBe('opened')
    expect(h.state.setActiveWorkspaceTab).toHaveBeenCalledWith('git')
    expect(h.state.setRightSidebarTab).not.toHaveBeenCalled()
  })

  it('a failing lookup rejects so the caller can report it', async () => {
    h.rpc.mockRejectedValue(new Error('boom'))
    await expect(openTaskWorktreeInGit('p1', 'wt-1')).rejects.toThrow('boom')
  })
})
