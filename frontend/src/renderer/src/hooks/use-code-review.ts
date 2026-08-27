// src/renderer/src/hooks/use-code-review.ts
// BL-CR-01~05: Hook that manages state for the full code review flow
// Loads changed files from git status, handles file selection, line annotation

import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { pushRuntimeGit } from '../runtime/runtime-git-client'
import { toRuntimeWorktreeSelector } from '../runtime/runtime-worktree-selector'
import { useAppStore } from '../store'
import { useWorkspace } from '../context/WorkspaceContext'
import type { ChangedFile, ChangeType } from '../components/code-review/changed-files-tree'
import type { GitStatusEntry, GitStatusResult } from '../../../shared/git-status-types'

type UseCodeReviewOptions = {
  reviewId?: string
}

// BUG-FE-RPC gap fix: this used to call a nonexistent `git.exec` escape hatch
// with a raw `git diff HEAD --numstat` argv (specs/frontend/tdd/api/gaps-and-mismatches.md
// Category 2) — no such method exists, and deliberately so (every git.* RPC is
// narrow and Zod-validated, not a raw-exec passthrough). `git.status` already
// returns per-entry added/removed line counts (git-uncommitted-line-stats.ts),
// which covers the same "changed since HEAD" data `--numstat` provided.
//
// Unlike `git diff HEAD --numstat` (one combined row per file), `git.status`
// reports staged and unstaged changes to the same file as separate entries, so
// merge them back into one row per path here. Untracked files are dropped to
// match `git diff HEAD`'s semantics, which never included them either.
function toChangedFiles(entries: GitStatusEntry[]): ChangedFile[] {
  const byPath = new Map<string, ChangedFile>()
  for (const entry of entries) {
    if (entry.area === 'untracked') {continue}

    const changeType: ChangeType =
      entry.status === 'added' ? 'added' :
      entry.status === 'deleted' ? 'deleted' :
      entry.status === 'renamed' ? 'renamed' :
      'modified' // 'modified' and 'copied' both collapse to 'modified' — ChangeType has no 'copied'

    const existing = byPath.get(entry.path)
    if (existing) {
      existing.additions += entry.added ?? 0
      existing.deletions += entry.removed ?? 0
      if (entry.oldPath) {existing.oldPath = entry.oldPath}
    } else {
      byPath.set(entry.path, {
        path: entry.path,
        changeType,
        additions: entry.added ?? 0,
        deletions: entry.removed ?? 0,
        ...(entry.oldPath ? { oldPath: entry.oldPath } : {})
      })
    }
  }
  return Array.from(byPath.values())
}

export function useCodeReview({ reviewId: _reviewId }: UseCodeReviewOptions = {}) {
  const [changedFiles, setChangedFiles]     = useState<ChangedFile[]>([])
  const [selectedFile, setSelectedFile]     = useState<string | null>(null)
  const [annotationLine, setAnnotationLine] = useState<number | null>(null)
  const [isLoadingFiles, setIsLoadingFiles] = useState(false)
  const [commitMessage, setCommitMessage]   = useState('')
  const [isCommitting, setIsCommitting]     = useState(false)

  const { project, currentWorktree } = useWorkspace()

  const refreshChangedFiles = useCallback(async () => {
    if (!project || !currentWorktree) {return}
    setIsLoadingFiles(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const status = await callRuntimeRpc<GitStatusResult>(target, 'git.status', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id)
      })
      const files = toChangedFiles(status.entries)

      setChangedFiles(files)
      // Auto-select first file
      if (files.length > 0 && selectedFile === null) {
        setSelectedFile(files[0].path)
      }
    } catch {
      // No changes or git unavailable
      setChangedFiles([])
    } finally {
      setIsLoadingFiles(false)
    }
  }, [project, currentWorktree, selectedFile])

  // Load on mount
  useEffect(() => {
    refreshChangedFiles()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleLineClick = useCallback((lineNumber: number) => {
    setAnnotationLine(lineNumber)
  }, [])

  const closeAnnotation = useCallback(() => {
    setAnnotationLine(null)
  }, [])

  const handleCommit = useCallback(async (push: boolean) => {
    if (!commitMessage.trim() || !project || !currentWorktree) {return}
    setIsCommitting(true)
    try {
      const settings = useAppStore.getState().settings
      const target = getActiveRuntimeTarget(settings)
      await callRuntimeRpc(target, 'git.commit', {
        worktree: toRuntimeWorktreeSelector(currentWorktree.id),
        message: commitMessage
      })
      if (push) {
        // git.commit has no push option of its own (GitCommit params: {worktree, message}
        // only) — chain the real git.push RPC, matching useGit.ts's commit+push pattern.
        await pushRuntimeGit(
          { settings, worktreeId: currentWorktree.id, worktreePath: currentWorktree.path },
          { publish: true }
        )
      }
      toast.success(push ? 'Committed and pushed' : 'Committed')
      setCommitMessage('')
      await refreshChangedFiles()
    } catch (err: any) {
      toast.error(err?.message ?? 'Commit failed')
    } finally {
      setIsCommitting(false)
    }
  }, [commitMessage, project, currentWorktree, refreshChangedFiles])

  return {
    changedFiles,
    selectedFile,
    setSelectedFile,
    annotationLine,
    handleLineClick,
    closeAnnotation,
    isLoadingFiles,
    refreshChangedFiles,
    commitMessage,
    setCommitMessage,
    isCommitting,
    handleCommit,
  }
}
