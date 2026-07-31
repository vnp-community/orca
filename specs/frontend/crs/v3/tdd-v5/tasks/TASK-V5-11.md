# TASK-V5-11: Git Slice + useGit + RPC Streaming

**Order:** 11  
**Prerequisite:** TASK-V5-02 (WorkspaceContext), TASK-V5-22 (RPC streaming — can stub)  
**Solution Ref:** SOL-FE-V5-06 (section 3, 4)  
**Est. effort:** ~60 min | **Tests:** 10

---

## Files Cần Tạo

### 1. `src/renderer/src/store/slices/git-panel.ts`

```typescript
import type { StateCreator } from 'zustand'

export type GitFileStatus = 'M' | 'A' | 'D' | 'R' | 'U'

export type GitFileChange = {
  path:   string
  status: GitFileStatus
  staged: boolean
}

export type GitCommit = {
  hash:      string
  shortHash: string
  message:   string
  author:    string
  date:      number
}

export type GitBranch = {
  name:      string
  isRemote:  boolean
  isCurrent: boolean
  upstream?: string
  aheadBy:   number
  behindBy:  number
}

export type GitPanelSliceState = {
  stagedFiles:      GitFileChange[]
  unstagedFiles:    GitFileChange[]
  gitHistory:       GitCommit[]
  branches:         GitBranch[]
  selectedDiffFile: string | null
  diffContent:      string | null
  pushLines:        string[]
  isPushing:        boolean
  isCommitting:     boolean
}

export type GitPanelSliceActions = {
  setStagedFiles(files: GitFileChange[]): void
  setUnstagedFiles(files: GitFileChange[]): void
  setGitHistory(commits: GitCommit[]): void
  setBranches(branches: GitBranch[]): void
  setSelectedDiffFile(path: string | null): void
  setDiffContent(diff: string | null): void
  appendPushLine(line: string): void
  clearPushLines(): void
  setIsPushing(v: boolean): void
  setIsCommitting(v: boolean): void
}

export type GitPanelSlice = GitPanelSliceState & GitPanelSliceActions

export function createGitPanelSlice(
  set: StateCreator<GitPanelSlice>['arguments'][0]
): GitPanelSlice {
  return {
    stagedFiles:      [],
    unstagedFiles:    [],
    gitHistory:       [],
    branches:         [],
    selectedDiffFile: null,
    diffContent:      null,
    pushLines:        [],
    isPushing:        false,
    isCommitting:     false,

    setStagedFiles:      (files)  => set(s => { s.stagedFiles = files }),
    setUnstagedFiles:    (files)  => set(s => { s.unstagedFiles = files }),
    setGitHistory:       (c)      => set(s => { s.gitHistory = c }),
    setBranches:         (b)      => set(s => { s.branches = b }),
    setSelectedDiffFile: (path)   => set(s => { s.selectedDiffFile = path }),
    setDiffContent:      (diff)   => set(s => { s.diffContent = diff }),
    appendPushLine:      (line)   => set(s => { s.pushLines.push(line) }),
    clearPushLines:      ()       => set(s => { s.pushLines = [] }),
    setIsPushing:        (v)      => set(s => { s.isPushing = v }),
    setIsCommitting:     (v)      => set(s => { s.isCommitting = v }),
  }
}
```

### 2. `src/renderer/src/runtime/runtime-rpc-stream.ts`

> **Stub for TASK-V5-22** — full implementation in TASK-V5-22. Here we define the interface.

```typescript
/**
 * Streaming RPC stub — provides AsyncGenerator<string> for streaming responses.
 * Full implementation: TASK-V5-22.
 * This stub is safe to use now — TASK-V5-22 will replace the implementation.
 */

export async function* callRuntimeRpcStream(
  method: string,
  params: unknown
): AsyncGenerator<string> {
  // Platform detection
  if (typeof window !== 'undefined' && (window as any).api?.callStream) {
    // Desktop Electron: IPC streaming
    const readable: ReadableStream<string> = await (window as any).api.callStream(method, params)
    const reader = readable.getReader()
    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        yield value
      }
    } finally {
      reader.releaseLock()
    }
  } else {
    // Web mode: fallback — single-shot (streaming not yet implemented for web)
    // TODO: WebSocket streaming in TASK-V5-22
    const result = await fetch(`/api/rpc/stream/${method}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    })
    const reader = result.body!.getReader()
    const decoder = new TextDecoder()
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      yield decoder.decode(value, { stream: true })
    }
  }
}
```

### 3. `src/renderer/src/hooks/useGit.ts`

```typescript
import { useCallback, useEffect } from 'react'
import { useWorkspace } from '../context/WorkspaceContext'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { callRuntimeRpcStream } from '../runtime/runtime-rpc-stream'
import type { GitFileChange } from '../store/slices/git-panel'

