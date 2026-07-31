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
