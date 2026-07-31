// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'

vi.mock('../../runtime/runtime-rpc-client', () => ({ callRuntimeRpc: vi.fn() }))
vi.mock('../../runtime/runtime-rpc-stream', () => ({
  callRuntimeRpcStream: vi.fn(async function*() {
    yield 'Counting objects: 3, done.'
    yield 'Writing objects: 100% done.'
  }),
}))

const emit             = vi.fn()
const refreshGitStatus = vi.fn()
const mockStore = {
  stagedFiles: [], unstagedFiles: [], isPushing: false, isCommitting: false, pushLines: [],
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
    currentWorktree:  null,
    emit,
    refreshGitStatus,
  }),
}))

import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)

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

  it('push: streams lines via callRuntimeRpcStream', async () => {
    mockRpc.mockResolvedValue({ staged: [], unstaged: [] })
    const { useGit } = await import('../useGit')
    const { result } = renderHook(() => useGit())
    await act(async () => { await result.current.push('main') })
    expect(mockStore.appendPushLine).toHaveBeenCalledWith('Counting objects: 3, done.')
    expect(mockStore.appendPushLine).toHaveBeenCalledWith('Writing objects: 100% done.')
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