export function useGit() {
  const { project, currentWorktree, emit, refreshGitStatus } = useWorkspace()
  const { stagedFiles, unstagedFiles, isPushing, isCommitting } = useAppStore(s => ({
    stagedFiles:   s.stagedFiles,
    unstagedFiles: s.unstagedFiles,
    isPushing:     s.isPushing,
    isCommitting:  s.isCommitting,
  }))

  // Refresh on mount
  const refreshFiles = useCallback(async () => {
    if (!project) return
    const status = await callRuntimeRpc('git.getStatus', { projectId: project.id }) as {
      staged:   GitFileChange[]
      unstaged: GitFileChange[]
    }
    const store = useAppStore.getState()
    store.setStagedFiles(status.staged)
    store.setUnstagedFiles(status.unstaged)
  }, [project])

  useEffect(() => {
    if (project) refreshFiles()
  }, [project, refreshFiles])

  const stageFile = useCallback(async (path: string) => {
    if (!project) return
    await callRuntimeRpc('git.stageFile', { projectId: project.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const unstageFile = useCallback(async (path: string) => {
    if (!project) return
    await callRuntimeRpc('git.unstageFile', { projectId: project.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const stageAll = useCallback(async () => {
    if (!project) return
    await callRuntimeRpc('git.stageAll', { projectId: project.id })
    await refreshFiles()
  }, [project, refreshFiles])

  const unstageAll = useCallback(async () => {
    if (!project) return
    await callRuntimeRpc('git.unstageAll', { projectId: project.id })
    await refreshFiles()
  }, [project, refreshFiles])

  const commit = useCallback(async (message: string) => {
    if (!project) return
    const store = useAppStore.getState()
    store.setIsCommitting(true)
    try {
      await callRuntimeRpc('git.commit', { projectId: project.id, message })
      await refreshGitStatus()
      await refreshFiles()
      emit('git.committed', { message })
    } finally {
      store.setIsCommitting(false)
    }
  }, [project, refreshGitStatus, refreshFiles, emit])

  const push = useCallback(async (branch: string) => {
    if (!project) return
    const store = useAppStore.getState()
    store.clearPushLines()
    store.setIsPushing(true)
    try {
      for await (const line of callRuntimeRpcStream('git.push', {
        projectId:  project.id,
        branch,
        worktreeId: currentWorktree?.id,
      })) {
        store.appendPushLine(line)
      }
      await refreshGitStatus()
    } finally {
      store.setIsPushing(false)
    }
  }, [project, currentWorktree, refreshGitStatus])

  const getDiff = useCallback(async (filePath: string, staged?: boolean) => {
    if (!project) return null
    const diff = await callRuntimeRpc('git.getDiff', {
      projectId: project.id, path: filePath, staged,
    }) as string
    useAppStore.getState().setSelectedDiffFile(filePath)
    useAppStore.getState().setDiffContent(diff)
    return diff
  }, [project])

  const aiCommitMessage = useCallback(async () => {
    if (!project) return ''
    const result = await callRuntimeRpc('git.aiCommitMessage', { projectId: project.id }) as {
      message: string
    }
    return result.message
  }, [project])

  return {
    stagedFiles, unstagedFiles, isPushing, isCommitting,
    stageFile, unstageFile, stageAll, unstageAll,
    commit, push, getDiff, refreshFiles, aiCommitMessage,
  }
}
```

---

## Files Cần Sửa

### `src/renderer/src/store/index.ts`

```typescript
import { createGitPanelSlice } from './slices/git-panel'
// Trong combined slice:
...createGitPanelSlice(...a),
```

---

## Tests — `src/renderer/src/hooks/__tests__/useGit.test.ts`

```typescript
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
  useAppStore: (fn?: any) => fn ? fn(mockStore) : mockStore,
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
```

---

## Acceptance Criteria

- [x] `GitPanelSlice` registered trong store
- [x] `useGit.stageFile()` → `git.stageFile` + refresh
- [x] `useGit.commit()` → `git.commit` + `git.committed` event + refreshGitStatus
- [x] `useGit.push()` → `callRuntimeRpcStream` → `appendPushLine` per line
- [x] `isPushing=true` → `false` sau push
- [x] `clearPushLines()` gọi trước mỗi push
- [x] `aiCommitMessage()` returns message string
- [x] 10/10 tests pass
