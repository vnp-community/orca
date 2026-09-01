import { useCallback, useEffect } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { useWorkspace } from '../context/WorkspaceContext'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { pushRuntimeGit } from '../runtime/runtime-git-client'
import { toRuntimeWorktreeSelector } from '../runtime/runtime-worktree-selector'
import type { GitFileChange, GitFileStatus } from '../store/slices/git-panel'
import type {
  GitStatusResult,
  GitFileStatus as BackendGitFileStatus
} from '../../../shared/git-status-types'
import type { GitDiffResult } from '../../../shared/types'
import { Tracers } from '../../../shared/trace/tracers'
import { splitWorktreeIdForFilesystem } from '../../../shared/worktree-id'

// Why (crash reported by user, GitPanel.tsx): this hook used to call
// callRuntimeRpc(method, params) — the real signature is
// callRuntimeRpc(target, method, params) — and nonexistent method names
// ('git.getStatus', 'git.stageFile', ...) with a {projectId} shape instead of
// the real {worktree} selector every backend git.* handler requires
// (backend/src/main/runtime/rpc/methods/git.ts, git-params.ts). Mirrors the
// already-correct, already-shipped pattern from
// WorkspaceContext.refreshGitStatus() (F38 roadmap 2c.2): resolve the
// `worktree` selector from currentWorktree.id via toRuntimeWorktreeSelector.
const BACKEND_STATUS_TO_LETTER: Record<BackendGitFileStatus, GitFileStatus> = {
  modified: 'M',
  added: 'A',
  deleted: 'D',
  renamed: 'R',
  untracked: 'U',
  copied: 'A'
}

function toGitFileChanges(status: GitStatusResult): {
  staged: GitFileChange[]
  unstaged: GitFileChange[]
} {
  const staged: GitFileChange[] = []
  const unstaged: GitFileChange[] = []
  for (const entry of status.entries) {
    const file: GitFileChange = {
      path: entry.path,
      status: BACKEND_STATUS_TO_LETTER[entry.status] ?? 'M',
      staged: entry.area === 'staged'
    }
    if (entry.area === 'staged') {
      staged.push(file)
    } else {
      unstaged.push(file)
    }
  }
  return { staged, unstaged }
}

function diffResultToText(diff: GitDiffResult): string {
  return diff.kind === 'text' ? diff.modifiedContent : '[Binary file]'
}

