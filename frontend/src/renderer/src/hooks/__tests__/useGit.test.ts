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

const emit = vi.fn()
const refreshGitStatus = vi.fn()
const mockStore = {
  stagedFiles: [],
  unstagedFiles: [],
  isPushing: false,
  isCommitting: false,
  pushLines: [],
  settings: null,
  setStagedFiles: vi.fn(),
  setUnstagedFiles: vi.fn(),
  setIsCommitting: vi.fn((v) => {
    mockStore.isCommitting = v
  }),
  setIsPushing: vi.fn((v) => {
    mockStore.isPushing = v
  }),
  appendPushLine: vi.fn((l) => {
    mockStore.pushLines.push(l)
  }),
  clearPushLines: vi.fn(() => {
    mockStore.pushLines = []
  }),
  setSelectedDiffFile: vi.fn(),
  setDiffContent: vi.fn()
}
vi.mock('../../store', () => ({
  useAppStore: Object.assign(
    (fn?: (s: typeof mockStore) => unknown) => (fn ? fn(mockStore) : mockStore),
    { getState: () => mockStore }
  )
}))
// Why (BUG-FE-RPC crash fix): currentWorktree carries {id, path} — id feeds
// toRuntimeWorktreeSelector() for git.* RPC calls; the F38 WorkspaceContext
// worktree model (renderer/src/types/workspace-types.ts) is distinct from the
// runtime's Repo-scoped Worktree — see useGit.ts's file header.
vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({
    project: { id: 'p1', name: 'myapp' },
    currentWorktree: {
      id: 'repo1::/repo/worktree',
      path: '/repo/worktree',
      branch: 'main',
      isMain: true
    },
    emit,
    refreshGitStatus
  })
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { pushRuntimeGit } from '../../runtime/runtime-git-client'
const mockRpc = vi.mocked(callRuntimeRpc)
const mockPushRuntimeGit = vi.mocked(pushRuntimeGit)

describe('useGit', () => {
  beforeEach(() => vi.clearAllMocks())

  it('stageFile calls git.stage with a worktree selector', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.stageFile('src/index.ts')
    })
    expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'git.stage', {
      worktree: 'id:repo1::/repo/worktree',
      filePath: 'src/index.ts'
    })
  })

  it('unstageFile calls git.unstage with a worktree selector', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.unstageFile('src/auth.ts')
    })
    expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'git.unstage', {
      worktree: 'id:repo1::/repo/worktree',
      filePath: 'src/auth.ts'
    })
  })

  it('commit calls git.commit and emits git.committed', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.commit('feat: add auth')
    })
    expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'git.commit', {
      worktree: 'id:repo1::/repo/worktree',
      message: 'feat: add auth'
    })
    expect(emit).toHaveBeenCalledWith('git.committed', { message: 'feat: add auth' })
  })

  it('commit sets isCommitting true then false', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.commit('fix: bug')
    })
    expect(mockStore.setIsCommitting).toHaveBeenCalledWith(true)
    expect(mockStore.setIsCommitting).toHaveBeenCalledWith(false)
  })

  it('push: calls pushRuntimeGit with the resolved worktree context and pushTarget', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.push('main')
    })
    expect(mockPushRuntimeGit).toHaveBeenCalledWith(
      { settings: null, worktreeId: 'repo1::/repo/worktree', worktreePath: '/repo/worktree' },
      { pushTarget: { remoteName: 'origin', branchName: 'main' } }
    )
    expect(mockStore.appendPushLine).toHaveBeenCalledWith('Pushed main')
  })

  it('push: isPushing=true during push, false after', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.push('main')
    })
    expect(mockStore.setIsPushing).toHaveBeenCalledWith(true)
    expect(mockStore.setIsPushing).toHaveBeenCalledWith(false)
  })

  it('push: clearPushLines called before start', async () => {
    mockRpc.mockResolvedValue({ entries: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {
      await result.current.push('main')
    })
    expect(mockStore.clearPushLines).toHaveBeenCalled()
  })

  it('aiCommitMessage returns message string via git.generateCommitMessage', async () => {
    mockRpc.mockResolvedValueOnce({ entries: [] }) // mount (refreshFiles)
    mockRpc.mockResolvedValueOnce({ success: true, message: 'feat: add JWT auth' })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => {})
    const msg = await result.current.aiCommitMessage()
    expect(msg).toBe('feat: add JWT auth')
    expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'git.generateCommitMessage', {
      worktree: 'id:repo1::/repo/worktree'
    })
  })

  it('refreshFiles on mount splits git.status entries into staged/unstaged', async () => {
    mockRpc.mockResolvedValue({
      entries: [
        { path: 'a.ts', status: 'modified', area: 'staged' },
        { path: 'b.ts', status: 'untracked', area: 'unstaged' }
      ]
    })
    const { useGit } = await import('../useGit')
    renderHook(() => useGit())
    await act(async () => {})
    expect(mockRpc).toHaveBeenCalledWith({ kind: 'local' }, 'git.status', {
      worktree: 'id:repo1::/repo/worktree'
    })
    expect(mockStore.setStagedFiles).toHaveBeenCalledWith([
      { path: 'a.ts', status: 'M', staged: true }
    ])
    expect(mockStore.setUnstagedFiles).toHaveBeenCalledWith([
      { path: 'b.ts', status: 'U', staged: false }
    ])
  })
})
