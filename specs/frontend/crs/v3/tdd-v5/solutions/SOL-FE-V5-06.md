# SOL-FE-V5-06: Remote Git UI

**TDD Ref:** [TDD-FE-16](../../../tdd/16-remote-git-ui.md)  
**Feature:** F39 | **ADR:** ADR-012 | **HLD:** C3.12, C4.10  
**Status:** ✅ DONE — Implemented via TASK-V5-11, TASK-V5-12, TASK-V5-13, TASK-V5-22  
**Dependency:** WorkspaceContext (SOL-FE-V5-02), rpc.callStream()

---

## 1. Files Cần Tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/git-panel.ts` | Zustand Slice | stagedFiles, gitHistory, branches, PRs |
| `src/renderer/src/components/workspace/git/GitPanel.tsx` | Component | Main git panel với 4 tabs |
| `src/renderer/src/components/workspace/git/StagingArea.tsx` | Component | Staged/Unstaged file lists |
| `src/renderer/src/components/workspace/git/DiffViewer.tsx` | Component | Unified/side-by-side diff |
| `src/renderer/src/components/workspace/git/CommitForm.tsx` | Component | Commit message + AI assist |
| `src/renderer/src/components/workspace/git/BranchManager.tsx` | Component | Branch list + CRUD |
| `src/renderer/src/components/workspace/git/PullRequestForm.tsx` | Component | PR creation form |
| `src/renderer/src/components/workspace/git/GitHistory.tsx` | Component | Commit log |
| `src/renderer/src/components/workspace/git/PushStreamPanel.tsx` | Component | Push output streaming |
| `src/renderer/src/hooks/useGit.ts` | Hook | Stage/unstage/commit/push ops |
| `src/renderer/src/hooks/useGitHistory.ts` | Hook | Fetch commit log + diff |
| `src/renderer/src/hooks/useGitBranches.ts` | Hook | List/create/delete branches |
| `src/renderer/src/hooks/usePullRequests.ts` | Hook | GitHub PR CRUD via `gh` CLI |

---

## 2. RPC Streaming Extension

```typescript
// src/renderer/src/runtime/runtime-rpc-client.ts — EXTEND (additive)
// Thêm callRuntimeRpcStream() cho streaming responses

export async function* callRuntimeRpcStream(
  method: string,
  params: unknown
): AsyncGenerator<string> {
  // Web mode: WebSocket JSON-RPC streaming
  // Desktop mode: IPC streaming

  if (ORCA_PLATFORM === 'web') {
    // WebSocket streaming via notifications
    yield* webRpcStream(method, params)
  } else {
    // Electron IPC streaming
    yield* electronRpcStream(method, params)
  }
}

// Hoặc expose qua window.api:
// window.api.callStream(method, params) → AsyncIterable<string>
```

---

## 3. Git Panel Slice

```typescript
// src/renderer/src/store/slices/git-panel.ts

export type GitFileChange = {
  path:   string
  status: 'M' | 'A' | 'D' | 'R' | 'U'  // Modified/Added/Deleted/Renamed/Untracked
  staged: boolean
}

export type GitCommit = {
  hash:      string
  message:   string
  author:    string
  date:      number
  shortHash: string
}

export type GitBranch = {
  name:     string
  isRemote: boolean
  isCurrent: boolean
  upstream?: string
  aheadBy:  number
  behindBy: number
}

export type GitPanelSlice = {
  stagedFiles:    GitFileChange[]
  unstagedFiles:  GitFileChange[]
  gitHistory:     GitCommit[]
  branches:       GitBranch[]
  selectedDiffFile: string | null
  diffContent:    string | null         // unified diff text
  pushLines:      string[]              // streaming push output
  isPushing:      boolean
  isCommitting:   boolean

  setStagedFiles(files: GitFileChange[]): void
  setUnstagedFiles(files: GitFileChange[]): void
  setGitHistory(commits: GitCommit[]): void
  setBranches(branches: GitBranch[]): void
  setSelectedDiffFile(path: string | null): void
  setDiffContent(diff: string | null): void
  appendPushLine(line: string): void
  clearPushLines(): void
  setIsPushing(v: boolean): void
}
```

---

## 4. useGit Hook

```typescript
// src/renderer/src/hooks/useGit.ts

export function useGit() {
  const { project, currentWorktree, emit, refreshGitStatus } = useWorkspace()
  const { stagedFiles, unstagedFiles } = useAppStore(s => ({
    stagedFiles:   s.stagedFiles,
    unstagedFiles: s.unstagedFiles,
  }))

  const stageFile = useCallback(async (path: string) => {
    await rpc('git.stageFile', { projectId: project!.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const unstageFile = useCallback(async (path: string) => {
    await rpc('git.unstageFile', { projectId: project!.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const stageAll = useCallback(async () => {
    await rpc('git.stageAll', { projectId: project!.id })
    await refreshFiles()
  }, [project, refreshFiles])

  const commit = useCallback(async (message: string) => {
    await rpc('git.commit', { projectId: project!.id, message })
    await refreshGitStatus()
    emit('git.committed', { message })
  }, [project, refreshGitStatus, emit])

  const push = useCallback(async (
    branch: string,
    onLine: (line: string) => void
  ) => {
    const store = useAppStore.getState()
    store.clearPushLines()
    store.setIsPushing(true)
    try {
      for await (const line of callRuntimeRpcStream('git.push', {
        projectId: project!.id,
        branch,
        worktreeId: currentWorktree?.id
      })) {
        store.appendPushLine(line)
        onLine(line)
      }
      await refreshGitStatus()
    } finally {
      store.setIsPushing(false)
    }
  }, [project, currentWorktree, refreshGitStatus])

  const refreshFiles = useCallback(async () => {
    const status = await rpc('git.getStatus', { projectId: project!.id }) as {
      staged: GitFileChange[]
      unstaged: GitFileChange[]
    }
    const store = useAppStore.getState()
    store.setStagedFiles(status.staged)
    store.setUnstagedFiles(status.unstaged)
  }, [project])

  const aiCommitMessage = useCallback(async () => {
    const result = await rpc('git.aiCommitMessage', {
      projectId: project!.id
    }) as { message: string }
    return result.message
  }, [project])

  // Fetch on mount
  useEffect(() => { if (project) refreshFiles() }, [project])

  return { stagedFiles, unstagedFiles, stageFile, unstageFile,
           stageAll, unstageAll: async () => rpc('git.unstageAll', { projectId: project!.id }),
           commit, push, refreshFiles, aiCommitMessage }
}
```

