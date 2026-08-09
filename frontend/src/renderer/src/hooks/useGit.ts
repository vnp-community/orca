import { useCallback, useEffect } from 'react'
import { useWorkspace } from '../context/WorkspaceContext'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { pushRuntimeGit } from '../runtime/runtime-git-client'
import type { GitFileChange } from '../store/slices/git-panel'
import { Tracers } from '../../../shared/trace/tracers'
import { splitWorktreeIdForFilesystem } from '../../../shared/worktree-id'

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
    if (!project) {return}
    const status = await callRuntimeRpc('git.getStatus', { projectId: project.id }) as {
      staged:   GitFileChange[]
      unstaged: GitFileChange[]
    }
    const store = useAppStore.getState()
    store.setStagedFiles(status.staged)
    store.setUnstagedFiles(status.unstaged)
  }, [project])

  useEffect(() => {
    if (project) {refreshFiles()}
  }, [project, refreshFiles])

  const stageFile = useCallback(async (path: string) => {
    if (!project) {return}
    await callRuntimeRpc('git.stageFile', { projectId: project.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const unstageFile = useCallback(async (path: string) => {
    if (!project) {return}
    await callRuntimeRpc('git.unstageFile', { projectId: project.id, path })
    await refreshFiles()
  }, [project, refreshFiles])

  const stageAll = useCallback(async () => {
    if (!project) {return}
    await callRuntimeRpc('git.stageAll', { projectId: project.id })
    await refreshFiles()
  }, [project, refreshFiles])

  const unstageAll = useCallback(async () => {
    if (!project) {return}
    await callRuntimeRpc('git.unstageAll', { projectId: project.id })
    await refreshFiles()
  }, [project, refreshFiles])

  const commit = useCallback(async (message: string) => {
    if (!project) {return}
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
  const push = useCallback(async (branch: string) => {
    if (!project || !currentWorktree) {return}
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
  }, [project, currentWorktree, refreshGitStatus])

  const getDiff = useCallback(async (filePath: string, staged?: boolean) => {
    if (!project) {return null}
    const diff = await callRuntimeRpc('git.getDiff', {
      projectId: project.id, path: filePath, staged,
    }) as string
    useAppStore.getState().setSelectedDiffFile(filePath)
    useAppStore.getState().setDiffContent(diff)
    return diff
  }, [project])

  const aiCommitMessage = useCallback(async () => {
    if (!project) {return ''}
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.codeReviewAiCommitFlow.start({ projectId: project.id, entry: 'commit-form' })
    try {
      // Why: callRuntimeRpc's real signature requires `target` as the first
      // argument — the original call site here omitted it (pre-existing bug,
      // out of scope for this CR); fixed to match the real signature instead
      // of repeating the mismatch while adding tracing.
      const result = await callRuntimeRpc<{ message: string }>(target, 'git.aiCommitMessage', {
        projectId: project.id,
        traceId: span.id
      })
      span.ok({ messageChars: result.message.length })
      return result.message
    } catch (err) {
      span.fail(err, { projectId: project.id })
      throw err
    }
  }, [project])

  return {
    stagedFiles, unstagedFiles, isPushing, isCommitting,
    stageFile, unstageFile, stageAll, unstageAll,
    commit, push, getDiff, refreshFiles, aiCommitMessage,
  }
}
