// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ kind: 'local' })
}))
// FIX BUG-FE-HLD-002: push() now calls pushRuntimeGit() (real, authenticated
// transport) instead of the removed callRuntimeRpcStream()/runtime-rpc-stream.ts.
vi.mock('../../runtime/runtime-git-client', () => ({
  pushRuntimeGit: vi.fn().mockResolvedValue(undefined)
}))

const emit             = vi.fn()
const refreshGitStatus = vi.fn()
const mockStore = {
  stagedFiles: [], unstagedFiles: [], isPushing: false, isCommitting: false, pushLines: [],
  settings: null,
  setStagedFiles:    vi.fn(), setUnstagedFiles: vi.fn(),
  setIsCommitting:   vi.fn(v => { mockStore.isCommitting = v }),
  setIsPushing:      vi.fn(v => { mockStore.isPushing = v }),
  appendPushLine:    vi.fn(l => { mockStore.pushLines.push(l) }),
  clearPushLines:    vi.fn(() => { mockStore.pushLines = [] }),
  setSelectedDiffFile: vi.fn(), setDiffContent: vi.fn(),
}
vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: any) => fn ? fn(mockStore) : mockStore,
    { getState: () => mockStore }
  ),
}))
vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({
    project:          { id: 'p1', name: 'myapp' },
    // Why: push() (BUG-FE-HLD-002 fix) requires a non-null currentWorktree to
    // resolve a worktreePath — the other hooks in this file don't touch it.
    currentWorktree:  { id: 'repo1::/repo/worktree' },
    emit,
    refreshGitStatus,
  }),
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { pushRuntimeGit } from '../../runtime/runtime-git-client'
const mockRpc = vi.mocked(callRuntimeRpc)
const mockPushRuntimeGit = vi.mocked(pushRuntimeGit)

describe('useGit', () => {
  beforeEach(() => vi.clearAllMocks())

  it('stageFile calls git.stageFile and refreshes', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.stageFile('src/index.ts') })
    expect(mockRpc).toHaveBeenCalledWith('git.stageFile', { projectId: 'p1', path: 'src/index.ts' })
  })

  it('unstageFile calls git.unstageFile', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.unstageFile('src/auth.ts') })
    expect(mockRpc).toHaveBeenCalledWith('git.unstageFile', { projectId: 'p1', path: 'src/auth.ts' })
  })

  it('commit calls git.commit and emits git.committed', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.commit('feat: add auth') })
    expect(mockRpc).toHaveBeenCalledWith('git.commit', { projectId: 'p1', message: 'feat: add auth' })
    expect(emit).toHaveBeenCalledWith('git.committed', { message: 'feat: add auth' })
  })

  it('commit sets isCommitting true then false', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.commit('fix: bug') })
    expect(mockStore.setIsCommitting).toHaveBeenCalledWith(true)
    expect(mockStore.setIsCommitting).toHaveBeenCalledWith(false)
  })

  it('push: calls pushRuntimeGit with the resolved worktree context and pushTarget', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.push('main') })
    expect(mockPushRuntimeGit).toHaveBeenCalledWith(
      { settings: null, worktreeId: 'repo1::/repo/worktree', worktreePath: '/repo/worktree' },
      { pushTarget: { remoteName: 'origin', branchName: 'main' } }
    )
    expect(mockStore.appendPushLine).toHaveBeenCalledWith('Pushed main')
  })

  it('push: isPushing=true during push, false after', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.push('main') })
    expect(mockStore.setIsPushing).toHaveBeenCalledWith(true)
    expect(mockStore.setIsPushing).toHaveBeenCalledWith(false)
  })

  it('push: clearPushLines called before start', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.push('main') })
    expect(mockStore.clearPushLines).toHaveBeenCalled()
  })

  it('aiCommitMessage returns message string', async () => {
    mockRpc.mockResolvedValueOnce({ staged: [], unstaged: [] })  // mount
    mockRpc.mockResolvedValueOnce({ message: 'feat: add JWT auth' })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {})
    const msg = await result.current.aiCommitMessage()
    expect(msg).toBe('feat: add JWT auth')
  })

  it('refreshFiles on mount when project set', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    renderHook(() => useGit())
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith('git.getStatus', { projectId: 'p1' })
  })
})