---

## 5. DiffViewer Component

```typescript
// src/renderer/src/components/workspace/git/DiffViewer.tsx
// Unified diff renderer — parse unified diff format into hunks

// Color mapping:
// + lines → bg-green-50 text-green-800
// - lines → bg-red-50 text-red-800
// @@ hunk headers → bg-gray-100 text-gray-500
// unchanged lines → white

// View modes:
// - 'unified' (default): sequential +/- lines
// - 'split': side-by-side left/right columns

// Lazy load: DiffViewer is heavy (large files)
export default function DiffViewer({ filePath, diff, mode = 'unified' }) { }
const DiffViewerLazy = lazy(() => import('./DiffViewer'))
```

---

## 6. PullRequestForm + usePullRequests

```typescript
// Creates GitHub PRs via `gh` CLI tool on dev server

export function usePullRequests() {
  const createPR = async (params: {
    title:  string
    body:   string
    base:   string
    head:   string
    draft:  boolean
  }) => {
    // Calls gh CLI via agent tool:
    // window.api.runAgentTool(devServerId, 'gh', ['pr', 'create', '--title', ...])
    const result = await rpc('git.createPR', params)
    toast.success('Pull request created!')
    return result
  }

  return { createPR }
}
```

---

## 7. Files Cần Sửa (Additive)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/index.ts` | Register `createGitPanelSlice` |
| `src/renderer/src/components/workspace/WorkspaceLayout.tsx` | Mount `<GitPanel />` trong git tab |
| `src/renderer/src/runtime/runtime-rpc-client.ts` | Thêm `callRuntimeRpcStream()` |

---

## 8. RPC Methods

| Method | Params | Return |
|--------|--------|--------|
| `git.getStatus` | `{ projectId }` | `{ staged, unstaged }` |
| `git.stageFile` | `{ projectId, path }` | `void` |
| `git.unstageFile` | `{ projectId, path }` | `void` |
| `git.stageAll` | `{ projectId }` | `void` |
| `git.unstageAll` | `{ projectId }` | `void` |
| `git.commit` | `{ projectId, message }` | `{ hash }` |
| `git.push` | `{ projectId, branch }` | `AsyncIterable<string>` (streaming) |
| `git.pull` | `{ projectId, branch }` | `AsyncIterable<string>` (streaming) |
| `git.getDiff` | `{ projectId, path, staged? }` | `string` (unified diff) |
| `git.getLog` | `{ projectId, limit?, branch? }` | `GitCommit[]` |
| `git.listBranches` | `{ projectId }` | `GitBranch[]` |
| `git.createBranch` | `{ projectId, name, from? }` | `void` |
| `git.checkout` | `{ projectId, branch }` | `void` |
| `git.deleteBranch` | `{ projectId, name, force? }` | `void` |
| `git.aiCommitMessage` | `{ projectId }` | `{ message }` |
| `git.createPR` | `{ title, body, base, head, draft }` | `{ url, number }` |

---

## 9. Test Plan

```
src/renderer/src/components/workspace/git/__tests__/
├── GitPanel.test.tsx              (5 tests)
│   ├── renders Changes tab by default
│   ├── renders staged files list
│   ├── renders unstaged files list
│   ├── switching to History tab shows GitHistory
│   └── switching to Branches shows BranchManager
├── StagingArea.test.tsx           (6 tests)
│   ├── stageFile called when Stage button clicked
│   ├── unstageFile called when Unstage button clicked
│   ├── stageAll stages all unstaged files
│   ├── unstageAll unstages all staged files
│   ├── shows file status badge (M/A/D)
│   └── click file → loads diff in DiffViewer
├── CommitForm.test.tsx            (5 tests)
│   ├── submit calls git.commit RPC
│   ├── empty message → submit disabled
│   ├── AI button calls git.aiCommitMessage and sets message
│   ├── "Commit & Push" calls commit then push
│   └── commit refreshes git status
├── DiffViewer.test.tsx            (4 tests)
│   ├── + lines → green background
│   ├── - lines → red background
│   ├── @@ hunk header → gray
│   └── empty diff → "No changes" text
└── hooks/__tests__/useGit.test.ts  (10 tests)
    ├── stageFile calls git.stageFile and refreshes
    ├── unstageFile calls git.unstageFile and refreshes
    ├── stageAll calls git.stageAll
    ├── commit calls git.commit and emits git.committed
    ├── commit refreshes git status
    ├── push: streams lines via callRuntimeRpcStream
    ├── push: isPushing=true during push
    ├── push: clearPushLines before start
    ├── aiCommitMessage returns message string
    └── fetchFiles on mount when project set
```

**Target:** ≥ 30 tests
