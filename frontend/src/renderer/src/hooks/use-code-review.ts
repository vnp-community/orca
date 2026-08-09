// src/renderer/src/hooks/use-code-review.ts
// BL-CR-01~05: Hook that manages state for the full code review flow
// Loads changed files from git numstat, handles file selection, line annotation

import { useState, useEffect, useCallback } from 'react'
import { toast } from 'sonner'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { useAppStore } from '../store'
import { useWorkspace } from '../context/WorkspaceContext'
import type { ChangedFile } from '../components/code-review/changed-files-tree'

type UseCodeReviewOptions = {
  reviewId?: string
}

// Parse git numstat output line into a ChangedFile record
// Format: "<additions>\t<deletions>\t<path>" (or "{old => new}" for renames)
function parseNumstatLine(line: string): ChangedFile | null {
  const parts = line.split('\t')
  if (parts.length < 3) {return null}
  const additions = Number.parseInt(parts[0], 10) || 0
  const deletions = Number.parseInt(parts[1], 10) || 0
  const rawPath = parts.slice(2).join('\t')

  // Detect rename: "src/{old => new}/file.ts" or "old/path => new/path"
  const renameMatch = rawPath.match(/^(.*)\{(.+) => (.+)\}(.*)$/)
  if (renameMatch) {
    const [, prefix, oldPart, newPart, suffix] = renameMatch
    return {
      path: `${prefix}${newPart}${suffix}`.replace('//', '/'),
      oldPath: `${prefix}${oldPart}${suffix}`.replace('//', '/'),
      changeType: 'renamed',
      additions,
      deletions,
    }
  }

  // Determine change type from additions/deletions pattern
  const changeType: ChangedFile['changeType'] =
    additions > 0 && deletions === 0 ? 'added' :
    additions === 0 && deletions > 0 ? 'deleted' :
    'modified'

  return { path: rawPath.trim(), changeType, additions, deletions }
}

export function useCodeReview({ reviewId }: UseCodeReviewOptions = {}) {
  const [changedFiles, setChangedFiles]     = useState<ChangedFile[]>([])
  const [selectedFile, setSelectedFile]     = useState<string | null>(null)
  const [annotationLine, setAnnotationLine] = useState<number | null>(null)
  const [isLoadingFiles, setIsLoadingFiles] = useState(false)
  const [commitMessage, setCommitMessage]   = useState('')
  const [isCommitting, setIsCommitting]     = useState(false)

  const { project, worktreePath } = useWorkspace()

  const refreshChangedFiles = useCallback(async () => {
    if (!project) {return}
    setIsLoadingFiles(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // git diff HEAD --numstat returns additions/deletions per file
      const numstatOutput = await callRuntimeRpc<string>(target, 'git.exec', {
        projectId: project.id,
        worktreePath: worktreePath ?? project.rootPath,
        args: ['diff', 'HEAD', '--numstat'],
      })
      const files = numstatOutput
        .split('\n')
        .filter(Boolean)
        .map(parseNumstatLine)
        .filter((f): f is ChangedFile => f !== null)

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
  }, [project, worktreePath, selectedFile])

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
    if (!commitMessage.trim() || !project) {return}
    setIsCommitting(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'git.commit', {
        projectId: project.id,
        worktreePath: worktreePath ?? project.rootPath,
        message: commitMessage,
        push,
      })
      toast.success(push ? 'Committed and pushed' : 'Committed')
      setCommitMessage('')
      await refreshChangedFiles()
    } catch (err: any) {
      toast.error(err?.message ?? 'Commit failed')
    } finally {
      setIsCommitting(false)
    }
  }, [commitMessage, project, worktreePath, refreshChangedFiles])

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