export function useGit() {
  const { project, currentWorktree, emit, refreshGitStatus } = useWorkspace()
  // Why useShallow: this selector returns a fresh object literal every call.
  // Zustand v5's React binding hands that straight to React's own
  // useSyncExternalStore with no built-in memoization, so an unguarded
  // object selector fails its own snapshot-equality check on every render —
  // an unconditional infinite re-render loop (React error #185), live-
  // reproduced simply by mounting GitPanel's default "Changes" tab.
  const { stagedFiles, unstagedFiles, isPushing, isCommitting } = useAppStore(
    useShallow((s) => ({
      stagedFiles: s.stagedFiles,
      unstagedFiles: s.unstagedFiles,
      isPushing: s.isPushing,
      isCommitting: s.isCommitting
    }))
  )

  // Refresh on mount
  const refreshFiles = useCallback(async () => {
    if (!currentWorktree) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const status = await callRuntimeRpc<GitStatusResult>(target, 'git.status', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id)
    })
    const { staged, unstaged } = toGitFileChanges(status)
    const store = useAppStore.getState()
    store.setStagedFiles(staged)
    store.setUnstagedFiles(unstaged)
  }, [currentWorktree])

  useEffect(() => {
    if (currentWorktree) {
      refreshFiles()
    }
  }, [currentWorktree, refreshFiles])

  const stageFile = useCallback(
    async (path: string) => {
      if (!currentWorktree) {
        return
      }
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'git.stage', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id),
        filePath: path
      })
      await refreshFiles()
    },
    [currentWorktree, refreshFiles]
  )

  const unstageFile = useCallback(
    async (path: string) => {
      if (!currentWorktree) {
        return
      }
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'git.unstage', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id),
        filePath: path
      })
      await refreshFiles()
    },
    [currentWorktree, refreshFiles]
  )

  const stageAll = useCallback(async () => {
    if (!currentWorktree || unstagedFiles.length === 0) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'git.bulkStage', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id),
      filePaths: unstagedFiles.map((f) => f.path)
    })
    await refreshFiles()
  }, [currentWorktree, unstagedFiles, refreshFiles])

  const unstageAll = useCallback(async () => {
    if (!currentWorktree || stagedFiles.length === 0) {
      return
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'git.bulkUnstage', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id),
      filePaths: stagedFiles.map((f) => f.path)
    })
    await refreshFiles()
  }, [currentWorktree, stagedFiles, refreshFiles])

  const commit = useCallback(
    async (message: string) => {
      if (!currentWorktree) {
        return
      }
      const store = useAppStore.getState()
      store.setIsCommitting(true)
      try {
        const target = getActiveRuntimeTarget(store.settings)
        await callRuntimeRpc(target, 'git.commit', {
          worktree: toRuntimeWorktreeSelector(currentWorktree.id),
          message
        })
        await refreshGitStatus()
        await refreshFiles()
        emit('git.committed', { message })
      } finally {
        store.setIsCommitting(false)
      }
    },
    [currentWorktree, refreshGitStatus, refreshFiles, emit]
  )

  // FIX BUG-FE-HLD-002: this used to stream 'git.push' through callRuntimeRpcStream,
  // which (a) authenticated with a sessionStorage Bearer token nothing ever set
  // (always sent empty), and (b) posted to /api/rpc/stream, an endpoint that does
  // not exist on the backend — git.push is a single request/response RPC there
  // (backend/src/main/runtime/rpc/methods/git.ts), never a multi-line stream.
  // pushRuntimeGit() already exists, already routes through the correctly
  // authenticated callRuntimeRpc()/window.api.git.push transport, and already
  // sends the params shape the real handler expects (`worktree` selector, not
  // projectId/branch). No `pushLines` UI consumer exists today (grep confirmed),
  // so this drops the illusory line-by-line log in favor of one real push call.
  const push = useCallback(
    async (branch: string) => {
      if (!project || !currentWorktree) {
        return
      }
      const store = useAppStore.getState()
      store.clearPushLines()
      store.setIsPushing(true)
      try {
        const parsed = splitWorktreeIdForFilesystem(currentWorktree.id)
        await pushRuntimeGit(
          {
            settings: store.settings,
            worktreeId: currentWorktree.id,
            worktreePath: parsed?.worktreePath ?? ''
          },
          { pushTarget: { remoteName: 'origin', branchName: branch } }
        )
        store.appendPushLine(`Pushed ${branch}`)
        await refreshGitStatus()
      } finally {
        store.setIsPushing(false)
      }
    },
    [project, currentWorktree, refreshGitStatus]
  )

  const getDiff = useCallback(
    async (filePath: string, staged?: boolean) => {
      if (!currentWorktree) {
        return null
      }
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const diff = await callRuntimeRpc<GitDiffResult>(target, 'git.diff', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id),
        filePath,
        staged: staged ?? false
      })
      const text = diffResultToText(diff)
      useAppStore.getState().setSelectedDiffFile(filePath)
      useAppStore.getState().setDiffContent(text)
      return text
    },
    [currentWorktree]
  )

  // Why: routing this through window.api.git.generateCommitMessage (like the
  // rest of runtime-git-client.ts's local-target branch) would hit the web
  // preload's deliberate 'unavailable in the web client' stub — calling
  // git.generateCommitMessage directly reaches the real backend handler, same
  // as every other fixed method in this hook, since generation itself has no
  // web-vs-desktop restriction server-side.
  const aiCommitMessage = useCallback(async () => {
    if (!currentWorktree) {
      return ''
    }
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.codeReviewAiCommitFlow.start({
      worktreeId: currentWorktree.id,
      entry: 'commit-form'
    })
    try {
      const result = await callRuntimeRpc<{ success: boolean; message?: string; error?: string }>(
        target,
        'git.generateCommitMessage',
        { worktree: toRuntimeWorktreeSelector(currentWorktree.id) }
      )
      if (!result.success || !result.message) {
        throw new Error(result.error ?? 'Failed to generate commit message')
      }
      span.ok({ messageChars: result.message.length })
      return result.message
    } catch (err) {
      span.fail(err, { worktreeId: currentWorktree.id })
      throw err
    }
  }, [currentWorktree])

  return {
    stagedFiles,
    unstagedFiles,
    isPushing,
    isCommitting,
    stageFile,
    unstageFile,
    stageAll,
    unstageAll,
    commit,
    push,
    getDiff,
    refreshFiles,
    aiCommitMessage
  }
}
