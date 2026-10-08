/**
 * useEditorQualityAnnotations.ts — FE-CV-TASK-087-11
 *
 * Quality markers for the plain file editor (`MonacoEditor`). The editor shows the worktree
 * file, so it is treated as the 'worktree' side — but only when the tab opened without an
 * unsaved draft: a draft already differs from disk, so run line numbers would not line up.
 * Edits after mount are handled by the marker hook (clear + "content changed" notice).
 *
 * @module components/editor/quality-annotations/useEditorQualityAnnotations
 */

import { useState } from 'react'
import type { editor } from 'monaco-editor'
import { useAppStore } from '@/store'
import { useQualityFindingMarkers } from './useQualityFindingMarkers'
import type { QualityMonacoApi, UseQualityFindingMarkersResult } from './useQualityFindingMarkers'

export type UseEditorQualityAnnotationsArgs = {
  editor: editor.ICodeEditor | null
  monacoApi: QualityMonacoApi
  fileId: string
  worktreeId: string | undefined
  relativePath: string
  readOnly?: boolean
}

export function useEditorQualityAnnotations(
  args: UseEditorQualityAnnotationsArgs
): UseQualityFindingMarkersResult {
  const { editor: codeEditor, monacoApi, fileId, worktreeId, relativePath, readOnly } = args
  // Why: decided once per mount (the editor remounts per file); a later save does not make
  // the run's line numbers valid again — only a new run does.
  const dirtyNow = useAppStore(
    (s) =>
      s.openFiles?.some((f) => f.id === fileId && f.isDirty) === true ||
      s.editorDrafts?.[fileId] !== undefined
  )
  const [openedClean] = useState(!dirtyNow)
  // Why: read-only tabs (e.g. AI Vault logs) are not worktree sources.
  const eligibleSource = openedClean && !readOnly
  return useQualityFindingMarkers({
    editor: codeEditor,
    monacoApi,
    worktreeId,
    relativePath,
    diffSource: eligibleSource ? 'worktree' : undefined
  })
}
