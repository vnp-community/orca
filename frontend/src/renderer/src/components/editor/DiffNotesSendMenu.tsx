import React, { useMemo } from 'react'
import type { DiffComment } from '../../../../shared/types'
import { useAppStore } from '@/store'
import { formatDiffComments } from '@/lib/diff-comments-format'
import { useComposedAllNotesPrompt } from '@/lib/use-composed-all-notes-prompt'
import { findWorktreeById } from '@/store/slices/worktree-helpers'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { NotesSendMenu, type NotesSendMenuScope } from './NotesSendMenu'
import { translate } from '@/i18n/i18n'

// TASK-FE-ANNOTATE-005: best-effort bookkeeping after a successful send —
// a failure here must never surface as a "send failed" error since the
// prompt was already delivered by the time this runs. Only meaningful for
// the 'environment' target (no annotation-service bridge in desktop mode,
// same guard as useComposedAllNotesPrompt/persistRemote).
function markAnnotationsSentBestEffort(annotationIds: readonly string[]): void {
  if (annotationIds.length === 0) {
    return
  }
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  if (target.kind === 'local') {
    return
  }
  void callRuntimeRpc(
    target,
    'annotation.markSent',
    { ids: annotationIds },
    { timeoutMs: 8000 }
  ).catch(() => {})
}

export function DiffNotesSendMenu({
  worktreeId,
  groupId,
  comments,
  filePath,
  showFileScope = false,
  triggerClassName,
  triggerLabel,
  triggerCount,
  actionLabel,
  iconClassName = 'size-3.5',
  align = 'end'
}: {
  worktreeId: string
  groupId: string
  comments: readonly DiffComment[]
  filePath?: string
  showFileScope?: boolean
  triggerClassName?: string
  triggerLabel?: string
  triggerCount?: number
  actionLabel?: string
  iconClassName?: string
  align?: 'start' | 'center' | 'end'
}): React.JSX.Element {
  const clearDeliveredDiffComments = useAppStore((s) => s.clearDeliveredDiffComments)
  // Why: "{worktree-name}" in the composed prompt's header (BL-CR-03 flow
  // step 3) — falls back to the raw id if the worktree isn't in the store
  // for some reason (never actually null in practice, but avoids an
  // undefined literal ending up in the prompt text).
  const worktreeName = useAppStore(
    (s) => findWorktreeById(s.worktreesByRepo, worktreeId)?.displayName ?? worktreeId
  )
  const unsentNotes = useMemo(() => comments.filter((comment) => !comment.sentAt), [comments])
  // TASK-FE-ANNOTATE-004: "all unsent notes" is enriched with backend-go's
  // ±2-line code context when reachable; falls back to the plain
  // client-side format otherwise. See useComposedAllNotesPrompt's own doc
  // comment for why this doesn't touch delivery.
  const { prompt: unsentPrompt, annotationIds: unsentAnnotationIds } = useComposedAllNotesPrompt(
    worktreeId,
    worktreeName,
    unsentNotes
  )
  const fileNotes = useMemo(
    () => (filePath ? comments.filter((comment) => comment.filePath === filePath) : []),
    [comments, filePath]
  )
  const unsentFileNotes = useMemo(() => fileNotes.filter((comment) => !comment.sentAt), [fileNotes])
  // Why: file-scope intentionally stays plain-client-format — see
  // SOL-FE-ANNOTATE-002 §2 ("chỉ scope all unsent notes"); enriching it too
  // would mean extending annotation.composeReviewPrompt with a file filter,
  // out of CR-ANNOTATE-002's scope.
  const unsentFilePrompt = useMemo(() => formatDiffComments(unsentFileNotes), [unsentFileNotes])
  const canSendFileScope = showFileScope && Boolean(filePath)
  const scopes = useMemo<NotesSendMenuScope<DiffComment>[]>(() => {
    const allNotesScope = {
      id: 'all',
      label: translate('auto.components.editor.DiffNotesSendMenu.8b87612461', 'All unsent notes'),
      notes: unsentNotes,
      prompt: unsentPrompt
    }
    if (!canSendFileScope) {
      return [allNotesScope]
    }
    return [
      {
        id: 'file',
        label: translate('auto.components.editor.DiffNotesSendMenu.f1aa04b5cf', 'This file'),
        notes: unsentFileNotes,
        prompt: unsentFilePrompt
      },
      allNotesScope
    ]
  }, [canSendFileScope, unsentFileNotes, unsentFilePrompt, unsentNotes, unsentPrompt])

  return (
    <NotesSendMenu
      worktreeId={worktreeId}
      groupId={groupId}
      modeIdParts={['diff-notes', worktreeId, groupId, filePath ?? 'all']}
      scopes={scopes}
      // Why: file-scoped menus should not broaden delivery before the user
      // intentionally hovers the "All unsent notes" submenu.
      defaultScopeId={canSendFileScope ? 'file' : 'all'}
      triggerClassName={triggerClassName}
      triggerLabel={triggerLabel}
      triggerCount={triggerCount}
      actionLabel={actionLabel}
      iconClassName={iconClassName}
      align={align}
      onDelivered={(notes) => {
        void clearDeliveredDiffComments(worktreeId, notes)
        // Why: unsentAnnotationIds are specifically the "all unsent notes"
        // scope's server ids (from useComposedAllNotesPrompt) — only valid
        // when `notes` (whatever NotesSendMenu actually delivered) IS that
        // scope. The file-scope send path has no server-id tracking of its
        // own (it never calls annotation.composeReviewPrompt — see that
        // scope's own note above), so mark-sent is skipped for it rather
        // than risk marking the wrong comments sent.
        const deliveredIsAllScope =
          notes.length === unsentNotes.length &&
          new Set(notes.map((n) => n.id)).size === unsentNotes.length &&
          unsentNotes.every((n) => notes.some((delivered) => delivered.id === n.id))
        if (deliveredIsAllScope) {
          markAnnotationsSentBestEffort(unsentAnnotationIds)
        }
      }}
    />
  )
}
