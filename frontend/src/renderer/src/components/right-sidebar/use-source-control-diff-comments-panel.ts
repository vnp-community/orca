import { useCallback, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import type { DiffComment } from '../../../../shared/types'
import {
  countPendingDiffCommentsClear,
  formatPendingDiffCommentsClearDescription,
  resolvePendingDiffCommentsClear,
  type PendingDiffCommentsClear
} from './diff-comments-clear-dialog-state'
import { useCopyFeedbackState } from './SourceControl'

import { uiWriteClipboardText } from '@/runtime/runtime-ui-client'
export type UseSourceControlDiffCommentsPanelStateInput = {
  activeWorktreeId: string | null
  diffCommentsForActive: DiffComment[]
  diffCommentsPrompt: string
  clearDiffComments: (worktreeId: string) => Promise<boolean>
  clearDiffCommentsForFile: (worktreeId: string, filePath: string) => Promise<boolean>
}

/**
 * Diff-comments panel state for `SourceControlInner`: expand/collapse,
 * copy-to-clipboard, and clear (single file / all) confirmation. Extracted
 * verbatim from `SourceControlInner` — see TASK-BIGFILE-225.
 */
export function useSourceControlDiffCommentsPanelState(
  input: UseSourceControlDiffCommentsPanelStateInput
) {
  const {
    activeWorktreeId,
    diffCommentsForActive,
    diffCommentsPrompt,
    clearDiffComments,
    clearDiffCommentsForFile
  } = input

  const [diffCommentsExpanded, setDiffCommentsExpanded] = useState(false)
  const [diffCommentsCopied, showDiffCommentsCopied] = useCopyFeedbackState(false)
  const [pendingDiffCommentsClear, setPendingDiffCommentsClear] =
    useState<PendingDiffCommentsClear | null>(null)
  const [isClearingDiffComments, setIsClearingDiffComments] = useState(false)

  const handleCopyDiffComments = useCallback(async (): Promise<void> => {
    if (diffCommentsForActive.length === 0) {
      return
    }
    try {
      await uiWriteClipboardText(diffCommentsPrompt)
      showDiffCommentsCopied(true)
    } catch {
      // Why: swallow — clipboard write can fail when the window isn't focused.
      // No dedicated error surface is warranted for a best-effort copy action.
    }
  }, [diffCommentsForActive, diffCommentsPrompt, showDiffCommentsCopied])

  const pendingDiffCommentsClearCount = useMemo(() => {
    return countPendingDiffCommentsClear(
      pendingDiffCommentsClear,
      activeWorktreeId,
      diffCommentsForActive
    )
  }, [activeWorktreeId, diffCommentsForActive, pendingDiffCommentsClear])

  const resolvedPendingDiffCommentsClear = resolvePendingDiffCommentsClear({
    activeWorktreeId,
    isClearing: isClearingDiffComments,
    pending: pendingDiffCommentsClear,
    pendingCount: pendingDiffCommentsClearCount
  })
  if (resolvedPendingDiffCommentsClear !== pendingDiffCommentsClear) {
    // Why: the confirmation is purely local UI state; clear impossible
    // confirmations before children observe a stale open dialog.
    setPendingDiffCommentsClear(resolvedPendingDiffCommentsClear)
  }

  const pendingDiffCommentsClearDescription = formatPendingDiffCommentsClearDescription(
    resolvedPendingDiffCommentsClear,
    pendingDiffCommentsClearCount
  )

  const handleConfirmDiffCommentsClear = useCallback(async (): Promise<void> => {
    const pending = resolvedPendingDiffCommentsClear
    if (!pending || isClearingDiffComments || pending.worktreeId !== activeWorktreeId) {
      return
    }
    if (pendingDiffCommentsClearCount === 0) {
      setPendingDiffCommentsClear(null)
      return
    }
    setIsClearingDiffComments(true)
    try {
      const ok =
        pending.kind === 'all'
          ? await clearDiffComments(pending.worktreeId)
          : await clearDiffCommentsForFile(pending.worktreeId, pending.filePath)
      if (ok) {
        setPendingDiffCommentsClear(null)
      } else {
        toast.error(
          translate(
            'auto.components.right.sidebar.SourceControl.eae7a1da5f',
            'Failed to clear notes.'
          )
        )
      }
    } finally {
      setIsClearingDiffComments(false)
    }
  }, [
    activeWorktreeId,
    clearDiffComments,
    clearDiffCommentsForFile,
    isClearingDiffComments,
    resolvedPendingDiffCommentsClear,
    pendingDiffCommentsClearCount
  ])

  // Why: the shared worktree-switch reset effect in SourceControlInner needs
  // to clear this panel's confirmation/loading state without reaching into
  // its module-private setters directly.
  const resetDiffCommentsPanel = useCallback((): void => {
    setPendingDiffCommentsClear(null)
    setIsClearingDiffComments(false)
  }, [])

  return {
    diffCommentsExpanded,
    setDiffCommentsExpanded,
    diffCommentsCopied,
    pendingDiffCommentsClear,
    setPendingDiffCommentsClear,
    isClearingDiffComments,
    setIsClearingDiffComments,
    handleCopyDiffComments,
    pendingDiffCommentsClearCount,
    resolvedPendingDiffCommentsClear,
    pendingDiffCommentsClearDescription,
    handleConfirmDiffCommentsClear,
    resetDiffCommentsPanel
  }
}
