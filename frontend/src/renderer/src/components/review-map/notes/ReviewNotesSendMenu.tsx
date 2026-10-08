/**
 * ReviewNotesSendMenu.tsx — FE-CV-TASK-060-04
 *
 * Wraps the shared NotesSendMenu for review-map notes with three scopes: all unsent notes (same
 * prompt as Source Control), notes of the current lens, and an explicit selection. Only the
 * "all" scope touches annotation-service. Delivery order lives in review-notes-delivery.ts.
 *
 * @module components/review-map/notes/ReviewNotesSendMenu
 */

import { useMemo, useState } from 'react'
import { Eye } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { formatDiffComments } from '@/lib/diff-comments-format'
import { markAnnotationsSentBestEffort } from '@/lib/annotation-mark-sent-best-effort'
import { useComposedAllNotesPrompt } from '@/lib/use-composed-all-notes-prompt'
import { findWorktreeById } from '@/store/slices/worktree-helpers'
import { recordReviewSurfaceDecision } from '@/lib/review-surface-decision'
import { NotesSendMenu } from '../../editor/NotesSendMenu'
import type { NotesSendMenuScope } from '../../editor/NotesSendMenu'
import type { DiffComment } from '../../../../../shared/types'
import { useReviewNotesPersistence } from '../../../hooks/useReviewNotesPersistence'
import { fileIdentities } from '../turns/turn-file-identity'
import { anchorLensId } from './review-note-anchor'
import { handleNotesDelivered } from './review-notes-delivery'
import { anchoredNotes } from './review-note-selectors'
import { ReviewSendPreviewDialog } from './ReviewSendPreviewDialog'
import { tn } from './notes-i18n'

export function ReviewNotesSendMenu({
  worktreeId,
  selectedCommentIds
}: {
  worktreeId: string
  /** Comment ids the user ticked in the panel; the "selection" scope exists only when non-empty. */
  selectedCommentIds?: readonly string[]
}): React.JSX.Element {
  const comments = useAppStore((s) => s.getDiffComments(worktreeId))
  const clearDelivered = useAppStore((s) => s.clearDeliveredDiffComments)
  const currentLens = useAppStore((s) => s.reviewUiByWorktree[worktreeId]?.lens ?? null)
  const worktreeName = useAppStore(
    (s) => findWorktreeById(s.worktreesByRepo, worktreeId)?.displayName ?? worktreeId
  )
  const { notes, updateNotes } = useReviewNotesPersistence(worktreeId)
  const [recordFailed, setRecordFailed] = useState(false)
  const [previewOpen, setPreviewOpen] = useState(false)

  const unsent = useMemo(() => comments.filter((c) => !c.sentAt), [comments])
  const { prompt: allPrompt, annotationIds } = useComposedAllNotesPrompt(worktreeId, worktreeName, unsent)
  const lensNotes = useMemo(
    () =>
      anchoredNotes(unsent, notes.anchors)
        .filter((n) => currentLens !== null && anchorLensId(n.anchor) === currentLens)
        .map((n) => n.comment),
    [unsent, notes.anchors, currentLens]
  )
  const selectedNotes = useMemo(
    () => unsent.filter((c) => selectedCommentIds?.includes(c.id)),
    [unsent, selectedCommentIds]
  )
  const lensPrompt = useMemo(() => formatDiffComments(lensNotes), [lensNotes])
  const selectionPrompt = useMemo(() => formatDiffComments(selectedNotes), [selectedNotes])

  const scopes = useMemo<NotesSendMenuScope<DiffComment>[]>(
    () => [
      { id: 'all', label: tn('ReviewNote.scope.all', 'All unsent notes'), notes: unsent, prompt: allPrompt },
      { id: 'lens', label: tn('ReviewNote.scope.lens', 'Notes in this lens'), notes: lensNotes, prompt: lensPrompt },
      {
        id: 'selection',
        label: tn('ReviewNote.scope.selection', 'Selected notes'),
        notes: selectedNotes,
        prompt: selectionPrompt
      }
    ],
    [unsent, allPrompt, lensNotes, lensPrompt, selectedNotes, selectionPrompt]
  )

  const onDelivered = (delivered: readonly DiffComment[]): void => {
    recordReviewSurfaceDecision(worktreeId, 'send_to_agent')
    const state = useAppStore.getState()
    const summary = state.gitBranchCompareSummaryByWorktree[worktreeId]
    const identities = fileIdentities(state.gitStatusByWorktree[worktreeId] ?? [], {
      headOid: summary?.headOid,
      mergeBase: summary?.mergeBase
    }).files
    const isAllScope =
      delivered.length === unsent.length &&
      new Set(delivered.map((n) => n.id)).size === unsent.length &&
      unsent.every((n) => delivered.some((d) => d.id === n.id))
    void handleNotesDelivered(
      {
        updateNotes,
        clearDelivered,
        markAnnotationsSent: markAnnotationsSentBestEffort,
        now: Date.now,
        newBatchId: () => `batch-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
      },
      {
        worktreeId,
        delivered,
        anchors: notes.anchors,
        isAllScope,
        annotationIds,
        fileIdentityByPath: Object.fromEntries(identities.map((f) => [f.p, f.h]))
      }
    ).then((outcome) => setRecordFailed(!outcome.recorded))
  }

  return (
    <span className="inline-flex items-center gap-1">
      <Button
        type="button"
        variant="ghost"
        size="xs"
        disabled={unsent.length === 0}
        onClick={() => setPreviewOpen(true)}
      >
        <Eye aria-hidden />
        {tn('ReviewNote.previewButton', 'Preview')}
      </Button>
      <NotesSendMenu
        worktreeId={worktreeId}
        groupId="review-map"
        modeIdParts={['review-notes', worktreeId]}
        scopes={scopes}
        defaultScopeId="all"
        source="diff-notes"
        onDelivered={onDelivered}
      />
      {recordFailed ? (
        <span role="status" className="text-[11px] text-destructive">
          {tn('ReviewSent.recordFailed', 'Sent, but the batch history was not saved.')}
        </span>
      ) : null}
      <ReviewSendPreviewDialog
        open={previewOpen}
        onOpenChange={setPreviewOpen}
        prompt={allPrompt}
        noteCount={unsent.length}
      />
    </span>
  )
}
